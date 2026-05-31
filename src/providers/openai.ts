import OpenAI from 'openai';
import type { Provider, Message, ToolDefinition, StreamEvent } from './types';
import type { MaxTokensField } from './catalog/types';

interface OpenAIProviderOptions {
  apiKey: string;
  baseURL?: string;
  model: string;
  defaultHeaders?: Record<string, string | null>;
  maxTokensField?: MaxTokensField;
  removeBodyFields?: string[];
}

export class OpenAIProvider implements Provider {
  private client: OpenAI;
  private model: string;
  private maxTokensField: MaxTokensField;
  private removeBodyFields: Set<string>;

  constructor({ apiKey, baseURL, model, defaultHeaders, maxTokensField, removeBodyFields }: OpenAIProviderOptions) {
    this.client = new OpenAI({
      apiKey,
      baseURL: baseURL || 'https://api.openai.com/v1',
      defaultHeaders,
    });
    this.model = model;
    this.maxTokensField = maxTokensField ?? 'max_tokens';
    this.removeBodyFields = new Set(removeBodyFields ?? []);
  }

  async *streamCompletion(
    messages: Message[],
    tools: ToolDefinition[]
  ): AsyncIterable<StreamEvent> {
    const openaiMessages = messages.map((m) => ({
      role: m.role,
      content: m.content,
      tool_calls: m.toolCalls?.map((tc) => ({
        id: tc.id,
        type: 'function' as const,
        function: {
          name: tc.name,
          arguments: tc.arguments,
        },
      })),
      tool_call_id: m.toolCallId,
    }));

    const openaiTools = tools.length > 0
      ? tools.map((t) => ({
          type: 'function' as const,
          function: {
            name: t.name,
            description: t.description,
            parameters: t.parameters,
          },
        }))
      : undefined;

    // Build request body
    const body: Record<string, unknown> = {
      model: this.model,
      messages: openaiMessages,
      tools: openaiTools,
      stream: true,
    };

    // Strip configured body fields
    for (const field of this.removeBodyFields) {
      delete body[field];
    }

    // Validate required fields before sending
    if (!body.model) {
      throw new Error('Provider request missing required field: model');
    }
    if (!body.messages || !Array.isArray(body.messages) || body.messages.length === 0) {
      throw new Error('Provider request missing required field: messages');
    }

    let stream;
    try {
      stream = await this.client.chat.completions.create(
        body as Parameters<typeof this.client.chat.completions.create>[0],
        { timeout: 120_000 } // 2 minute timeout
      );
    } catch (err: any) {
      const message = getDetailedErrorMessage(err);
      if (message.includes('401') || message.toLowerCase().includes('invalid') || message.toLowerCase().includes('unauthorized')) {
        throw new Error(`Provider authentication error (check your API key): ${message}`);
      }
      if (message.toLowerCase().includes('rate') || message.toLowerCase().includes('quota')) {
        throw new Error(`Provider rate limit error: ${message}`);
      }
      throw new Error(`Provider returned error: ${message}`);
    }

    const toolCallAccumulators = new Map<number, { 
      id: string; 
      name: string;
      arguments: string;
      started: boolean;
      ended: boolean;
    }>();

    try {
      for await (const chunk of stream) {
        // Some OpenAI-compatible servers send errors as special chunks
        if ((chunk as any).error) {
          const errData = (chunk as any).error;
          const msg = errData.message || JSON.stringify(errData);
          throw new Error(`Provider returned error: ${msg}`);
        }

        const delta = chunk.choices[0]?.delta;
        const finishReason = chunk.choices[0]?.finish_reason;

        if (delta?.content) {
          yield { type: 'text', content: delta.content };
        }

        if (delta?.tool_calls) {
          for (const tc of delta.tool_calls) {
            if (tc.index === undefined) continue;

            let acc = toolCallAccumulators.get(tc.index);
            if (!acc) {
              acc = { id: '', name: '', arguments: '', started: false, ended: false };
              toolCallAccumulators.set(tc.index, acc);
            }

            // If we already had data at this index and now get a new id, 
            // the previous tool call is complete.
            if (tc.id && acc.id && acc.id !== tc.id) {
              yield { type: 'tool-call-end', id: acc.id };
              acc = { id: '', name: '', arguments: '', started: false, ended: false };
              toolCallAccumulators.set(tc.index, acc);
            }

            if (tc.id) acc.id = tc.id;
            if (tc.function?.name) acc.name = tc.function.name;
            if (tc.function?.arguments) {
              acc.arguments += tc.function.arguments;
              // Only yield deltas once we have the real id
              if (acc.id) {
                yield {
                  type: 'tool-call-delta',
                  id: acc.id,
                  argumentsFragment: tc.function.arguments,
                };
              }
            }

            // Emit start event the first time we have both id and name
            if (acc.id && acc.name && !acc.started) {
              yield { type: 'tool-call-start', id: acc.id, name: acc.name };
              acc.started = true;
            }
          }
        }

        if (chunk.usage) {
          yield {
            type: 'usage',
            promptTokens: chunk.usage.prompt_tokens,
            completionTokens: chunk.usage.completion_tokens,
          };
        }

        // If the model signaled it's done with tool calls, close any open ones
        if (finishReason === 'tool_calls') {
          for (const [_, acc] of toolCallAccumulators) {
            if (acc.id && !acc.ended) {
              yield { type: 'tool-call-end', id: acc.id };
              acc.ended = true;
            }
          }
        }
      }

      // End of stream - close any remaining open tool calls
      for (const [_, acc] of toolCallAccumulators) {
        if (acc.id && !acc.ended) {
          yield { type: 'tool-call-end', id: acc.id };
          acc.ended = true;
        }
      }

      yield { type: 'done' };
    } catch (err: any) {
      const message = getDetailedErrorMessage(err);
      throw new Error(`Provider returned error during streaming: ${message}`);
    }
  }
}

function getDetailedErrorMessage(err: any): string {
  if (!err) return 'Unknown error';

  // Try common places where real error lives (especially for custom gateways)
  if (err.message && !err.message.includes('Provider returned error')) {
    return err.message;
  }

  if (err.error) {
    if (typeof err.error === 'string') return err.error;
    if (err.error.message) return err.error.message;
    try { return JSON.stringify(err.error); } catch { return String(err.error); }
  }

  if (err.response?.data) {
    const data = err.response.data;
    if (data.error?.message) return data.error.message;
    if (typeof data.error === 'string') return data.error;
    try { return JSON.stringify(data); } catch { return '[unserializable response data]'; }
  }

  if (err.cause?.message) return err.cause.message;

  return err.message || String(err);
}

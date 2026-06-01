import type { Provider } from '../providers/types';
import type { ToolCall, ToolResult } from '../tools/types';
import { toolRegistry } from '../tools';
import { DEFAULT_SYSTEM_PROMPT, PLAN_MODE_SYSTEM_PROMPT } from './prompts';
import { clearPlan, getCurrentPlan, type PlanItem } from '../tools/plan';
import { z } from 'zod';

export interface AgentOptions {
  maxTurns?: number;
  onText?: (text: string) => void;
  onToolCall?: (toolCall: ToolCall) => void;
  onToolResult?: (result: ToolResult) => void;
  onUsage?: (usage: { promptTokens: number; completionTokens: number }) => void;
  onPlanUpdate?: (plan: PlanItem[]) => void;
  toolsEnabled?: boolean;
  debug?: boolean;
  planMode?: boolean;
}

interface PendingToolCall {
  id: string;
  name: string;
  arguments: string;
}

export async function runAgent(
  initialPrompt: string,
  provider: Provider,
  options: AgentOptions = {}
): Promise<string> {
  const {
    maxTurns = 12,
    onText,
    onToolCall,
    onToolResult,
    onUsage,
    onPlanUpdate,
    toolsEnabled = true,
    debug = false,
    planMode = false,
  } = options;

  // Clear any previous plan when starting a new task
  clearPlan();
  onPlanUpdate?.(getCurrentPlan());

  const systemPrompt = planMode ? PLAN_MODE_SYSTEM_PROMPT : DEFAULT_SYSTEM_PROMPT;

  const messages: any[] = [
    { role: 'system', content: systemPrompt },
    { role: 'user', content: initialPrompt },
  ];

  const tools = toolRegistry.getAll();
  let finalAnswer = '';

  for (let turn = 0; turn < maxTurns; turn++) {
    const toolDefinitions = (toolsEnabled && tools.length > 0)
      ? tools.map(t => {
          const jsonSchema = z.toJSONSchema(t.parameters, { target: 'draft-7' }) as any;
          delete jsonSchema.$schema;

          if (jsonSchema.type === 'object' && !('additionalProperties' in jsonSchema)) {
            jsonSchema.additionalProperties = false;
          }

          return {
            name: t.name,
            description: t.description,
            parameters: jsonSchema,
          };
        })
      : [];

    let currentText = '';
    const toolCallMap = new Map<string, PendingToolCall>();

    if (debug) {
      const red = '\x1b[31m';
      const reset = '\x1b[0m';
      const border = '-'.repeat(50);
      const toolsList = toolDefinitions.map(t => t.name).join(', ');
      const preview = String(messages[messages.length - 1]?.content || '').slice(0, 45);

      console.log(`\n${red}+${border}+`);
      console.log(`|  SENDING TO PROVIDER${' '.repeat(31)}|`);
      console.log(`+${border}+`);
      console.log(`| Messages: ${messages.length}${' '.repeat(Math.max(0, 40 - String(messages.length).length))}|`);
      console.log(`| Tools enabled: ${toolDefinitions.length > 0}${' '.repeat(33)}|`);
      console.log(`| Tool count: ${toolDefinitions.length}${' '.repeat(Math.max(0, 38 - String(toolDefinitions.length).length))}|`);
      if (toolsList) {
        console.log(`| Tools: ${toolsList.slice(0, 42)}${' '.repeat(Math.max(0, 43 - toolsList.length))}|`);
      }
      console.log(`| Last message: ${preview}${' '.repeat(Math.max(0, 36 - preview.length))}|`);
      console.log(`+${border}+${reset}\n`);
    }

    // Stream the response
    for await (const event of provider.streamCompletion(messages, toolDefinitions)) {
      if (event.type === 'text') {
        currentText += event.content;
        if (onText) onText(event.content);
      }

      if (event.type === 'usage') {
        onUsage?.({
          promptTokens: event.promptTokens,
          completionTokens: event.completionTokens,
        });
      }

      if (event.type === 'tool-call-start') {
        toolCallMap.set(event.id, {
          id: event.id,
          name: event.name,
          arguments: '',
        });
        // Do NOT emit to UI yet — we want the full arguments for proper formatting
      }

      if (event.type === 'tool-call-delta') {
        const existing = toolCallMap.get(event.id);
        if (existing) {
          existing.arguments += event.argumentsFragment;
        }
      }

      if (event.type === 'tool-call-end') {
        // Tool call is now complete (we can execute it later)
      }
    }

    // Convert accumulated tool calls
    const assistantToolCalls: ToolCall[] = Array.from(toolCallMap.values()).map(tc => ({
      id: tc.id,
      name: tc.name,
      arguments: tc.arguments,
    }));

    // Emit complete tool calls to the UI (with full arguments) so the formatter can show the actual command
    if (onToolCall) {
      for (const tc of assistantToolCalls) {
        onToolCall(tc);
      }
    }

    // Add assistant message to history
    messages.push({
      role: 'assistant',
      content: currentText || null,
      toolCalls: assistantToolCalls.length > 0 ? assistantToolCalls : undefined,
    });

    if (assistantToolCalls.length === 0) {
      finalAnswer = currentText;
      break;
    }

    // === Execute tools (in parallel) ===
    const toolPromises = assistantToolCalls.map(async (tc) => {
      const tool = tools.find(t => t.name === tc.name);

      let result: string;

      if (!tool) {
        result = `Error: Unknown tool "${tc.name}".`;
      } else {
        let parsedArgs: any = {};
        try {
          parsedArgs = tc.arguments ? JSON.parse(tc.arguments) : {};
        } catch (e: any) {
          result = `Error: Failed to parse arguments for ${tc.name}: ${e.message}`;
          if (onToolResult) onToolResult({ toolCallId: tc.id, result });
          return { toolCallId: tc.id, result };
        }

        try {
          result = await tool.execute(parsedArgs);
        } catch (e: any) {
          result = `Error executing ${tc.name}: ${e.message}`;
        }
      }

      if (onToolResult) {
        onToolResult({ toolCallId: tc.id, result });
      }

      const effects = tool?.onAfterExecute?.(result);
      if (effects?.planUpdated) {
        onPlanUpdate?.(getCurrentPlan());
      }

      return { toolCallId: tc.id, result };
    });

    const toolResults = await Promise.all(toolPromises);

    // Feed tool results back into the conversation
    for (const tr of toolResults) {
      messages.push({
        role: 'tool',
        content: tr.result,
        toolCallId: tr.toolCallId,
      });
    }
  }

  return finalAnswer || 'Agent reached maximum number of turns without a final answer.';
}

import { afterEach, describe, expect, it, mock } from 'bun:test';

const createMock: any = mock(async () => {
  throw new Error('create mock not configured');
});

mock.module('openai', () => ({
  default: class MockOpenAI {
    chat = {
      completions: {
        create: createMock,
      },
    };
  },
}));

async function* streamFrom(chunks: any[]) {
  for (const chunk of chunks) {
    yield chunk;
  }
}

afterEach(() => {
  createMock.mockReset();
});

describe('OpenAIProvider stream options compatibility', () => {
  it('requests usage and retries without stream_options when a compatible provider rejects it', async () => {
    const { OpenAIProvider } = await import('../src/providers/openai');
    const unsupportedStreamOptions = Object.assign(new Error('unknown parameter: stream_options.include_usage'), {
      status: 400,
    });

    createMock
      .mockRejectedValueOnce(unsupportedStreamOptions)
      .mockResolvedValueOnce(streamFrom([
        {
          choices: [{ delta: { content: 'hi' }, finish_reason: 'stop' }],
        },
      ]));

    const provider = new OpenAIProvider({ apiKey: 'test', model: 'test-model' });
    const events = [];

    for await (const event of provider.streamCompletion([{ role: 'user', content: 'hello' }], [])) {
      events.push(event);
    }

    expect(createMock).toHaveBeenCalledTimes(2);
    expect(createMock.mock.calls[0]?.[0]?.stream_options).toEqual({ include_usage: true });
    expect(createMock.mock.calls[1]?.[0]?.stream_options).toBeUndefined();
    expect(events).toContainEqual({ type: 'text', content: 'hi' });
  });

  it('does not retry unrelated unknown-parameter errors', async () => {
    const { OpenAIProvider } = await import('../src/providers/openai');
    const unsupportedToolChoice = Object.assign(new Error('unknown parameter: tool_choice'), {
      status: 400,
    });

    createMock.mockRejectedValueOnce(unsupportedToolChoice);

    const provider = new OpenAIProvider({ apiKey: 'test', model: 'test-model' });

    await expect((async () => {
      for await (const _event of provider.streamCompletion([{ role: 'user', content: 'hello' }], [])) {
        // Drain the stream.
      }
    })()).rejects.toThrow('unknown parameter: tool_choice');

    expect(createMock).toHaveBeenCalledTimes(1);
  });

  it('categorizes mid-stream authentication errors', async () => {
    const { OpenAIProvider } = await import('../src/providers/openai');

    createMock.mockResolvedValueOnce(streamFrom([
      {
        error: {
          message: 'Incorrect API key provided',
        },
      },
    ]));

    const provider = new OpenAIProvider({ apiKey: 'test', model: 'test-model' });

    await expect((async () => {
      for await (const _event of provider.streamCompletion([{ role: 'user', content: 'hello' }], [])) {
        // Drain the stream.
      }
    })()).rejects.toThrow('Provider authentication error');
  });

  it('emits usage events from compatible OpenAI streams', async () => {
    const { OpenAIProvider } = await import('../src/providers/openai');

    createMock.mockResolvedValueOnce(streamFrom([
      {
        choices: [{ delta: {}, finish_reason: 'stop' }],
        usage: {
          prompt_tokens: 8,
          completion_tokens: 3,
        },
      },
    ]));

    const provider = new OpenAIProvider({ apiKey: 'test', model: 'test-model' });
    const events = [];

    for await (const event of provider.streamCompletion([{ role: 'user', content: 'hello' }], [])) {
      events.push(event);
    }

    expect(createMock.mock.calls[0]?.[0]?.stream_options).toEqual({ include_usage: true });
    expect(events).toContainEqual({ type: 'usage', promptTokens: 8, completionTokens: 3 });
  });
});

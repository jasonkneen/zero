import { describe, expect, it } from 'bun:test';
import { existsSync, readFileSync, rmSync } from 'fs';
import { join } from 'path';
import {
  getGlobalModelDefinition,
  getModelsForProvider,
  getProviderDefinition,
  listFirstPartyProviders,
  listGlobalModels,
  listModelDefinitions,
  listProviderDefinitions,
  listProvidersByKind,
  resolveEffectiveTransportConfig,
} from '../src/providers/catalog';
import { discoverModelsForProvider } from '../src/providers/discovery';
import { createProvider, isProviderRuntimeSupported } from '../src/providers/factory';
import { OpenAIProvider } from '../src/providers/openai';
import { parseProviderCommand, resolveProviderCommandConfig } from '../src/config/provider';
import type { ProviderDefinition } from '../src/providers/catalog/types';

function testCacheDir(name: string): string {
  return join(
    import.meta.dir,
    '..',
    '.zero-test-cache',
    `${name}-${Date.now()}-${Math.random().toString(36).slice(2)}`
  );
}

function cleanupTestCacheDir(path: string): void {
  try {
    rmSync(path, { recursive: true, force: true });
  } catch (err: any) {
    if (err?.code !== 'EACCES' && err?.code !== 'EPERM') {
      throw err;
    }
  }
}

describe('provider catalog', () => {
  it('auto-discovers provider and gateway definitions', () => {
    const definitions = listProviderDefinitions();
    const opengateway = getProviderDefinition('opengateway');

    expect(definitions.map((definition) => definition.id)).toContain('opengateway');
    expect(opengateway?.kind).toBe('gateway');
    expect(opengateway?.baseURL).toBe('https://opengateway.gitlawb.com/v1');
    expect(opengateway?.defaultModel).toBe('mimo-v2.5-pro');
  });

  it('auto-discovers localhost provider definitions', () => {
    const ollama = getProviderDefinition('ollama');

    expect(ollama).toBeDefined();
    expect(ollama?.kind).toBe('localhost');
    expect(ollama?.category).toBe('local');
    expect(ollama?.apiKeyRequired).toBe(false);
    expect(ollama?.setup?.authMode).toBe('none');
  });

  it('lists providers by kind', () => {
    const gateways = listProvidersByKind('gateway');
    const localhost = listProvidersByKind('localhost');

    expect(gateways.map((d) => d.id)).toContain('opengateway');
    expect(localhost.map((d) => d.id)).toContain('ollama');
  });

  it('auto-discovers global model catalog entries', () => {
    const models = listModelDefinitions();
    const gpt4o = getGlobalModelDefinition('gpt-4o');

    expect(models.map((model) => model.id)).toContain('gpt-4o');
    expect(gpt4o?.tier).toBe('first-party');
    expect(gpt4o?.contextWindow).toBe(128000);
    expect(gpt4o?.maxOutputTokens).toBe(16384);
    expect(gpt4o?.capabilities?.supportsVision).toBe(true);
  });

  it('lists all global models', () => {
    const globals = listGlobalModels();
    const ids = globals.map((m) => m.id);

    expect(ids).toContain('gpt-4o');
    expect(ids).toContain('gpt-4o-mini');
    expect(ids).toContain('claude-sonnet-4');
    expect(ids).toContain('deepseek-v3');
  });

  it('resolves globalModelId references in gateway catalog models', () => {
    const opengateway = getProviderDefinition('opengateway');
    expect(opengateway).toBeDefined();

    const models = getModelsForProvider(opengateway!);
    // Model inherits id from global catalog when not explicitly set
    const gpt4oOverride = models.find((m) => m.apiName === 'gpt-4o');

    expect(gpt4oOverride).toBeDefined();
    expect(gpt4oOverride?.apiName).toBe('gpt-4o');
    // Gateway overrides reduced limits
    expect(gpt4oOverride?.contextWindow).toBe(32000);
    expect(gpt4oOverride?.maxOutputTokens).toBe(4096);
    // Inherited from global model
    expect(gpt4oOverride?.capabilities?.supportsVision).toBe(true);
    expect(gpt4oOverride?.tier).toBe('first-party');
    // Provider metadata
    expect(gpt4oOverride?.providerId).toBe('opengateway');
    expect(gpt4oOverride?.providerKind).toBe('gateway');
  });

  it('creates runtime providers from provider config', () => {
    const provider = createProvider({
      providerId: 'opengateway',
      apiKey: 'test-key',
      baseURL: 'https://opengateway.gitlawb.com/v1',
      model: 'mimo-v2.5-pro',
    });

    expect(provider).toBeInstanceOf(OpenAIProvider);
  });

  it('applies custom headers with env var interpolation', () => {
    const originalEnv = process.env.HICAP_SECRET;
    process.env.HICAP_SECRET = 'test-secret-value';

    try {
      const provider = createProvider({
        providerId: 'opengateway',
        apiKey: 'test-key',
        baseURL: 'https://opengateway.gitlawb.com/v1',
        model: 'mimo-v2.5-pro',
      });

      expect(provider).toBeInstanceOf(OpenAIProvider);
    } finally {
      if (originalEnv === undefined) {
        delete process.env.HICAP_SECRET;
      } else {
        process.env.HICAP_SECRET = originalEnv;
      }
    }
  });

  it('anthropic-compatible transport is not yet supported', () => {
    const anthropicDef: ProviderDefinition = {
      id: 'anthropic-test',
      name: 'Anthropic Test',
      kind: 'provider',
      description: 'Test anthropic-compatible provider',
      baseURL: 'https://api.anthropic.com',
      defaultModel: 'claude-sonnet-4',
      transportConfig: {
        kind: 'anthropic-compatible',
      },
    };

    // TODO: implement anthropic-compatible transport
    expect(isProviderRuntimeSupported(anthropicDef)).toBe(false);
  });

  it('rejects unsupported transport kinds', () => {
    const nativeDefinition: ProviderDefinition = {
      id: 'native-test',
      name: 'Native Test',
      kind: 'provider',
      description: 'Unsupported native test provider',
      baseURL: 'https://example.com',
      defaultModel: 'native-model',
      transportConfig: {
        kind: 'gemini-native' as any,
      },
    };

    expect(isProviderRuntimeSupported(nativeDefinition)).toBe(false);
  });

  it('uses catalog defaults when provider commands only return a provider id', () => {
    const config = resolveProviderCommandConfig({
      provider_id: 'opengateway',
      api_key: 'ogw_live_test',
    });

    expect(config).toEqual({
      providerId: 'opengateway',
      apiKey: 'ogw_live_test',
      baseURL: 'https://opengateway.gitlawb.com/v1',
      model: 'mimo-v2.5-pro',
    });
  });

  it('parses quoted provider command arguments', () => {
    expect(parseProviderCommand('"C:\\Program Files\\Zero\\provider.exe" --json "{\\"provider_id\\":\\"opengateway\\"}"')).toEqual([
      'C:\\Program Files\\Zero\\provider.exe',
      '--json',
      '{"provider_id":"opengateway"}',
    ]);
  });

  it('rejects unterminated provider command quotes', () => {
    expect(() => parseProviderCommand('provider-command "unterminated')).toThrow('Unterminated');
  });

  it('discovers OpenAI-compatible models and merges them with static catalog models', async () => {
    const originalFetch = globalThis.fetch;
    const originalConfigDir = process.env.ZERO_CONFIG_DIR;
    const testConfigDir = testCacheDir('provider-catalog');
    const requests: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];

    cleanupTestCacheDir(testConfigDir);
    process.env.ZERO_CONFIG_DIR = testConfigDir;

    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      requests.push({ input, init });
      return new Response(JSON.stringify({
        data: [
          { id: 'discovered-model' },
          { id: 'mimo-v2.5-pro' },
        ],
      }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      });
    }) as typeof fetch;

    try {
      const result = await discoverModelsForProvider('opengateway', {
        apiKey: 'ogw_live_test',
        forceRefresh: true,
      });

      expect(result?.source).toBe('network');
      expect(result?.models.map((model) => model.apiName)).toContain('mimo-v2.5-pro');
      expect(result?.models.map((model) => model.apiName)).toContain('gpt-4o');
      expect(result?.models.map((model) => model.apiName)).toContain('discovered-model');
      expect(String(requests[0]?.input)).toBe('https://opengateway.gitlawb.com/v1/models');
      expect((requests[0]?.init?.headers as Record<string, string>)?.authorization).toBe('Bearer ogw_live_test');
      expect(readFileSync(join(testConfigDir, 'model-cache.json'), 'utf-8')).not.toContain('ogw_live_test');
    } finally {
      globalThis.fetch = originalFetch;
      if (originalConfigDir === undefined) {
        delete process.env.ZERO_CONFIG_DIR;
      } else {
        process.env.ZERO_CONFIG_DIR = originalConfigDir;
      }
      cleanupTestCacheDir(testConfigDir);
    }
  });

  it('resolves effective transport config with per-model overrides', () => {
    const opengateway = getProviderDefinition('opengateway');
    expect(opengateway).toBeDefined();

    const gpt4oModel = {
      id: 'gpt-4o-og',
      apiName: 'gpt-4o',
      globalModelId: 'gpt-4o',
      contextWindow: 32000,
      maxOutputTokens: 4096,
      transportOverrides: {
        maxTokensField: 'max_tokens' as const,
        removeBodyFields: ['store'],
      },
    };

    const effective = resolveEffectiveTransportConfig(opengateway!, gpt4oModel);
    expect(effective).toBeDefined();
    // Per-model override should take precedence
    expect(effective?.maxTokensField).toBe('max_tokens');
    // removeBodyFields should be unioned (definition + model)
    expect(effective?.removeBodyFields).toContain('store');
    expect(effective?.removeBodyFields).toContain('stream_options');
  });

  it('first-party providers auto-expose their owned global models', () => {
    const openai = getProviderDefinition('openai');
    expect(openai).toBeDefined();
    expect(openai?.isFirstParty).toBe(true);

    const models = getModelsForProvider(openai!);
    const modelIds = models.map((m) => m.apiName);

    expect(modelIds).toContain('gpt-4o');
    expect(modelIds).toContain('gpt-4o-mini');
    expect(modelIds).toContain('o1');
    expect(modelIds).toContain('o1-mini');
    expect(modelIds).not.toContain('claude-sonnet-4');
    expect(modelIds).not.toContain('deepseek-v3');

    const gpt4o = models.find((m) => m.apiName === 'gpt-4o');
    expect(gpt4o?.providerId).toBe('openai');
    expect(gpt4o?.providerKind).toBe('provider');
  });

  it('non-first-party providers only show their own catalog models', () => {
    const opengateway = getProviderDefinition('opengateway');
    expect(opengateway?.isFirstParty).toBeUndefined();

    const models = getModelsForProvider(opengateway!);
    const modelIds = models.map((m) => m.apiName);

    expect(modelIds).toContain('mimo-v2.5-pro');
    expect(modelIds).toContain('gpt-4o'); // via globalModelId override
    expect(modelIds).not.toContain('claude-sonnet-4');
    expect(modelIds).not.toContain('deepseek-v3');
  });

  it('returns definition-level transport config when no model overrides exist', () => {
    const opengateway = getProviderDefinition('opengateway');
    const effective = resolveEffectiveTransportConfig(opengateway!, undefined);

    expect(effective?.maxTokensField).toBe('max_completion_tokens');
    expect(effective?.removeBodyFields).toEqual(['store', 'stream_options']);
  });

  it('does not cache first-time discovery failures as fresh results', async () => {
    const originalFetch = globalThis.fetch;
    const originalConfigDir = process.env.ZERO_CONFIG_DIR;
    const testConfigDir = testCacheDir('provider-catalog-failure');
    let requestCount = 0;

    cleanupTestCacheDir(testConfigDir);
    process.env.ZERO_CONFIG_DIR = testConfigDir;

    globalThis.fetch = (async () => {
      requestCount += 1;
      return new Response(JSON.stringify({ error: 'temporary failure' }), {
        status: 503,
        headers: { 'content-type': 'application/json' },
      });
    }) as typeof fetch;

    try {
      const first = await discoverModelsForProvider('opengateway', {
        apiKey: 'ogw_live_test',
      });
      const second = await discoverModelsForProvider('opengateway', {
        apiKey: 'ogw_live_test',
      });

      expect(first?.source).toBe('error');
      expect(second?.source).toBe('error');
      expect(requestCount).toBe(2);
      expect(existsSync(join(testConfigDir, 'model-cache.json'))).toBe(false);
    } finally {
      globalThis.fetch = originalFetch;
      if (originalConfigDir === undefined) {
        delete process.env.ZERO_CONFIG_DIR;
      } else {
        process.env.ZERO_CONFIG_DIR = originalConfigDir;
      }
      cleanupTestCacheDir(testConfigDir);
    }
  });

  it('discovers Ollama models via /api/tags', async () => {
    const originalFetch = globalThis.fetch;
    const originalConfigDir = process.env.ZERO_CONFIG_DIR;
    const testConfigDir = testCacheDir('ollama-discovery');

    cleanupTestCacheDir(testConfigDir);
    process.env.ZERO_CONFIG_DIR = testConfigDir;

    globalThis.fetch = (async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/api/tags')) {
        return new Response(JSON.stringify({
          models: [
            { name: 'llama3.2:latest' },
            { name: 'mistral:7b' },
          ],
        }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        });
      }
      return new Response('not found', { status: 404 });
    }) as typeof fetch;

    try {
      const result = await discoverModelsForProvider('ollama', {
        forceRefresh: true,
      });

      expect(result?.source).toBe('network');
      expect(result?.models.map((m) => m.apiName)).toContain('llama3.2:latest');
      expect(result?.models.map((m) => m.apiName)).toContain('mistral:7b');
      // Ollama models should be tagged as local tier
      const llama = result?.models.find((m) => m.apiName === 'llama3.2:latest');
      expect(llama?.tier).toBe('local');
    } finally {
      globalThis.fetch = originalFetch;
      if (originalConfigDir === undefined) {
        delete process.env.ZERO_CONFIG_DIR;
      } else {
        process.env.ZERO_CONFIG_DIR = originalConfigDir;
      }
      cleanupTestCacheDir(testConfigDir);
    }
  });

  it('returns stale-cache on network failure when cache exists', async () => {
    const originalFetch = globalThis.fetch;
    const originalConfigDir = process.env.ZERO_CONFIG_DIR;
    const testConfigDir = testCacheDir('stale-cache-recovery');
    let fetchCount = 0;

    cleanupTestCacheDir(testConfigDir);
    process.env.ZERO_CONFIG_DIR = testConfigDir;

    // First call succeeds and populates cache
    globalThis.fetch = (async () => {
      fetchCount++;
      return new Response(JSON.stringify({
        data: [{ id: 'cached-model' }],
      }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      });
    }) as typeof fetch;

    try {
      // Populate cache
      await discoverModelsForProvider('opengateway', {
        apiKey: 'ogw_test',
        forceRefresh: true,
      });

      // Now make fetch fail
      globalThis.fetch = (async () => {
        fetchCount++;
        return new Response(JSON.stringify({ error: 'down' }), {
          status: 503,
          headers: { 'content-type': 'application/json' },
        });
      }) as typeof fetch;

      // Should return stale-cache, not error
      const result = await discoverModelsForProvider('opengateway', {
        apiKey: 'ogw_test',
        forceRefresh: true,
      });

      expect(result?.source).toBe('stale-cache');
      expect(result?.stale).toBe(true);
      // Should still have the cached models
      expect(result?.models.map((m) => m.apiName)).toContain('cached-model');
    } finally {
      globalThis.fetch = originalFetch;
      if (originalConfigDir === undefined) {
        delete process.env.ZERO_CONFIG_DIR;
      } else {
        process.env.ZERO_CONFIG_DIR = originalConfigDir;
      }
      cleanupTestCacheDir(testConfigDir);
    }
  });
});

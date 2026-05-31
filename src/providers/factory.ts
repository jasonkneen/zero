import { OpenAIProvider } from './openai';
import type { Provider } from './types';
import type { ProviderConfig } from '../config/provider';
import { getProviderDefinition, resolveEffectiveTransportConfig, getGlobalModelDefinition, getModelsForProvider } from './catalog';
import type { ProviderDefinition } from './catalog/types';
import { buildAuthHeaders, resolveHeaders } from './auth';

export function createProvider(config: ProviderConfig): Provider {
  const definition = config.providerId ? getProviderDefinition(config.providerId) : undefined;
  if (definition && !isProviderRuntimeSupported(definition)) {
    throw new Error(
      `Provider "${definition.name}" uses unsupported transport "${definition.transportConfig?.kind}". ` +
      'Supported: openai-compatible'
    );
  }

  // Resolve per-model transport overrides
  const globalModel = getGlobalModelDefinition(config.model);
  const catalogModel = definition && getModelsForProvider(definition).find(
    (m) => m.apiName === config.model || m.id === config.model
  );
  const modelDef = catalogModel ?? globalModel;

  const effectiveTransport = definition
    ? resolveEffectiveTransportConfig(definition, modelDef)
    : undefined;

  const apiKey = config.apiKey || '';
  const rawHeaders = {
    ...(effectiveTransport?.headers ?? {}),
    ...buildAuthHeaders(apiKey, effectiveTransport?.authHeader),
  };
  const headers = resolveHeaders(rawHeaders);

  return new OpenAIProvider({
    apiKey,
    baseURL: config.baseURL,
    model: config.model,
    defaultHeaders: Object.keys(headers).length > 0 ? headers : undefined,
    maxTokensField: effectiveTransport?.maxTokensField,
    removeBodyFields: effectiveTransport?.removeBodyFields,
  });
}

export function isProviderRuntimeSupported(definition: ProviderDefinition): boolean {
  const kind = definition.transportConfig?.kind ?? 'openai-compatible';
  // TODO: implement anthropic-compatible transport
  return kind === 'openai-compatible';
}

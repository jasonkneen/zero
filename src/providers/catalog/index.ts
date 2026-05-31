import { existsSync, readdirSync, readFileSync } from 'fs';
import { join } from 'path';
import { parse } from 'smol-toml';
import type {
  ModelDefinition,
  ProviderDefinition,
  ResolvedModelDefinition,
  TransportConfig,
} from './types';

type EmbeddedFile = Blob & { name: string };

function parseTomlFile(content: string, directory: 'definitions' | 'models'): unknown[] {
  const parsed = parse(content);

  if (directory === 'definitions') {
    // Each definition file is a single object
    return [parsed];
  }

  // Model files use [[model]] array-of-tables syntax.
  // Parsed result is { model: [...] }. Extract the array.
  if (parsed && typeof parsed === 'object' && 'model' in parsed) {
    const models = (parsed as any).model;
    return Array.isArray(models) ? models : [models];
  }

  return [parsed];
}

async function readTomlFiles<T>(directory: 'definitions' | 'models'): Promise<T[]> {
  const directoryPath = join(import.meta.dir, directory);
  let items: unknown[] = [];

  if (existsSync(directoryPath)) {
    items = readdirSync(directoryPath)
      .filter((file) => file.endsWith('.toml'))
      .flatMap((file) => parseTomlFile(readFileSync(join(directoryPath, file), 'utf-8'), directory));
  } else {
    // Fallback for bundled/embedded files
    const embeddedFiles = (globalThis as any).Bun?.embeddedFiles as EmbeddedFile[] | undefined;
    if (!embeddedFiles) {
      console.warn(`[providers] No ${directory} directory and no embedded files found`);
      return [];
    }

    const tomlFiles = embeddedFiles.filter((file) => file.name.endsWith('.toml'));
    if (tomlFiles.length === 0) {
      console.warn(`[providers] No .toml files found in embedded files for ${directory}`);
      return [];
    }

    items = (await Promise.all(
      tomlFiles.map(async (file) => parseTomlFile(await file.text(), directory))
    )).flat();
  }

  if (directory === 'definitions') {
    return items.filter(isProviderDefinition) as T[];
  }

  return items.filter(isModelDefinition) as T[];
}

function isProviderDefinition(value: any): value is ProviderDefinition {
  return (
    value &&
    typeof value === 'object' &&
    typeof value.id === 'string' &&
    typeof value.name === 'string' &&
    (value.kind === 'provider' || value.kind === 'gateway' || value.kind === 'localhost') &&
    typeof value.baseURL === 'string' &&
    typeof value.defaultModel === 'string'
  );
}

function isModelDefinition(value: any): value is ModelDefinition {
  return (
    value &&
    typeof value === 'object' &&
    typeof value.id === 'string' &&
    // Positive: model definitions have these fields
    !('baseURL' in value) &&
    !('defaultModel' in value) &&
    // Negative: provider definitions have 'kind' as a string enum
    // Models don't have 'kind' at the top level
    !('kind' in value && typeof value.kind === 'string' &&
      ['provider', 'gateway', 'localhost'].includes(value.kind))
  );
}

function validateModelDefinition(model: ModelDefinition): void {
  if (model.contextWindow != null && typeof model.contextWindow !== 'number') {
    console.warn(`[providers] Model "${model.id}": contextWindow should be a number, got ${typeof model.contextWindow}`);
  }
  if (model.maxOutputTokens != null && typeof model.maxOutputTokens !== 'number') {
    console.warn(`[providers] Model "${model.id}": maxOutputTokens should be a number, got ${typeof model.maxOutputTokens}`);
  }
  if (model.defaultTemperature != null && typeof model.defaultTemperature !== 'number') {
    console.warn(`[providers] Model "${model.id}": defaultTemperature should be a number, got ${typeof model.defaultTemperature}`);
  }
  if (model.capabilities != null && typeof model.capabilities !== 'object') {
    console.warn(`[providers] Model "${model.id}": capabilities should be an object, got ${typeof model.capabilities}`);
  }
  if (model.cost != null) {
    if (typeof model.cost !== 'object') {
      console.warn(`[providers] Model "${model.id}": cost should be an object, got ${typeof model.cost}`);
    } else {
      if (typeof model.cost.inputPerMillion !== 'number') {
        console.warn(`[providers] Model "${model.id}": cost.inputPerMillion should be a number`);
      }
      if (typeof model.cost.outputPerMillion !== 'number') {
        console.warn(`[providers] Model "${model.id}": cost.outputPerMillion should be a number`);
      }
    }
  }
}

/**
 * Resolve a model entry that references a global model via globalModelId.
 * The global model's fields are used as base; the entry's explicit fields override.
 */
function resolveGlobalModelReference(
  model: ModelDefinition,
  globalModelById: Map<string, ModelDefinition>
): ModelDefinition {
  if (!model.globalModelId) return model;

  const globalModel = globalModelById.get(model.globalModelId);
  if (!globalModel) return model;

  return {
    ...globalModel,
    ...model,
    // Only override id/apiName/name if the entry explicitly sets them
    id: model.id ?? globalModel.id,
    apiName: model.apiName ?? model.id ?? globalModel.apiName ?? globalModel.id,
    name: model.name ?? globalModel.name,
    tier: model.tier ?? globalModel.tier,
    description: model.description ?? globalModel.description,
    classification: model.classification ?? globalModel.classification,
    capabilities: {
      ...globalModel.capabilities,
      ...model.capabilities,
    },
    transportOverrides: model.transportOverrides ?? globalModel.transportOverrides,
  };
}

/**
 * Normalize a definition with defaults, and resolve vendorId links.
 */
function normalizeDefinition(
  definition: ProviderDefinition,
  definitionById: Map<string, ProviderDefinition>
): ProviderDefinition {
  const setup = definition.setup ?? {
    requiresAuth: definition.apiKeyRequired !== false,
    authMode: definition.apiKeyRequired === false ? 'none' : 'api-key',
    credentialEnvVars: definition.credentialEnvVars,
  } as const;

  const catalog = definition.catalog ?? {
    source: definition.models ? 'static' : 'dynamic',
    models: definition.models,
  } as const;

  // Resolve vendorId: inherit transport config from linked vendor
  let transportConfig = definition.transportConfig;
  if (definition.vendorId && !transportConfig) {
    const vendor = definitionById.get(definition.vendorId);
    if (vendor?.transportConfig) {
      transportConfig = vendor.transportConfig;
    }
  }

  return {
    ...definition,
    setup,
    transportConfig: transportConfig ?? {
      kind: 'openai-compatible',
    },
    catalog,
    apiKeyRequired: definition.apiKeyRequired ?? setup.requiresAuth,
    credentialEnvVars: definition.credentialEnvVars ?? setup.credentialEnvVars,
    models: definition.models ?? catalog.models,
  };
}

export function resolveModel(
  model: ModelDefinition,
  definition?: ProviderDefinition
): ResolvedModelDefinition {
  return {
    ...model,
    apiName: model.apiName ?? model.id,
    providerId: definition?.id,
    providerName: definition?.name,
    providerKind: definition?.kind,
  };
}

// First pass: raw definitions for vendorId resolution
const rawDefinitions = (await readTomlFiles<ProviderDefinition>('definitions'))
  .filter((definition): definition is ProviderDefinition => Boolean(definition));

const rawDefinitionById = new Map(rawDefinitions.map((d) => [d.id, d]));

// Second pass: normalize with vendorId resolution
const definitions = rawDefinitions
  .map((definition) => normalizeDefinition(definition, rawDefinitionById))
  .sort((a, b) => a.name.localeCompare(b.name));

const globalModels = (await readTomlFiles<ModelDefinition>('models'))
  .sort((a, b) => a.id.localeCompare(b.id));

// Validate parsed models
for (const model of globalModels) {
  validateModelDefinition(model);
}

const definitionById = new Map(definitions.map((definition) => [definition.id, definition]));
const globalModelById = new Map(globalModels.map((model) => [model.id, model]));

export function listProviderDefinitions(): ProviderDefinition[] {
  return [...definitions];
}

export function getProviderDefinition(id: string): ProviderDefinition | undefined {
  return definitionById.get(id);
}

/**
 * Get resolved models for a specific provider definition.
 */
export function getModelsForProvider(definition: ProviderDefinition): ResolvedModelDefinition[] {
  const catalogModels = definition.catalog?.models ?? definition.models ?? [];

  const resolvedCatalogModels = catalogModels.map((model) =>
    resolveGlobalModelReference(model, globalModelById)
  );

  if (definition.isFirstParty) {
    const catalogApiNames = new Set(
      resolvedCatalogModels.map((m) => (m.apiName ?? m.id).toLowerCase())
    );
    const additionalGlobals = globalModels.filter(
      (m) => m.ownerProviderId === definition.id && !catalogApiNames.has(m.id.toLowerCase())
    );
    return [
      ...resolvedCatalogModels.map((m) => resolveModel(m, definition)),
      ...additionalGlobals.map((m) => resolveModel(m, definition)),
    ];
  }

  return resolvedCatalogModels.map((m) => resolveModel(m, definition));
}

/**
 * Resolve the effective transport config for a specific model within a provider.
 * Definition-level config is the base; per-model overrides are merged on top.
 * removeBodyFields is unioned (not replaced).
 */
export function resolveEffectiveTransportConfig(
  definition: ProviderDefinition,
  model?: ModelDefinition
): TransportConfig | undefined {
  const base = definition.transportConfig;
  if (!base) return undefined;
  if (!model?.transportOverrides) return base;

  const overrides = model.transportOverrides;
  return {
    ...base,
    ...overrides,
    headers: {
      ...(base.headers ?? {}),
      ...(overrides.headers ?? {}),
    },
    removeBodyFields: [
      ...new Set([
        ...(base.removeBodyFields ?? []),
        ...(overrides.removeBodyFields ?? []),
      ]),
    ],
  };
}

export function listModelDefinitions(): ResolvedModelDefinition[] {
  const routeModels = definitions.flatMap((definition) =>
    getModelsForProvider(definition)
  );

  // Deduplicate by apiName, preferring provider-attributed versions
  const allModels = [...globalModels.map((model) => resolveModel(model)), ...routeModels];
  const seen = new Map<string, ResolvedModelDefinition>();
  for (const model of allModels) {
    const key = model.apiName.toLowerCase();
    // Prefer versions with providerId over bare global models
    if (!seen.has(key) || (model.providerId && !seen.get(key)?.providerId)) {
      seen.set(key, model);
    }
  }
  return [...seen.values()];
}

export function getGlobalModelDefinition(id: string): ModelDefinition | undefined {
  return globalModelById.get(id);
}

export function listGlobalModels(): ModelDefinition[] {
  return [...globalModels];
}

export function listProvidersByKind(kind: ProviderDefinition['kind']): ProviderDefinition[] {
  return definitions.filter((d) => d.kind === kind);
}

export function listFirstPartyProviders(): ProviderDefinition[] {
  return definitions.filter((d) => d.isFirstParty === true);
}

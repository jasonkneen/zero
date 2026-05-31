export type ProviderKind = 'provider' | 'gateway' | 'localhost';
export type AuthMode = 'api-key' | 'oauth' | 'adc' | 'token' | 'none';
export type TransportKind =
  | 'openai-compatible'
  | 'anthropic-compatible';
export type ModelCatalogSource = 'static' | 'dynamic' | 'hybrid';
export type DiscoveryRefreshMode = 'manual' | 'on-open' | 'background-if-stale' | 'startup';
export type ModelDiscoveryKind = 'openai-compatible' | 'ollama' | 'custom';
export type MaxTokensField = 'max_tokens' | 'max_completion_tokens';

export interface ModelDefinition {
  id: string;
  name?: string;
  apiName?: string;
  label?: string;
  default?: boolean;
  hidden?: boolean;
  globalModelId?: string;
  ownerProviderId?: string;
  classification?: ('chat' | 'reasoning' | 'vision' | 'coding')[];
  tier?: 'first-party' | 'hosted' | 'local' | 'community';
  description?: string;
  capabilities?: CapabilityFlags;
  contextWindow?: number;
  maxOutputTokens?: number;
  defaultTemperature?: number;
  temperatureRange?: TemperatureRange;
  cost?: ModelCost;
  transportOverrides?: Partial<TransportConfig>;
  notes?: string;
}

export interface CapabilityFlags {
  supportsVision?: boolean;
  supportsStreaming?: boolean;
  supportsFunctionCalling?: boolean;
  supportsJsonMode?: boolean;
  supportsReasoning?: boolean;
  supportsPreciseTokenCount?: boolean;
  supportsEmbeddings?: boolean;
  supportsTemperature?: boolean;
}

export interface TemperatureRange {
  min: number;
  max: number;
}

export interface ModelCost {
  inputPerMillion: number;
  outputPerMillion: number;
  cachePerMillion?: number;
  currency?: string;
}

export interface AuthHeaderConfig {
  name: string;
  scheme?: 'bearer' | 'raw';
}

export interface TransportConfig {
  kind: TransportKind;
  headers?: Record<string, string>;
  authHeader?: AuthHeaderConfig;
  maxTokensField?: MaxTokensField;
  removeBodyFields?: string[];
  endpointPath?: string;
}

export interface ModelDiscoveryConfig {
  kind: ModelDiscoveryKind;
  requiresAuth?: boolean;
  path?: string;
}

export interface ModelCatalogConfig {
  source: ModelCatalogSource;
  discovery?: ModelDiscoveryConfig;
  discoveryCacheTtl?: string | number;
  discoveryRefreshMode?: DiscoveryRefreshMode;
  allowManualRefresh?: boolean;
  models?: ModelDefinition[];
}

export interface SetupMetadata {
  requiresAuth: boolean;
  authMode: AuthMode;
  credentialEnvVars?: string[];
  setupPrompt?: string;
}

export interface StartupMetadata {
  autoDetectable?: boolean;
  probeReadiness?: 'ollama-generation' | 'openai-compatible-models';
  enablementEnvVar?: string;
}

export interface UsageMetadata {
  supported: boolean;
  silentlyIgnore?: boolean;
}

export interface ValidationMetadata {
  kind: 'credential-env';
  credentialEnvVars: string[];
  missingCredentialMessage?: string;
  matchBaseUrlHosts?: string[];
}

export interface PresetBadge {
  text: string;
  color?: string;
}

export interface ProviderPresetMetadata {
  id: string;
  description: string;
  label?: string;
  apiKeyEnvVars?: string[];
  fallbackBaseUrl?: string;
  fallbackModel?: string;
  badge?: PresetBadge;
}

export interface ProviderDefinition {
  id: string;
  name: string;
  kind: ProviderKind;
  vendorId?: string;
  category?: 'local' | 'hosted' | 'aggregating';
  description: string;
  baseURL: string;
  defaultModel: string;
  supportsModelRouting?: boolean;
  setup?: SetupMetadata;
  startup?: StartupMetadata;
  transportConfig?: TransportConfig;
  catalog?: ModelCatalogConfig;
  validation?: ValidationMetadata;
  usage?: UsageMetadata;
  preset?: ProviderPresetMetadata;
  isFirstParty?: boolean;
  apiKeyLabel?: string;
  apiKeyPlaceholder?: string;
  apiKeyRequired?: boolean;
  credentialEnvVars?: string[];
  models?: ModelDefinition[];
}

export interface ResolvedModelDefinition extends ModelDefinition {
  apiName: string;
  providerId?: string;
  providerName?: string;
  providerKind?: ProviderKind;
}

export interface ModelDiscoveryResult {
  providerId: string;
  models: ResolvedModelDefinition[];
  source: 'static' | 'network' | 'cache' | 'stale-cache' | 'error';
  stale: boolean;
  error?: string;
}

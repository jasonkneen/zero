# zero

To install dependencies:

```bash
bun install
```

To run:

```bash
bun run dev
```

To list built-in provider catalog entries:

```bash
bun run src/index.ts providers catalog
```

## Provider catalog

Provider catalog entries are TOML files loaded from
`src/providers/catalog/definitions`. Global model metadata is loaded separately
from `src/providers/catalog/models`.

Use a provider definition file for every provider, gateway, or local endpoint.
Only add a global model file when a first-party provider owns new model
metadata. Gateways should normally reference global models with
`globalModelId` and put route-specific names, limits, costs, and overrides on
their own `[[catalog.models]]` entries.

| What you're adding | Files to touch |
|--------------------|----------------|
| First-party provider with no new global models | `definitions/<provider>.toml` |
| First-party provider with owned global models | `definitions/<provider>.toml` and `models/<family>.toml` |
| Gateway or hosted proxy | `definitions/<gateway>.toml` |
| Local endpoint such as Ollama | `definitions/<local>.toml` |

### Provider definitions

Every provider definition must include `id`, `name`, `kind`, `description`,
`baseURL`, and `defaultModel`.

```toml
id = "my-gateway"
name = "My Gateway"
kind = "gateway"
description = "OpenAI-compatible model gateway"
category = "aggregating"
baseURL = "https://api.example.com/v1"
defaultModel = "vendor/model-name"
supportsModelRouting = true
apiKeyLabel = "My Gateway API key"
apiKeyPlaceholder = "gw_live_..."
apiKeyRequired = true

[setup]
requiresAuth = true
authMode = "api-key"
credentialEnvVars = ["MY_GATEWAY_API_KEY"]

[validation]
kind = "credential-env"
credentialEnvVars = ["MY_GATEWAY_API_KEY"]
missingCredentialMessage = "MY_GATEWAY_API_KEY is required."
matchBaseUrlHosts = ["api.example.com"]

[transportConfig]
kind = "openai-compatible"
maxTokensField = "max_completion_tokens"
removeBodyFields = ["store", "stream_options"]
authHeader.name = "authorization"
authHeader.scheme = "bearer"
headers.X-Trace-Source = "zero"
headers.X-Tenant = "$MY_GATEWAY_TENANT"

[catalog]
source = "hybrid"
discoveryCacheTtl = "1h"
discoveryRefreshMode = "manual"
allowManualRefresh = true
discovery.kind = "openai-compatible"
discovery.requiresAuth = true
discovery.path = "/models"

[[catalog.models]]
globalModelId = "gpt-4o"
apiName = "openai/gpt-4o"
contextWindow = 32000
maxOutputTokens = 4096
cost.inputPerMillion = 2.50
cost.outputPerMillion = 10.00
cost.cachePerMillion = 0.25
cost.currency = "USD"

[usage]
supported = false

[preset]
id = "my-gateway"
description = "My Gateway hosted models"
apiKeyEnvVars = ["MY_GATEWAY_API_KEY"]
fallbackBaseUrl = "https://api.example.com/v1"
fallbackModel = "vendor/model-name"
badge.text = "HOSTED"
badge.color = "success"
```

| Field | Required | Accepted values or behavior |
|-------|----------|-----------------------------|
| `id` | Yes | Stable provider ID. |
| `name` | Yes | Human-readable provider name. |
| `kind` | Yes | `provider`, `gateway`, or `localhost`. |
| `description` | Yes | Short provider description. |
| `baseURL` | Yes | Provider API base URL. |
| `defaultModel` | Yes | Model used when the user does not choose one. |
| `category` | No | `local`, `hosted`, or `aggregating`. |
| `vendorId` | No | Inherits another provider's `transportConfig` when this definition omits one. |
| `isFirstParty` | No | Adds owned global models from `models/*.toml` to this provider's model list. |
| `supportsModelRouting` | No | Metadata for providers that can route multiple model families. |
| `apiKeyLabel` / `apiKeyPlaceholder` | No | UI labels for credential entry. |
| `apiKeyRequired` | No | `false` for local/no-auth providers; otherwise defaults from `setup.requiresAuth`. |
| `credentialEnvVars` | No | Legacy top-level credential env list; `setup.credentialEnvVars` is preferred. |

### Setup, validation, usage, and presets

`setup` describes how `/provider` should ask for credentials. Supported
`authMode` values are `api-key`, `oauth`, `adc`, `token`, and `none`.

| Setup field | Required | Accepted values or behavior |
|-------------|----------|-----------------------------|
| `requiresAuth` | Yes when `setup` is present | `true` for credentialed providers; `false` for no-auth local endpoints. |
| `authMode` | Yes when `setup` is present | `api-key`, `oauth`, `adc`, `token`, or `none`. |
| `credentialEnvVars` | No | Env vars that may contain provider credentials. |
| `setupPrompt` | No | Optional credential setup prompt text. |

`validation` currently supports `kind = "credential-env"` with
`credentialEnvVars`, an optional `missingCredentialMessage`, and optional
`matchBaseUrlHosts` for identifying matching configured providers.

| Validation field | Required | Accepted values or behavior |
|------------------|----------|-----------------------------|
| `kind` | Yes when `validation` is present | Currently `credential-env`. |
| `credentialEnvVars` | Yes | Env vars checked for credentials. |
| `missingCredentialMessage` | No | Message shown when required credentials are missing. |
| `matchBaseUrlHosts` | No | Hostnames used to match an existing provider config. |

`usage` is provider metadata. Set `supported = false` for providers where usage
accounting should not be expected. `silentlyIgnore = true` suppresses usage
noise for providers such as local runtimes.

| Usage field | Required | Accepted values or behavior |
|-------------|----------|-----------------------------|
| `supported` | Yes when `usage` is present | Whether usage accounting is expected for this provider. |
| `silentlyIgnore` | No | Suppress usage-accounting noise when unsupported. |

`preset` provides fallback values and display metadata for provider setup:
`id`, `description`, `label`, `apiKeyEnvVars`, `fallbackBaseUrl`,
`fallbackModel`, and optional `preset.badge.text` / `preset.badge.color`.

Keep `preset` and `preset.badge` in one `[preset]` block with dotted
`badge.*` keys.

| Preset field | Required | Accepted values or behavior |
|--------------|----------|-----------------------------|
| `id` | Yes when `preset` is present | Stable preset ID. |
| `description` | Yes when `preset` is present | Preset description. |
| `label` | No | Optional UI label. |
| `apiKeyEnvVars` | No | Env vars to look at for provider credentials. |
| `fallbackBaseUrl` | No | Base URL used when config does not provide one. |
| `fallbackModel` | No | Model used when config does not provide one. |
| `badge.text` / `badge.color` | No | Optional display badge metadata. |

### Transport config

`transportConfig.kind` accepts `openai-compatible` and
`anthropic-compatible`. OpenAI-compatible providers are supported at runtime.
Anthropic-compatible descriptors are recognized by the catalog, but runtime
creation currently rejects them until that transport is implemented.

Keep `transportConfig`, `transportConfig.authHeader`, and
`transportConfig.headers` in one `[transportConfig]` block with dotted keys.

| Field | Required | Accepted values or behavior |
|-------|----------|-----------------------------|
| `kind` | Yes when `transportConfig` is present | `openai-compatible` or `anthropic-compatible`. |
| `headers.*` | No | Static headers. Values beginning with `$` are read from `process.env`; unset env vars are omitted. |
| `authHeader.name` | No | Header name used for API key auth. Defaults to `authorization` during discovery. |
| `authHeader.scheme` | No | `bearer` sends `Bearer <key>`; `raw` sends the key value directly. Defaults to `bearer`. |
| `maxTokensField` | No | `max_tokens` or `max_completion_tokens`. |
| `removeBodyFields` | No | Request body fields stripped before sending to the provider. |
| `endpointPath` | No | Optional transport endpoint path metadata. |

For runtime OpenAI-compatible calls, the OpenAI SDK supplies its normal
`Authorization` header unless a custom raw or non-Authorization `authHeader` is
configured. For model discovery, Zero builds fetch headers directly and sends
the configured auth header when discovery requires auth.

### Model catalog

`catalog.source` accepts `static`, `dynamic`, or `hybrid`.

Keep `catalog` and `catalog.discovery` in one `[catalog]` block with dotted
`discovery.*` keys.

| Field | Required | Accepted values or behavior |
|-------|----------|-----------------------------|
| `source` | Yes when `catalog` is present | `static`, `dynamic`, or `hybrid`. |
| `models` | No | Static `[[catalog.models]]` entries. These are returned before discovered models. |
| `discovery.kind` | No | `openai-compatible`, `ollama`, or `custom`; only OpenAI-compatible and Ollama discovery execute today. |
| `discovery.path` | No | Discovery path appended to `baseURL`; defaults to `/models`. |
| `discovery.requiresAuth` | No | `false` skips auth headers and cache key partitioning by API key. |
| `discoveryCacheTtl` | No | Milliseconds as a number, or a string ending in `m`, `h`, or `d`; defaults to `1h`. |
| `discoveryRefreshMode` | No | `manual`, `on-open`, `background-if-stale`, or `startup` metadata. |
| `allowManualRefresh` | No | Metadata for UI refresh affordances. |

OpenAI-compatible discovery accepts either `{ data: [{ id: "..." }] }` or a raw
array of model IDs/objects. Ollama discovery calls `/api/tags` on the base URL
without the `/v1` suffix and reads `models[].name`.

### Global model files

Files under `src/providers/catalog/models` use TOML `[[model]]` entries. Each
entry must stay as one model block. Nested model subtables such as
`[model.cost]`, `[model.capabilities]`, or `[catalog.models.cost]` are not
allowed; use dotted keys inside the `[[model]]` or `[[catalog.models]]` block.

```toml
[[model]]
id = "gpt-4o"
name = "GPT-4o"
apiName = "gpt-4o"
ownerProviderId = "openai"
tier = "first-party"
description = "OpenAI flagship multimodal model"
classification = ["chat", "vision", "coding"]
contextWindow = 128000
maxOutputTokens = 16384
defaultTemperature = 0.7
temperatureRange.min = 0
temperatureRange.max = 2
capabilities.supportsVision = true
capabilities.supportsStreaming = true
capabilities.supportsFunctionCalling = true
capabilities.supportsJsonMode = true
capabilities.supportsReasoning = false
capabilities.supportsPreciseTokenCount = true
capabilities.supportsEmbeddings = false
capabilities.supportsTemperature = true
cost.inputPerMillion = 2.50
cost.outputPerMillion = 10.00
cost.cachePerMillion = 0.25
cost.currency = "USD"
notes = "Optional maintainer note."
```

### Model entries

Global `[[model]]` entries and provider `[[catalog.models]]` entries share the
same shape. Provider entries may define local models directly, or reference a
global model with `globalModelId` and override route-specific fields.

| Field | Required | Accepted values or behavior |
|-------|----------|-----------------------------|
| `id` | Yes for global models; no for provider entries with `globalModelId` | Stable catalog ID. |
| `apiName` | No | Exact model name sent to the API. Defaults to `id`; with `globalModelId`, defaults to the global model ID unless overridden. |
| `globalModelId` | No | Inherits metadata from a global model, then applies provider-entry overrides. |
| `ownerProviderId` | No | First-party provider that owns a global model. Used by `isFirstParty` providers. |
| `name` / `label` | No | Display metadata. |
| `default` / `hidden` | No | Optional selection metadata. |
| `classification` | No | Any of `chat`, `reasoning`, `vision`, and `coding`. |
| `tier` | No | `first-party`, `hosted`, `local`, or `community`. |
| `contextWindow` | No | Input/context token limit for this model or route. |
| `maxOutputTokens` | No | Output token limit for this model or route. |
| `defaultTemperature` | No | Suggested temperature default. |
| `temperatureRange.min` / `temperatureRange.max` | No | Supported temperature bounds. |
| `capabilities.*` | No | Feature flags listed below. |
| `cost.inputPerMillion` / `cost.outputPerMillion` | Yes when `cost` is present | Pricing per one million input and output tokens. |
| `cost.cachePerMillion` / `cost.currency` | No | Optional cached-token price and currency code. |
| `transportOverrides.*` | No | Per-model transport overrides merged over provider `transportConfig`. |
| `notes` | No | Free-form maintainer note. |

When a gateway exposes a global model under a provider-specific name, limit, or
price, keep those route-specific values in that gateway's single
`[[catalog.models]]` block:

```toml
[[catalog.models]]
globalModelId = "claude-sonnet-4"
apiName = "anthropic/claude-sonnet-4"
contextWindow = 200000
maxOutputTokens = 64000
capabilities.supportsReasoning = true
cost.inputPerMillion = 3.00
cost.outputPerMillion = 15.00
cost.cachePerMillion = 0.30
cost.currency = "USD"
transportOverrides.maxTokensField = "max_tokens"
transportOverrides.removeBodyFields = ["store"]
```

`capabilities` flags are `supportsVision`, `supportsStreaming`,
`supportsFunctionCalling`, `supportsJsonMode`, `supportsReasoning`,
`supportsPreciseTokenCount`, `supportsEmbeddings`, and
`supportsTemperature`.

`cost` requires `inputPerMillion` and `outputPerMillion` when present.
`cachePerMillion` and `currency` are optional.

`transportOverrides` accepts the same fields as `transportConfig`. Header
overrides are merged with provider headers, and `removeBodyFields` is unioned
with provider-level removals.

This project was created using `bun init` in bun v1.3.11. [Bun](https://bun.com) is a fast all-in-one JavaScript runtime.

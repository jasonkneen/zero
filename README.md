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

Providers, gateways, and localhost endpoints are discovered from TOML descriptor
files in `src/providers/catalog/definitions`. Drop one file there and the TUI
`/provider` flow picks it up automatically — no source edits needed.

### Adding a new provider

| What you're adding | Files to touch |
|--------------------|----------------|
| Gateway (proxy/aggregator) | 1 file: `definitions/<name>.toml` |
| Localhost (Ollama, LM Studio) | 1 file: `definitions/<name>.toml` |
| First-tier provider (new API) | 1-2 files: `definitions/<name>.toml` + `models/<brand>.toml` if defining new models |

Gateways and localhost endpoints reference existing global models via
`globalModelId` — no separate model file needed. The models directory is
reserved for first-tier providers defining models they own.

### Provider kinds

| Kind | Purpose | Example |
|------|---------|---------|
| `provider` | First-party or direct API provider | OpenAI, Anthropic, DeepSeek |
| `gateway` | Proxy, aggregator, or relay | OpenGateway, OpenRouter, Groq |
| `localhost` | Local inference endpoint | Ollama, LM Studio |

### Transport kinds

| Kind | Purpose |
|------|---------|
| `openai-compatible` | OpenAI chat completions API format (default) |
| `anthropic-compatible` | Anthropic messages API format (recognized in descriptors, runtime not implemented yet) |

### Definition file structure

```toml
# Description of the provider

id = "my-provider"
name = "My Provider"
kind = "gateway"
description = "What this provider does"
baseURL = "https://api.example.com/v1"
defaultModel = "model-name"

[transportConfig]
kind = "openai-compatible"
maxTokensField = "max_completion_tokens"
removeBodyFields = ["store"]

# Custom headers - $ prefix reads from env var
[transportConfig.headers]
X-Custom = "$MY_ENV_VAR"

# How to send the API key
[transportConfig.authHeader]
name = "X-Api-Key"
scheme = "raw"

[catalog]
source = "hybrid"

[catalog.discovery]
kind = "openai-compatible"
requiresAuth = true

# Static models - shown first, before discovered models
[[catalog.models]]
id = "gpt-4o-limited"
apiName = "gpt-4o"
globalModelId = "gpt-4o"    # inherit from global catalog
contextWindow = 32000       # override limit
maxOutputTokens = 4096      # override limit
```

### Key fields

| Field | Purpose |
|-------|---------|
| `kind` | `provider`, `gateway`, or `localhost` |
| `isFirstParty` | Auto-exposes all global models from this provider |
| `vendorId` | Link to another provider to inherit its transport config when `transportConfig` is omitted |
| `ownerProviderId` | On global model entries: first-party provider that owns the model |
| `category` | `local`, `hosted`, or `aggregating` |
| `transportConfig.kind` | `openai-compatible` or `anthropic-compatible` |
| `transportConfig.headers` | Custom headers (supports `$ENV_VAR` interpolation) |
| `transportConfig.authHeader` | How to send the API key (name + scheme) |
| `transportConfig.maxTokensField` | `max_tokens` or `max_completion_tokens` |
| `transportConfig.removeBodyFields` | Fields to strip from requests |
| `catalog.source` | `static`, `dynamic`, or `hybrid` |
| `catalog.discovery` | Config for dynamic model fetching |
| `catalog.discovery.kind` | `openai-compatible` or `ollama` currently execute discovery |
| `catalog.models` | Static model list (used in static/hybrid mode) |
| `globalModelId` | On a catalog model entry: reference a global model and override its limits |

### Custom headers

Header values can reference environment variables with `$` prefix:

```toml
[transportConfig.headers]
X-Tenant = "hardcoded-value"
X-Secret = "$MY_SECRET_ENV_VAR"
```

Values starting with `$` are resolved from `process.env` at runtime.
If the env var is not set, the header is silently omitted.

### Auth header config

Control how the API key is sent:

```toml
[transportConfig.authHeader]
name = "X-Api-Key"
scheme = "raw"    # send key directly, no "Bearer" prefix
# scheme = "bearer"  # sends "Authorization: Bearer ***" (default)
```

When no custom `authHeader` is provided, authenticated model discovery sends
`Authorization: Bearer <api key>`. Runtime OpenAI-compatible requests rely on
the OpenAI SDK's default `Authorization` header unless a custom raw auth header
is configured.

### Global model files

Global model metadata lives in `src/providers/catalog/models/`. These files
are reserved for first-tier providers defining models they own (e.g., OpenAI
defining GPT models, Anthropic defining Claude models). Gateways should NOT
add model files — use `globalModelId` references instead.

Model files use TOML's `[[model]]` array-of-tables syntax:

```toml
[[model]]
id = "gpt-4o"
name = "GPT-4o"
ownerProviderId = "openai"
tier = "first-party"
description = "OpenAI flagship multimodal model"
classification = ["chat", "vision", "coding"]
contextWindow = 128000
maxOutputTokens = 16384

[model.capabilities]
supportsVision = true
supportsStreaming = true
supportsFunctionCalling = true
```

This project was created using `bun init` in bun v1.3.11. [Bun](https://bun.com) is a fast all-in-one JavaScript runtime.

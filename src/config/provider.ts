import { configManager } from './manager';
import { getProviderDefinition } from '../providers/catalog';

export interface ProviderConfig {
  providerId?: string;
  apiKey?: string;
  baseURL: string;
  model: string;
}

export function resolveProviderCommandConfig(parsed: any): ProviderConfig {
  if (!parsed || typeof parsed !== 'object') {
    throw new Error('Provider command must return a JSON object');
  }

  const definition = parsed.provider_id ? getProviderDefinition(parsed.provider_id) : undefined;

  return {
    providerId: parsed.provider_id,
    apiKey: parsed.api_key,
    baseURL: parsed.base_url || definition?.baseURL || 'https://api.openai.com/v1',
    model: parsed.model || definition?.defaultModel || 'gpt-4o',
  };
}

export function parseProviderCommand(command: string): string[] {
  const parts: string[] = [];
  let current = '';
  let quote: '"' | "'" | null = null;

  for (let i = 0; i < command.length; i++) {
    const char = command[i];

    if (quote) {
      if (char === quote) {
        quote = null;
        continue;
      }

      if (char === '\\' && quote === '"') {
        const next = command[i + 1];
        if (next === '"' || next === '\\') {
          current += next;
          i++;
          continue;
        }
      }

      current += char;
      continue;
    }

    if (char === '"' || char === "'") {
      quote = char;
      continue;
    }

    if (/\s/.test(char)) {
      if (current) {
        parts.push(current);
        current = '';
      }
      continue;
    }

    current += char;
  }

  if (quote) {
    throw new Error(`Unterminated ${quote} quote in provider command`);
  }

  if (current) {
    parts.push(current);
  }

  if (parts.length === 0) {
    throw new Error('Provider command is empty');
  }

  return parts;
}

/**
 * Loads the effective provider configuration.
 * Priority order:
 *   1. ZERO_PROVIDER_COMMAND (external command) - highest
 *   2. Active profile from config (set via /provider)
 *   3. OPENAI_* environment variables
 */
export async function loadProviderConfig(): Promise<ProviderConfig> {
  // 1. Highest priority: external provider command
  const providerCommand = process.env.ZERO_PROVIDER_COMMAND;
  if (providerCommand) {
    try {
      const parts = parseProviderCommand(providerCommand);
      const program = parts[0];
      const args = parts.slice(1);

      const proc = Bun.spawn([program, ...args], {
        stdout: 'pipe',
        stderr: 'pipe',
      });

      const stdout = await new Response(proc.stdout).text();
      const exitCode = await proc.exited;

      if (exitCode !== 0) {
        const stderr = await new Response(proc.stderr).text();
        throw new Error(`Command exited with code ${exitCode}: ${stderr}`);
      }

      const parsed = JSON.parse(stdout);
      return resolveProviderCommandConfig(parsed);
    } catch (err: any) {
      throw new Error(
        `Failed to run ZERO_PROVIDER_COMMAND "${providerCommand}": ${err.message}\n` +
        'Expected JSON output with provider configuration.'
      );
    }
  }

  // 2. Active profile from saved config
  const fromProfile = configManager.getEffectiveProviderConfig();
  if (fromProfile) {
    return fromProfile;
  }

  // 3. Fallback to raw environment variables
  const envApiKey = process.env.OPENAI_API_KEY;
  const envBaseURL = process.env.OPENAI_BASE_URL || 'https://api.openai.com/v1';
  const envModel = process.env.OPENAI_MODEL || 'gpt-4o';

  // If we have no API key, give a helpful error
  if (!envApiKey) {
    throw new Error(
      'No LLM provider configured.\n\n' +
      'Please run /provider to add one, or set OPENAI_API_KEY environment variable.'
    );
  }

  return {
    apiKey: envApiKey,
    baseURL: envBaseURL,
    model: envModel,
  };
}

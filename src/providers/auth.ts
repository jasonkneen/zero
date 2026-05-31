import type { AuthHeaderConfig } from '../providers/catalog/types';

/**
 * Build auth headers from API key and auth header config.
 * Returns null values to suppress default headers.
 */
export function buildAuthHeaders(
  apiKey: string,
  authHeader?: AuthHeaderConfig
): Record<string, string | null> {
  if (!authHeader) return {};
  if (!apiKey) {
    console.warn('[providers] Auth header configured but no API key provided');
    return {};
  }

  const headerName = authHeader.name;
  const lowerName = headerName.toLowerCase();
  const headerValue = authHeader.scheme === 'raw' ? apiKey : `Bearer ${apiKey}`;

  if (lowerName === 'authorization' && authHeader.scheme !== 'raw') {
    return {};
  }

  return {
    [headerName]: headerValue,
    ...(lowerName === 'authorization' ? {} : { Authorization: null }),
  };
}

/**
 * Resolve header values, interpolating env vars.
 * Values starting with $ are resolved from process.env.
 * Null values are preserved (used to suppress default headers).
 */
export function resolveHeaders(headers: Record<string, string | null>): Record<string, string | null> {
  const resolved: Record<string, string | null> = {};
  for (const [name, value] of Object.entries(headers)) {
    if (value == null) {
      resolved[name] = null;
      continue;
    }
    if (value.startsWith('$')) {
      const envVar = value.slice(1);
      const envValue = process.env[envVar];
      if (envValue) {
        resolved[name] = envValue;
      } else {
        console.warn(`[providers] Header "${name}" references unset env var $${envVar}`);
      }
    } else {
      resolved[name] = value;
    }
  }
  return resolved;
}

export interface ProviderProfile {
  name: string;
  providerId?: string;
  kind?: 'provider' | 'gateway' | 'localhost' | 'custom';
  baseURL: string;
  apiKey?: string;
  model: string;
  description?: string;
}

export interface ZeroConfig {
  activeProvider?: string;
  providers: ProviderProfile[];
}

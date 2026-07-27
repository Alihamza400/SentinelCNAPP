import { useAuth } from '@/components/auth-provider';

interface Asset {
  id: string;
  provider: string;
  asset_type: string;
  name: string;
  region: string;
  tags: Record<string, string>;
  internet_facing: boolean;
  environment: string;
  active: boolean;
  discovered_at: string;
  last_synced_at: string;
}

interface ListAssetsResponse {
  assets: Asset[];
  total: number;
  page: number;
  page_size: number;
}

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || 'http://localhost:8080';

export class APIError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'APIError';
  }
}

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const token = typeof window !== 'undefined' ? localStorage.getItem('sentinel_token') : null;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { ...headers, ...options?.headers },
  });

  if (!res.ok) {
    throw new APIError(res.status, `API error: ${res.statusText}`);
  }

  return res.json();
}

export async function listAssets(params?: {
  provider?: string;
  type?: string;
  region?: string;
  environment?: string;
  search?: string;
  page?: number;
  page_size?: number;
}): Promise<ListAssetsResponse> {
  const searchParams = new URLSearchParams();
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== '') {
        searchParams.set(key, String(value));
      }
    });
  }
  const query = searchParams.toString();
  return fetchAPI<ListAssetsResponse>(`/api/v1/assets${query ? `?${query}` : ''}`);
}

export async function getAsset(id: string): Promise<Asset> {
  return fetchAPI<Asset>(`/api/v1/assets/${encodeURIComponent(id)}`);
}

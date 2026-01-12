export interface Item {
  id: string;
  title: string;
  link?: string;
  content: string;
  createdAt: string;
  updatedAt: string;
}

export interface SearchResult {
  item: Item;
  rank: number;
  snippet: string;
}

export interface CreateItemRequest {
  title: string;
  content: string;
  link?: string;
}

export interface UpdateItemRequest {
  title: string;
  content: string;
  link?: string;
}

// Auth types
export interface WhoAmI {
  authenticated: boolean;
  user?: {
    cn: string;
    auth_method: 'cert' | 'token' | 'none';
  };
  mode: 'single-user' | 'authenticated';
}

export interface TokenInfo {
  id: string;
  user_cn: string;
  name: string;
  created_at: string;
  expires_at: string;
  last_used_at?: string;
}

export interface CreateTokenRequest {
  name: string;
  expires_in?: string; // e.g., "720h"
}

export interface CreateTokenResponse {
  id: string;
  name: string;
  token: string; // The actual token - only shown once
  created_at: string;
  expires_at: string;
}

export interface StatusResponse {
  version: string;
  status: string;
}

class ApiClient {
  private baseUrl = '/api';

  async listItems(limit = 50, offset = 0): Promise<Item[]> {
    const res = await fetch(`${this.baseUrl}/items?limit=${limit}&offset=${offset}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getItem(id: string): Promise<Item> {
    const res = await fetch(`${this.baseUrl}/items/${id}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async createItem(req: CreateItemRequest): Promise<Item> {
    const res = await fetch(`${this.baseUrl}/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async updateItem(id: string, req: UpdateItemRequest): Promise<Item> {
    const res = await fetch(`${this.baseUrl}/items/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async deleteItem(id: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/items/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error(await res.text());
  }

  async search(query: string, limit = 20): Promise<SearchResult[]> {
    const res = await fetch(`${this.baseUrl}/search?q=${encodeURIComponent(query)}&limit=${limit}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async health(): Promise<{ status: string }> {
    const res = await fetch(`${this.baseUrl}/health`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async status(): Promise<StatusResponse> {
    const res = await fetch(`${this.baseUrl}/status`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  // Auth methods
  async whoami(): Promise<WhoAmI> {
    const res = await fetch(`${this.baseUrl}/whoami`);
    if (res.status === 401) {
      // Auth required but not provided
      throw new AuthRequiredError();
    }
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async listTokens(): Promise<TokenInfo[]> {
    const res = await fetch(`${this.baseUrl}/tokens`);
    if (res.status === 401) throw new AuthRequiredError();
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async createToken(req: CreateTokenRequest): Promise<CreateTokenResponse> {
    const res = await fetch(`${this.baseUrl}/tokens`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    });
    if (res.status === 401) throw new AuthRequiredError();
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async deleteToken(id: string): Promise<void> {
    const res = await fetch(`${this.baseUrl}/tokens/${id}`, { method: 'DELETE' });
    if (res.status === 401) throw new AuthRequiredError();
    if (!res.ok) throw new Error(await res.text());
  }
}

export class AuthRequiredError extends Error {
  constructor() {
    super('Authentication required');
    this.name = 'AuthRequiredError';
  }
}

export const api = new ApiClient();

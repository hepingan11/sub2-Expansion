const API_BASE = import.meta.env.VITE_API_BASE ?? '';
const TOKEN_KEY = 'redeem_admin_token';

export type RedeemCodeStatus = 'AVAILABLE' | 'ASSIGNED' | 'USED' | 'VOIDED';

export interface RedeemCode {
  id: number;
  code: string;
  userId: string | null;
  signDate: string | null;
  amount: number;
  status: RedeemCodeStatus;
  createdAt: string;
  updatedAt: string;
}

export interface PageResult<T> {
  content: T[];
  totalElements: number;
  totalPages: number;
  number: number;
  size: number;
}

export interface Stats {
  total: number;
  available: number;
  assigned: number;
  used: number;
  voided: number;
  amountStats: AmountStats[];
}

export interface AmountStats {
  amount: number;
  total: number;
  available: number;
}

export interface CodePayload {
  code?: string;
  userId?: string;
  signDate?: string;
  amount: string;
  status: RedeemCodeStatus;
}

export interface BatchImportPayload {
  codesText: string;
  amount: string;
}

export interface BatchImportResult {
  totalParsed: number;
  imported: number;
  duplicated: number;
  duplicatedCodes: string[];
}

export interface CheckInSettings {
  dailyMaxUsers: number;
  prizeTiers: PrizeTierSetting[];
  sub2api: Sub2APISettings;
}

export interface PrizeTierSetting {
  amount: number;
  probability: number;
}

export interface Sub2APISettings {
  baseUrl: string;
  authMode: 'admin_api_key' | 'jwt' | 'password';
  adminApiKey?: string;
  adminApiKeySet: boolean;
  jwt?: string;
  jwtSet: boolean;
  adminEmail: string;
  adminPassword?: string;
  adminPasswordSet: boolean;
  timeoutSeconds: number;
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

export async function login(username: string, password: string) {
  const data = await request<{ token: string; expiresInHours: number }>('/api/admin/login', {
    method: 'POST',
    body: JSON.stringify({ username, password })
  }, false);
  setToken(data.token);
  return data;
}

export async function fetchStats() {
  return request<Stats>('/api/admin/stats');
}

export async function fetchCheckInSettings() {
  return request<CheckInSettings>('/api/admin/settings/check-in');
}

export async function updateCheckInSettings(dailyMaxUsers: number, prizeTiers: PrizeTierSetting[], sub2api: Sub2APISettings) {
  return request<CheckInSettings>('/api/admin/settings/check-in', {
    method: 'PUT',
    body: JSON.stringify({ dailyMaxUsers, prizeTiers, sub2api })
  });
}

export async function fetchCodes(params: Record<string, string | number | undefined>) {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<PageResult<RedeemCode>>(`/api/admin/codes?${query.toString()}`);
}

export async function createCode(payload: CodePayload) {
  return request<RedeemCode>('/api/admin/codes', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function batchImportCodes(payload: BatchImportPayload) {
  return request<BatchImportResult>('/api/admin/codes/batch-import', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function updateCode(id: number, payload: CodePayload) {
  return request<RedeemCode>(`/api/admin/codes/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function deleteCode(id: number) {
  return request<void>(`/api/admin/codes/${id}`, {
    method: 'DELETE'
  });
}

async function request<T>(path: string, init: RequestInit = {}, withAuth = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json;charset=UTF-8');

  const token = getToken();
  if (withAuth && token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers
  });

  if (response.status === 401) {
    clearToken();
    throw new Error('登录已过期，请重新登录');
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: '请求失败' }));
    throw new Error(error.message ?? '请求失败');
  }

  if (response.status === 204 || init.method === 'DELETE') {
    return undefined as T;
  }

  return response.json();
}

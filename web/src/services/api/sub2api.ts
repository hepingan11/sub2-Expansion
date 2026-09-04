import { SUB2API_URL } from "@/constant/runtime-config";
import type { ApiCallFormat } from "@/stores/use-config-store";

type ApiEnvelope<T> = { code?: number; data?: T; message?: string };
export type Sub2apiGroup = { id: number; name: string; platform: string; status: string };
type Sub2apiKey = { key: string; group_id: number | null; status: string };
type Paginated<T> = { items?: T[] };

export type Sub2apiLoginResult = {
    access_token?: string;
    requires_2fa?: boolean;
    user?: Record<string, unknown>;
};

const panelApiBase = "/auth-api/api/v1";
const gatewayApiBase = "/auth-api/v1";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(`${panelApiBase}${path}`, {
        ...init,
        headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
    });
    const payload = (await response.json().catch(() => ({}))) as ApiEnvelope<T>;
    if (!response.ok || (typeof payload.code === "number" && payload.code !== 0)) {
        throw new Error(payload.message || `请求失败（${response.status}）`);
    }
    return (payload.data ?? payload) as T;
}

function auth(token: string) {
    return { Authorization: `Bearer ${token}` };
}

export function sub2apiChannelBaseUrl() {
    if (!SUB2API_URL) throw new Error("管理员尚未配置 SUB2API_URL");
    return `${SUB2API_URL.replace(/\/api\/v1$/i, "")}/v1`;
}

export function loginSub2api(email: string, password: string) {
    return request<Sub2apiLoginResult>("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
}

export function logoutSub2api(token: string) {
    return request("/auth/logout", { method: "POST", headers: auth(token), body: JSON.stringify({}) });
}

export function getSub2apiUser(token: string) {
    return request<Record<string, unknown>>("/auth/me", { headers: auth(token) });
}

export async function fetchSub2apiModels(apiKey: string) {
    const response = await fetch(`${gatewayApiBase}/models`, { headers: auth(apiKey) });
    const payload = (await response.json().catch(() => ({}))) as { data?: Array<{ id?: string }>; error?: { message?: string }; message?: string };
    if (!response.ok) throw new Error(payload.error?.message || payload.message || `模型列表请求失败（${response.status}）`);
    return (payload.data || [])
        .map((model) => model.id?.trim())
        .filter((id): id is string => Boolean(id))
        .sort((a, b) => a.localeCompare(b));
}

export function fetchSub2apiGroups(token: string) {
    if (!token) throw new Error("登录状态已失效，请重新登录");
    return request<Sub2apiGroup[]>("/groups/available", { headers: auth(token) });
}

export async function resolveSub2apiChannel(group: Sub2apiGroup, token: string) {
    if (!token) throw new Error("登录状态已失效，请重新登录");
    if (group.status !== "active") throw new Error(`sub2api 分组“${group.name}”当前不可用`);
    const query = new URLSearchParams({ page: "1", page_size: "20", status: "active", group_id: String(group.id) });
    const keys = await request<Paginated<Sub2apiKey>>(`/keys?${query}`, { headers: auth(token) });
    const apiKey = (keys.items || []).find((item) => item.status === "active" && item.group_id === group.id)?.key || "";
    if (!apiKey) throw new Error(`请先在 sub2api 的“${group.name}”分组下创建 API Key`);
    return {
        apiKey,
        apiFormat: apiFormatForSub2apiPlatform(group.platform),
        baseUrl: sub2apiChannelBaseUrl(),
        groupId: group.id,
        groupName: group.name,
        models: await fetchSub2apiModels(apiKey),
    };
}

export function apiFormatForSub2apiPlatform(platform: string): ApiCallFormat {
    const value = platform.trim().toLowerCase();
    if (value === "gemini" || value === "antigravity") return "gemini";
    if (value === "grok") return "grok";
    if (value === "anthropic" || value === "claude") return "anthropic";
    return "openai";
}

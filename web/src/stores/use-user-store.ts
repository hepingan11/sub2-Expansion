import { create } from "zustand";
import { persist } from "zustand/middleware";
import { SUB2API_URL } from "@/constant/runtime-config";
import { defaultConfig, useConfigStore } from "@/stores/use-config-store";

export type LocalUser = {
    id: string;
    username: string;
    displayName: string;
    avatarUrl: string;
    email?: string;
};

type UserStore = {
    user: LocalUser | null;
    accessToken: string;
    hydrated: boolean;
    login: (email: string, password: string) => Promise<void>;
    logout: () => Promise<void>;
    clearSession: () => void;
};

const apiBase = SUB2API_URL.endsWith("/api/v1") ? SUB2API_URL : `${SUB2API_URL}/api/v1`;

function normalizeUser(raw: Record<string, unknown>): LocalUser {
    const id = String(raw.id ?? raw.user_id ?? raw.email ?? "");
    return { id, username: String(raw.username ?? raw.email ?? id), displayName: String(raw.display_name ?? raw.username ?? raw.email ?? id), avatarUrl: String(raw.avatar_url ?? ""), email: typeof raw.email === "string" ? raw.email : undefined };
}

async function sub2apiRequest<T>(path: string, init?: RequestInit): Promise<T> {
    if (!SUB2API_URL) throw new Error("未配置 SUB2API_URL");
    const response = await fetch(`${apiBase}${path}`, { ...init, headers: { "Content-Type": "application/json", ...(init?.headers || {}) } });
    const payload = (await response.json()) as { data?: T; message?: string; code?: number };
    if (!response.ok || (typeof payload.code === "number" && payload.code !== 0)) throw new Error(payload.message || `请求失败（${response.status}）`);
    return (payload.data ?? payload) as T;
}

export const useUserStore = create<UserStore>()(
    persist(
        (set, get) => ({
            user: null,
            accessToken: "",
            hydrated: false,
            login: async (email, password) => {
                const result = await sub2apiRequest<{ access_token?: string; requires_2fa?: boolean; user?: Record<string, unknown> }>("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
                if (result.requires_2fa || !result.access_token || !result.user) throw new Error("当前账号启用了双因素认证，请先在 sub2api 完成登录后再使用画布");
                const user = normalizeUser(result.user);
                localStorage.setItem("infinite-canvas:auth", JSON.stringify(user));
                set({ user, accessToken: result.access_token });
                const [{ useCanvasStore }, { useAssetStore }] = await Promise.all([import("@/stores/canvas/use-canvas-store"), import("@/stores/use-asset-store")]);
                useCanvasStore.setState({ projects: [], deletedProjects: [] });
                useAssetStore.setState({ assets: [] });
                useConfigStore.setState({ config: defaultConfig });
                await Promise.all([useCanvasStore.persist.rehydrate(), useAssetStore.persist.rehydrate(), useConfigStore.persist.rehydrate()]);
                let hasActiveKey = false;
                try {
                    const keys = await sub2apiRequest<{ items?: Array<{ key: string; status: string }> }>("/keys?page=1&page_size=20", { headers: { Authorization: `Bearer ${result.access_token}` } });
                    const key = keys.items?.find((item) => item.status === "active")?.key;
                    hasActiveKey = Boolean(key);
                    if (key) {
                        const config = useConfigStore.getState().config;
                        const baseUrl = `${SUB2API_URL}/v1`;
                        const channels = config.channels.map((channel, index) => (index === 0 ? { ...channel, baseUrl, apiKey: key } : channel));
                        useConfigStore.getState().updateConfig("channels", channels);
                        useConfigStore.getState().updateConfig("baseUrl", baseUrl);
                        useConfigStore.getState().updateConfig("apiKey", key);
                    }
                } catch {
                    // Login remains valid when API key listing is unavailable.
                }
                if (!hasActiveKey) {
                    const config = useConfigStore.getState().config;
                    useConfigStore.getState().updateConfig("channels", config.channels.map((channel, index) => (index === 0 ? { ...channel, baseUrl: `${SUB2API_URL}/v1` } : channel)));
                    useConfigStore.getState().updateConfig("baseUrl", `${SUB2API_URL}/v1`);
                }
            },
            logout: async () => {
                const token = get().accessToken;
                if (token) void sub2apiRequest("/auth/logout", { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: JSON.stringify({}) }).catch(() => undefined);
                localStorage.removeItem("infinite-canvas:auth");
                set({ user: null, accessToken: "" });
            },
            clearSession: () => {
                localStorage.removeItem("infinite-canvas:auth");
                set({ user: null, accessToken: "" });
            },
        }),
        { name: "infinite-canvas:user-session", partialize: (state) => ({ user: state.user, accessToken: state.accessToken }), onRehydrateStorage: () => () => useUserStore.setState({ hydrated: true }) },
    ),
);

export async function sub2apiFetch<T>(path: string, token: string) {
    return sub2apiRequest<T>(path, { headers: { Authorization: `Bearer ${token}` } });
}

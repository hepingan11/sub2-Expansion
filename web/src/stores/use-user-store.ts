import { create } from "zustand";
import { persist } from "zustand/middleware";

import { apiFormatForSub2apiPlatform, fetchSub2apiGroups, getSub2apiUser, loginSub2api, logoutSub2api, resolveSub2apiChannel, sub2apiChannelBaseUrl } from "@/services/api/sub2api";
import { defaultConfig, guessCapability, useConfigStore } from "@/stores/use-config-store";

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
    sessionChecked: boolean;
    sessionChecking: boolean;
    login: (email: string, password: string) => Promise<void>;
    logout: () => Promise<void>;
    validateSession: () => Promise<void>;
    clearSession: () => void;
};

function normalizeUser(raw: Record<string, unknown>): LocalUser {
    const id = String(raw.id ?? raw.user_id ?? raw.email ?? "");
    return {
        id,
        username: String(raw.username ?? raw.email ?? id),
        displayName: String(raw.display_name ?? raw.username ?? raw.email ?? id),
        avatarUrl: String(raw.avatar_url ?? ""),
        email: typeof raw.email === "string" ? raw.email : undefined,
    };
}

export const useUserStore = create<UserStore>()(
    persist(
        (set, get) => ({
            user: null,
            accessToken: "",
            sessionChecked: false,
            sessionChecking: false,
            login: async (email, password) => {
                const result = await loginSub2api(email, password);
                if (result.requires_2fa || !result.access_token || !result.user) {
                    throw new Error("当前账号启用了双因素认证，请先在 sub2api 完成登录后再使用画布");
                }
                const user = normalizeUser(result.user);
                localStorage.setItem("infinite-canvas:auth", JSON.stringify(user));
                set({ user, accessToken: result.access_token, sessionChecked: true, sessionChecking: false });

                const [{ useCanvasStore }, { useAssetStore }] = await Promise.all([
                    import("@/stores/canvas/use-canvas-store"),
                    import("@/stores/use-asset-store"),
                ]);
                useCanvasStore.setState({ projects: [], deletedProjects: [] });
                useAssetStore.setState({ assets: [] });
                useConfigStore.setState({ config: defaultConfig });
                await Promise.all([useCanvasStore.persist.rehydrate(), useAssetStore.persist.rehydrate(), useConfigStore.persist.rehydrate()]);

                const baseUrl = sub2apiChannelBaseUrl();
                const config = useConfigStore.getState().config;
                const groups = (await fetchSub2apiGroups(result.access_token)).filter((group) => group.status === "active");
                const groupsByChannel = config.channels.map((channel) => groups.find((group) => group.id === channel.groupId) || groups.find((group) => apiFormatForSub2apiPlatform(group.platform) === channel.apiFormat) || groups[0]);
                const selectedGroups = Array.from(new Map(groupsByChannel.flatMap((group) => (group ? [[group.id, group] as const] : []))).values());
                const results = await Promise.allSettled(selectedGroups.map(async (group) => [group.id, await resolveSub2apiChannel(group, result.access_token as string)] as const));
                const resolvedByGroup = new Map(results.flatMap((entry) => (entry.status === "fulfilled" ? [entry.value] : [])));
                const channels = config.channels.map((item, index) => {
                    const group = groupsByChannel[index];
                    const resolved = group ? resolvedByGroup.get(group.id) : undefined;
                    if (!resolved) return { ...item, baseUrl, apiKey: "" };
                    const existingModels = new Map(item.models.map((model) => [model.name, model]));
                    return {
                        ...item,
                        ...resolved,
                        models: resolved.models.map((name) => {
                            const existing = existingModels.get(name);
                            return { ...existing, name, capability: resolved.apiFormat === "anthropic" ? "text" : existing?.capability || guessCapability(name) };
                        }),
                    };
                });
                const primary = channels[0];
                useConfigStore.getState().updateConfig("channels", channels);
                useConfigStore.getState().updateConfig("baseUrl", primary?.baseUrl || baseUrl);
                useConfigStore.getState().updateConfig("apiKey", primary?.apiKey || "");
                useConfigStore.getState().updateConfig("apiFormat", primary?.apiFormat || "openai");
            },
            logout: async () => {
                const token = get().accessToken;
                if (token) {
                    void logoutSub2api(token).catch(() => undefined);
                }
                localStorage.removeItem("infinite-canvas:auth");
                set({ user: null, accessToken: "", sessionChecked: true, sessionChecking: false });
            },
            validateSession: async () => {
                const { accessToken, sessionChecked, sessionChecking, user } = get();
                if (sessionChecked || sessionChecking) return;
                if (!user || !accessToken) {
                    set({ user: null, accessToken: "", sessionChecked: true, sessionChecking: false });
                    return;
                }
                set({ sessionChecking: true });
                try {
                    const currentUser = await getSub2apiUser(accessToken);
                    const normalized = normalizeUser(currentUser);
                    localStorage.setItem("infinite-canvas:auth", JSON.stringify(normalized));
                    set({ user: normalized, sessionChecked: true, sessionChecking: false });
                } catch {
                    localStorage.removeItem("infinite-canvas:auth");
                    set({ user: null, accessToken: "", sessionChecked: true, sessionChecking: false });
                }
            },
            clearSession: () => {
                localStorage.removeItem("infinite-canvas:auth");
                set({ user: null, accessToken: "", sessionChecked: true, sessionChecking: false });
            },
        }),
        {
            name: "infinite-canvas:user-session",
            partialize: (state) => ({ user: state.user, accessToken: state.accessToken }),
        },
    ),
);

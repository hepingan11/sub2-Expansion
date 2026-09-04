import localforage from "localforage";
import type { StateStorage } from "zustand/middleware";

localforage.config({
    name: "infinite-canvas",
    storeName: "app_state",
});

export function activeStorageScope() {
    if (typeof window === "undefined") return "anonymous";
    try {
        const raw = window.localStorage.getItem("infinite-canvas:auth");
        const user = raw ? (JSON.parse(raw) as { id?: string }) : null;
        return user?.id ? `user-${user.id}` : "anonymous";
    } catch {
        return "anonymous";
    }
}

function scopedName(name: string) {
    return `${name}:${activeStorageScope()}`;
}

export const localForageStorage: StateStorage = {
    getItem: async (name) => {
        if (typeof window === "undefined") return null;
        try {
            return (await localforage.getItem<string>(scopedName(name))) || null;
        } catch {
            return window.localStorage.getItem(scopedName(name));
        }
    },
    setItem: async (name, value) => {
        if (typeof window === "undefined") return;
        try {
            await localforage.setItem(scopedName(name), value);
        } catch {
            window.localStorage.setItem(scopedName(name), value);
        }
    },
    removeItem: async (name) => {
        if (typeof window === "undefined") return;
        try {
            await localforage.removeItem(scopedName(name));
        } catch {
            window.localStorage.removeItem(scopedName(name));
        }
    },
};

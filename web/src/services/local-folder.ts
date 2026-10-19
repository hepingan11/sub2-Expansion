import localforage from "localforage";
import { activeStorageScope } from "@/lib/localforage-storage";

const store = localforage.createInstance({ name: "infinite-canvas", storeName: "local_folders" });
const key = () => `directory:${activeStorageScope()}`;

export async function pickLocalFolder() {
    if (!window.isSecureContext) throw new Error("本地文件夹连接需要使用 localhost 或 HTTPS 打开页面");
    if (!("showDirectoryPicker" in window)) throw new Error("当前浏览器不支持本地文件夹连接");
    const handle = await (window as Window & { showDirectoryPicker: () => Promise<FileSystemDirectoryHandle> }).showDirectoryPicker();
    await store.setItem(key(), handle);
    return handle;
}

export function isLocalFolderPickerCancelled(error: unknown) {
    return error instanceof DOMException && error.name === "AbortError";
}

export async function getLocalFolder() {
    return store.getItem<FileSystemDirectoryHandle>(key());
}

export async function verifyLocalFolder(handle: FileSystemDirectoryHandle | null) {
    if (!handle) return false;
    try {
        const permission = await handle.queryPermission({ mode: "readwrite" });
        return permission === "granted" || (permission === "prompt" && (await handle.requestPermission({ mode: "readwrite" })) === "granted");
    } catch {
        return false;
    }
}

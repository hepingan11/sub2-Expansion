import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { App, Button } from "antd";
import { FolderOpen, LogOut } from "lucide-react";

import { AgentPanel } from "@/components/agent/agent-panel";
import { AppTopNav } from "@/components/layout/app-top-nav";
import { getLocalFolder, pickLocalFolder, verifyLocalFolder } from "@/services/local-folder";
import { useUserStore } from "@/stores/use-user-store";

export default function UserLayout({ children }: { children: ReactNode }) {
    const { message } = App.useApp();
    const location = useLocation();
    const navigate = useNavigate();
    const user = useUserStore((state) => state.user);
    const hydrated = useUserStore((state) => state.hydrated);
    const logout = useUserStore((state) => state.logout);
    const [folderReady, setFolderReady] = useState(false);
    useEffect(() => { if (!user) return setFolderReady(false); void getLocalFolder().then((handle) => verifyLocalFolder(handle).then(setFolderReady)); }, [user]);
    if (!hydrated) return <div className="flex h-dvh items-center justify-center bg-background text-sm text-stone-500">正在恢复登录状态…</div>;
    if (!user) return <Navigate to={`/login?next=${encodeURIComponent(location.pathname)}`} replace />;
    if (!folderReady) return <main className="flex h-dvh items-center justify-center bg-background px-5"><div className="w-full max-w-md border border-stone-200 bg-background p-8 text-center shadow-sm dark:border-stone-800"><FolderOpen className="mx-auto mb-4 size-9 text-[#a87928]" /><h1 className="text-xl font-semibold">连接本地工作区</h1><p className="mt-3 text-sm leading-6 text-stone-500">登录后必须连接自己的电脑文件夹，画布和素材会按账号隔离保存。</p><div className="mt-6 flex justify-center gap-2"><Button type="primary" onClick={() => void pickLocalFolder().then(() => setFolderReady(true)).catch((error) => message.error(error instanceof Error ? error.message : "连接失败"))}>选择文件夹</Button><Button icon={<LogOut className="size-4" />} onClick={() => void logout().then(() => navigate("/login", { replace: true }))}>退出登录</Button></div></div></main>;
    return (
        <div className="flex h-dvh overflow-hidden bg-background text-foreground">
            <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
                <AppTopNav />
                <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
            </div>
            <AgentPanel />
        </div>
    );
}

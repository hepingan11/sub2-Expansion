import { App, Button, Form, Input } from "antd";
import { ArrowRight, FolderOpen, LogIn } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { SUB2API_URL } from "@/constant/runtime-config";
import { pickLocalFolder } from "@/services/local-folder";
import { useUserStore } from "@/stores/use-user-store";

type LoginValues = { email: string; password: string };

export default function LoginPage() {
    const { message } = App.useApp();
    const navigate = useNavigate();
    const login = useUserStore((state) => state.login);
    const [loading, setLoading] = useState(false);
    const submit = async ({ email, password }: LoginValues) => {
        setLoading(true);
        try {
            await login(email, password);
            try { await pickLocalFolder(); } catch { /* UserLayout will offer the picker again. */ }
            message.success("登录成功");
            navigate("/canvas", { replace: true });
        } catch (error) {
            message.error(error instanceof Error ? error.message : "登录失败");
        } finally {
            setLoading(false);
        }
    };
    return (
        <main className="flex min-h-dvh items-center justify-center bg-[#f4f1ea] px-5 py-10 text-stone-950 dark:bg-[#171512] dark:text-stone-100">
            <div className="grid w-full max-w-5xl overflow-hidden border border-stone-200 bg-[#fbfaf7] shadow-[0_24px_80px_rgba(55,48,40,.12)] dark:border-stone-800 dark:bg-[#211f1b] md:grid-cols-[1.05fr_.95fr]">
                <div className="relative hidden min-h-[580px] overflow-hidden bg-[#25241f] p-12 text-[#f3efe7] md:block">
                    <div className="absolute inset-0 opacity-40" style={{ backgroundImage: "linear-gradient(120deg, transparent 0 49%, rgba(244,211,119,.55) 50%, transparent 51%), linear-gradient(30deg, transparent 0 49%, rgba(127,191,186,.35) 50%, transparent 51%)", backgroundSize: "92px 92px" }} />
                    <div className="relative flex h-full flex-col justify-between">
                        <div><div className="mb-8 size-10 bg-current" style={{ mask: "url(/logo.svg) center / contain no-repeat", WebkitMask: "url(/logo.svg) center / contain no-repeat" }} /><p className="text-xs uppercase tracking-[.28em] text-[#d7b65c]">INFINITE CANVAS</p><h1 className="mt-8 max-w-sm text-5xl font-semibold leading-[1.05]">把灵感，留在你的工作区。</h1></div>
                        <p className="max-w-sm text-sm leading-7 text-[#bcb6aa]">每个账号拥有独立的画布、素材和配置。数据保存在你选择的本地文件夹中。</p>
                    </div>
                </div>
                <div className="flex min-h-[580px] flex-col justify-center p-8 sm:p-14">
                    <div className="mb-10"><p className="text-sm font-medium text-[#a87928]">欢迎回来</p><h2 className="mt-2 text-3xl font-semibold">登录画布</h2><p className="mt-3 text-sm leading-6 text-stone-500">使用 sub2api 账号继续。首次登录需要选择本地工作区。</p></div>
                    <Form<LoginValues> layout="vertical" requiredMark={false} onFinish={submit} autoComplete="on">
                        <Form.Item name="email" label="邮箱" rules={[{ required: true, type: "email", message: "请输入有效邮箱" }]}><Input size="large" prefix={<LogIn className="size-4 text-stone-400" />} autoComplete="username" /></Form.Item>
                        <Form.Item name="password" label="密码" rules={[{ required: true, message: "请输入密码" }]}><Input.Password size="large" autoComplete="current-password" /></Form.Item>
                        <Button htmlType="submit" type="primary" size="large" block loading={loading} icon={<ArrowRight className="size-4" />} iconPosition="end">登录并连接文件夹</Button>
                    </Form>
                    <div className="mt-8 border-t border-stone-200 pt-5 text-xs leading-5 text-stone-500 dark:border-stone-700"><FolderOpen className="mr-2 inline size-4" />文件夹权限只授予当前浏览器，不会上传文件内容。{SUB2API_URL ? ` 当前服务：${new URL(SUB2API_URL).hostname}` : "管理员尚未配置 sub2api 服务地址。"}</div>
                </div>
            </div>
        </main>
    );
}

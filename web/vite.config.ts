import { readdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv, type Plugin } from "vite";

import { parseChangelog } from "./src/lib/release";

const webDir = dirname(fileURLToPath(import.meta.url));
const localVersion = readFileSync(resolve(webDir, "../VERSION"), "utf8").trim() || "dev";
const localChangelog = readFileSync(resolve(webDir, "../CHANGELOG.md"), "utf8");

// Expose /plugins/index.json with local plugin files from public/plugins.
// The frontend can discover and list them when enabled; development reads the directory live, while builds emit a static registry.
function localPluginsManifest(): Plugin {
    const pluginsDir = resolve(webDir, "public/plugins");
    const listLocalPlugins = () => {
        try {
            return readdirSync(pluginsDir)
                .filter((file) => file.endsWith(".js"))
                .sort()
                .map((file) => `/plugins/${file}`);
        } catch {
            return [];
        }
    };
    return {
        name: "local-plugins-manifest",
        configureServer(server) {
            server.middlewares.use("/plugins/index.json", (_req, res) => {
                res.setHeader("Content-Type", "application/json");
                res.end(JSON.stringify(listLocalPlugins()));
            });
        },
        generateBundle() {
            this.emitFile({ type: "asset", fileName: "plugins/index.json", source: JSON.stringify(listLocalPlugins()) });
        },
    };
}

export default defineConfig(({ mode }) => {
    const projectDir = resolve(webDir, "..");
    const env = loadEnv(mode, projectDir, "");
    const sub2apiUrl = process.env.VITE_SUB2API_URL || env.VITE_SUB2API_URL || env.SUB2API_URL;
    return {
        envDir: projectDir,
        base: process.env.VITE_BASE || env.VITE_BASE || "/",
        plugins: [react(), localPluginsManifest()],
        resolve: {
            alias: {
                "@": resolve(webDir, "src"),
            },
        },
        define: {
            "import.meta.env.VITE_SUB2API_URL": JSON.stringify(sub2apiUrl || ""),
            __APP_VERSION__: JSON.stringify(localVersion),
            __APP_RELEASES__: JSON.stringify(parseChangelog(localChangelog)),
        },
        server: sub2apiUrl
            ? {
                  proxy: {
                      "/auth-api": {
                          target: sub2apiUrl,
                          changeOrigin: true,
                          rewrite: (path) => path.replace(/^\/auth-api/, ""),
                      },
                  },
              }
            : undefined,
    };
});

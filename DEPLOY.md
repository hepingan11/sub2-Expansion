# Docker Compose 部署

这个 compose 会一起启动：

- React 前端，使用 Nginx 提供静态文件和 `/api` 反向代理
- Go 后端
- PostgreSQL，数据保存在 Docker volume 中

## 一条命令启动

如果服务器已经安装好 Docker 和 Docker Compose v2，可以直接执行：

```bash
curl -fsSL https://raw.githubusercontent.com/hepingan11/sub2-Expansion/main/scripts/install.sh | sh
```

脚本会拉取 GitHub 仓库、首次生成 `.env`，并执行 `docker compose up -d --build`。首次生成的后台密码会打印在终端里。

如果当前目录没有写入权限，可以指定安装目录：

```bash
curl -fsSL https://raw.githubusercontent.com/hepingan11/sub2-Expansion/main/scripts/install.sh -o /tmp/sub2-install.sh
INSTALL_DIR="$HOME/sub2-Expansion" sh /tmp/sub2-install.sh
```

也可以手动进入已拉取的项目目录启动：

```bash
docker compose up -d --build
```

访问：

```text
http://your-server-ip:6779/
```

如果直接使用 compose 默认配置，后台账号为：

```text
admin / admin123
```

如果使用上面的一键安装脚本，后台密码会随机生成，请以脚本终端输出为准。

## 生产配置

生产环境建议先创建 `.env`，至少修改数据库密码、后台密码和 `AUTH_SECRET`：

```bash
cp deploy.env.example .env
docker compose up -d --build
```

生成随机 `AUTH_SECRET` 可以用：

```bash
openssl rand -hex 32
```

前端容器会把 `/api` 代理到后端容器，所以 `VITE_API_BASE` 通常保持为空即可。后端不会对外暴露端口，浏览器只访问前端容器的 `HTTP_PORT`。

## 版本更新

后台“系统设置”会通过 GitHub Releases 检测 `hepingan11/sub2-Expansion` 的 latest release。检测到新版本后：

- Docker 部署会自动使用 `scripts/update.sh`，无需手工填写 `SYSTEM_UPDATE_COMMAND`，后台可以直接点击“立即更新”。
- 更新命令会切换到 latest release tag，更新 `.env` 里的 `APP_VERSION`，再执行 `docker compose up -d --build`。
- 为了让后台容器执行 compose 更新，`backend` 服务会挂载项目目录和 `/var/run/docker.sock`。这等同于授予后台管理员主机 Docker 管理权限，只建议在可信管理员环境使用。

如果不想允许后台执行更新，将 `.env` 里的 `SYSTEM_UPDATE_ENABLED` 设为 `false`。之后可以在服务器手动更新：

```bash
git fetch --tags origin
git checkout <release-tag>
docker compose up -d --build
```

## 常用命令

```bash
docker compose ps
docker compose logs -f backend
docker compose logs -f frontend
docker compose logs -f postgres
docker compose pull
docker compose up -d --build
docker compose down
```

PostgreSQL 数据保存在 `postgres_data` volume。`docker compose down` 会保留数据；`docker compose down -v` 会删除数据。

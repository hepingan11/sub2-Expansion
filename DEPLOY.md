# Docker Compose 部署

这个 compose 会一起启动：

- React 前端，使用 Nginx 提供静态文件和 `/api` 反向代理
- Go 后端
- PostgreSQL，数据保存在 Docker volume 中

## 一条命令启动

```bash
docker compose up -d --build
```

访问：

```text
http://your-server-ip:6779/
```

默认后台账号：

```text
admin / admin123
```

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

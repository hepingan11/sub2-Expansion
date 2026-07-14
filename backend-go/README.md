# Go 后端

这是原 Java Spring Boot 后端的 Go + Gin + GORM 迁移版，接口路径保持一致：

- `POST /api/admin/login`
- `GET /api/admin/codes`
- `GET /api/admin/codes/:id`
- `POST /api/admin/codes`
- `POST /api/admin/codes/batch-import`
- `PUT /api/admin/codes/:id`
- `DELETE /api/admin/codes/:id`
- `GET /api/admin/stats`
- `GET /api/admin/settings/check-in`
- `PUT /api/admin/settings/check-in`
- `POST /api/checkins`
- `POST /api/checkins/social`（群机器人签到/邀请绑定入口）
- `GET /api/user/invitation`
- `POST /api/user/invitation/code`
- `GET /api/admin/invitations`（管理员分页查看邀请记录）

## 群机器人邀请绑定

机器人收到 `@咕咕嘎嘎 绑定邀请码` 后，请调用：

```http
POST /api/checkins/social
Content-Type: application/json

{
  "platform": "qq",
  "userId": "群成员平台用户ID",
  "inviteCode": "ABCDEFGH"
}
```

未绑定时接口返回 `404`、`code=SOCIAL_ACCOUNT_NOT_BOUND`，并在 `bindingUrl` 中返回带有
`platform`、`userid`、`invitecode` 的登录链接。机器人应直接把该链接发给新人。

新人登录后，扩展服务会绑定平台账号、校验 Sub2API 用户的 `created_at` 是否严格晚于管理员设置的时间门槛，随后用幂等请求给邀请人增加配置的余额。

管理员邀请记录接口支持 `page`、`size`、`keyword`、`status` 和 `platform` 查询参数。`keyword` 可搜索邀请码、邀请人/新人用户 ID、平台账号 ID；`status` 可选 `REWARDED`、`PENDING`、`FAILED`。

## 启动

```powershell
cd backend-go
go mod tidy
go run .
```

默认监听 `8625`。推荐复制 `.env.example` 为 `.env`，再按本机 PostgreSQL 信息修改：

```powershell
cd backend-go
Copy-Item .env.example .env
go run .
```

PostgreSQL 需要先创建数据库，例如 `createdb redeem_code_system`，或在管理工具里创建同名库。

`.env` 中可以配置：

```dotenv
SERVER_PORT=8625
DB_URL=postgres://postgres:postgres@localhost:5432/redeem_code_system?sslmode=disable&TimeZone=Asia/Shanghai
DB_USERNAME=postgres
DB_PASSWORD=postgres
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin123
AUTH_SECRET=please-change-this-secret
AUTH_TOKEN_TTL_HOURS=12
CORS_ALLOWED_ORIGINS=http://localhost:5173
CHECK_IN_DAILY_MAX_USERS=20
SUB2API_BASE_URL=https://your-sub2api-host
SUB2API_ADMIN_API_KEY=
SUB2API_ADMIN_EMAIL=admin@example.com
SUB2API_ADMIN_PASSWORD=
SUB2API_TIMEOUT_SECONDS=15
SUB2API_TOKEN_REFRESH_ENABLED=true
SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS=300
```

系统环境变量优先级高于 `.env`。如果需要临时覆盖，也可以直接设置环境变量：

```powershell
$env:SERVER_PORT="8625"
$env:DB_URL="postgres://postgres:postgres@localhost:5432/redeem_code_system?sslmode=disable&TimeZone=Asia/Shanghai"
$env:DB_USERNAME="postgres"
$env:DB_PASSWORD="postgres"
$env:ADMIN_USERNAME="admin"
$env:ADMIN_PASSWORD="admin123"
$env:AUTH_SECRET="please-change-this-secret"
$env:AUTH_TOKEN_TTL_HOURS="12"
$env:CORS_ALLOWED_ORIGINS="http://localhost:5173"
$env:CHECK_IN_DAILY_MAX_USERS="20"
go run .
```

也可以直接提供 PostgreSQL DSN：

```powershell
$env:DB_DSN="host=localhost user=postgres password=postgres dbname=redeem_code_system port=5432 sslmode=disable TimeZone=Asia/Shanghai"
go run .
```

## 说明

- 使用 PostgreSQL 存储数据，并在启动时执行表结构迁移。
- 管理员 token 格式兼容原 Java HMAC token，不是 JWT。
- PostgreSQL 连接池默认限制为 `MaxOpenConns=10`、`MaxIdleConns=5`，更适合小内存服务器。
- 兑换码金额和概率用 decimal 处理，JSON 仍返回数字格式。
- 签到成功时会按抽中的金额调用 Sub2API `POST /api/v1/admin/redeem-codes/generate` 生成 `balance` 兑换码，再把生成的码绑定到本地签到记录。`SUB2API_*` 环境变量作为默认值，也可以在后台“签到设置”里覆盖远程地址、认证方式、管理员账号密码/API Key 和超时时间。
- 使用管理员账号密码认证 Sub2API 时，后端会定时预热 access token，并保存到 `system_settings`。后续请求会优先复用未过期 token，临近过期再自动登录刷新。

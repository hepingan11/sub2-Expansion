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
- `GET /api/admin/platforms/:platform/settings`
- `PUT /api/admin/platforms/:platform/settings`
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

## 平台配置

社交平台可以覆盖签到和邀请配置。未配置的平台会沿用现有 `social` 配置，账号绑定、签到记录和邀请关系仍然基于同一个 Sub2API 用户 ID 共享。

```http
PUT /api/admin/platforms/telegram/settings
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "checkIn": {
    "dailyMaxUsers": 20,
    "groupLink": "https://t.me/your_group",
    "prizeTiers": [
      {"amount": 1, "probability": 70},
      {"amount": 3, "probability": 20},
      {"amount": 5, "probability": 8},
      {"amount": 10, "probability": 2}
    ]
  },
  "invitation": {
    "afterTime": "2026-07-13T00:00:00Z",
    "amount": 5
  }
}
```

## Telegram Bot

后台“系统设置”中可以配置和连接 Telegram Bot：

1. 在 BotFather 创建 Bot，拿到 Bot Token。
2. 进入后台 `系统设置 -> 公开访问地址`，填写用户能访问的前端地址，Bot 会用它生成网页登录绑定链接。
3. 进入后台 `系统设置 -> Telegram Bot`，填写 Bot Token、API 地址和轮询间隔。
4. 点击保存，再点“连接测试”。连接成功后后端会调用 Telegram `getMe`、清理旧 webhook，并立即启动或重启长轮询。

也可以用 `FRONTEND_PUBLIC_URL`、`TELEGRAM_BOT_*` 环境变量作为初始默认值；后台保存后的配置会优先生效。

```http
POST /api/admin/telegram/connect
Authorization: Bearer <admin-token>
```

已绑定 Telegram 的用户可使用：

- `/bind` 获取网页登录绑定链接
- `/checkin` 签到
- `/invite` 获取邀请码和 `t.me` 邀请链接
- `/me` 查看绑定状态
- `/help` 查看帮助

邀请链接使用 `/start <邀请码>`。新人打开链接后，Bot 会返回带 `platform=telegram`、`userid=<telegram user id>`、`invitecode=<邀请码>` 的网页登录绑定链接；登录后会写入共享绑定表并按 Telegram 平台配置发放邀请奖励。

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
TELEGRAM_BOT_ENABLED=false
TELEGRAM_BOT_TOKEN=
TELEGRAM_BOT_API_BASE_URL=https://api.telegram.org
TELEGRAM_BOT_POLL_INTERVAL_SECONDS=2
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

# Sub2 Expansion

Sub2 Expansion 是 Sub2API 的运营扩展服务，提供签到奖励、社交平台绑定、邀请奖励、累计充值活动、收藏网站和分组倍率监控等功能。

当前版本：`v0.2`

## v0.2 版本亮点

- 新增 Telegram Bot 长轮询接入，支持 `/bind`、`/checkin`、`/invite`、`/me` 和 `/help`。
- Telegram 邀请使用 `t.me/<bot>?start=<邀请码>` 深链；可在后台启用指定群成员校验，只有已入群用户才能获得绑定链接。
- Telegram 绑定链接带有短期签名凭证，网页登录时后端会验证凭证并再次检查群成员状态，避免手工拼接参数绕过校验。
- 支持按社交平台分别配置签到人数上限、奖励档位、群链接、邀请时间门槛和奖励金额。
- 分组倍率变化图改为阶梯折线图，悬浮任意时间点可同时查看所有分组当时生效的倍率。
- 优化 Docker 构建依赖缓存，并让版本更新检查同时兼容 `v0.2` 与 `v0.2.0` 格式。

## 架构

- 后端：Go、Gin、GORM、PostgreSQL
- 前端：React、Vite、ECharts
- 关联服务：Sub2API 管理 API
- 默认时区：`Asia/Shanghai`

项目内的 `sub2api/` 为关联项目源码，仅用于集成参考；扩展服务本身位于 `backend-go/` 和 `frontend/`。

## 功能

- 管理端维护签到金额档位、概率、每日配额和兑换码。
- 用户签到和社交平台签到；同一用户每天只能成功签到一次。
- 社交账号绑定与群机器人场景的邀请新用户流程，包括 Telegram 深链邀请、入群校验和签名绑定凭证。
- 邀请记录、近 30 日成功邀请人数和奖励金额趋势。
- 累计充值活动、奖励档位、领取记录、总返利及近 30 日返利趋势。
- 累计充值活动奖励通过 Sub2API 的管理员余额调整入账，不计入用户 `total_recharged`。
- 收藏网站管理、Sub2API 分组倍率监控、倍率变更记录和全分组阶梯图悬浮对比。
- `sub2api-admin` skill/CLI 支持社交签到、邀请码绑定链接、邀请记录和邀请统计查询。

## 业务规则

### 签到奖励

管理员配置金额与概率，所有概率之和必须为 `100%`。后端将概率转换为万分位区间后使用安全随机源抽样，最终校验和抽奖均在服务端完成。

直接签到和社交签到可以使用独立的每日配额和奖励档位。签到奖励使用 Sub2API 的余额兑换码流程；若当日已有签到记录，服务端返回原有记录，不会重复抽奖或发奖。

### 邀请新用户

1. 邀请人在用户中心生成专属邀请码。
2. 新用户注册 Sub2API 账号，且账号创建时间必须晚于管理员配置的时间门槛。
3. 机器人使用新用户的社交平台 ID 和邀请码请求绑定链接；Telegram 可先检查用户是否已加入指定群组。
4. 新用户必须自行打开该链接并登录，完成平台账号和邀请码绑定。
5. 后端校验邀请码、时间门槛、非自邀、未被其他邀请人绑定等条件后，向邀请人发放奖励。

绑定链接包含平台账号标识与邀请码，只能私发给对应新用户。启用 Telegram 入群校验后，链接还包含由后端签发的短期凭证，登录绑定时会再次验证群成员状态。邀请奖励仅以 `REWARDED` 状态为成功依据。

### 累计充值活动

活动按用户在 Sub2API 的 `total_recharged` 判断是否达到门槛。奖励领取后调用 Sub2API 管理员余额调整接口增加余额，并使用稳定幂等键防止重复入账；奖励金额不会计入累计充值。

## Docker 部署

Docker Compose 会启动 PostgreSQL、后端和 Nginx 前端，默认对外暴露前端 `6779` 端口，并由 Nginx 将 `/api/` 代理到后端。

### 一键安装（Linux）

服务器已安装 Docker Engine、Docker Compose v2 和 Git 时，可直接执行：

```bash
curl -fsSL https://raw.githubusercontent.com/hepingan11/sub2-Expansion/main/scripts/install.sh | sh
```

脚本会克隆项目、首次生成含随机密码和令牌密钥的 `.env`、构建镜像并启动服务。首次安装后，终端只显示一次后台管理员账号和密码；数据库密码与 `AUTH_SECRET` 仅保存在项目 `.env` 中。

指定安装目录：

```bash
curl -fsSL https://raw.githubusercontent.com/hepingan11/sub2-Expansion/main/scripts/install.sh -o /tmp/sub2-expansion-install.sh
INSTALL_DIR="$HOME/sub2-Expansion" sh /tmp/sub2-expansion-install.sh
```

可在执行前通过环境变量覆盖仓库、分支或端口：

```bash
curl -fsSL https://raw.githubusercontent.com/hepingan11/sub2-Expansion/main/scripts/install.sh -o /tmp/sub2-expansion-install.sh
REPO_URL="https://github.com/hepingan11/sub2-Expansion.git" BRANCH="main" HTTP_PORT="6779" \
  sh /tmp/sub2-expansion-install.sh
```

首次安装完成后，编辑 `<安装目录>/.env`，设置 `SUB2API_BASE_URL` 及相应的 Sub2API 管理认证信息；修改后执行 `docker compose up -d --build` 使配置生效。

脚本默认开启后台“一键更新”能力。该能力需要项目目录和 Docker Socket 挂载到后端容器，适合仅有可信管理员访问的环境。不需要时，在 `.env` 中将 `SYSTEM_UPDATE_COMMAND` 设为空后重新部署。

### 手动安装

```powershell
Copy-Item deploy.env.example .env
# 编辑 .env，替换所有密码、密钥和实际域名
docker compose up -d --build
docker compose ps
```

访问地址：`http://<server-host>:6779`

升级镜像后重新执行：

```powershell
docker compose up -d --build
```

若不使用后台一键更新，可手动切换到已发布标签后更新：

```bash
git fetch --tags origin
git checkout <release-tag>
# 同步将 .env 中的 APP_VERSION 设置为对应标签，例如 v0.2
docker compose up -d --build
```

## 本地开发

### 1. 启动 PostgreSQL

创建一个 PostgreSQL 数据库，例如 `redeem_code_system`。推荐复制后端示例配置：

```powershell
Copy-Item backend-go\.env.example backend-go\.env
```

编辑 `backend-go/.env` 中的数据库连接和管理员配置。`DB_DSN` 存在时优先于 `DB_URL`、`DB_USERNAME`、`DB_PASSWORD`。

### 2. 启动后端

```powershell
cd backend-go
go run .
```

默认地址：`http://127.0.0.1:8625`

健康检查：`http://127.0.0.1:8625/healthz`

后端会在启动时创建或补充所需的数据表和索引。

### 3. 启动前端

新开终端：

```powershell
cd frontend
npm install
$env:VITE_PROXY_TARGET="http://127.0.0.1:8625"
npm run dev
```

默认地址：`http://127.0.0.1:5173`

若前端和后端不通过同源反向代理部署，可在构建前设置 `VITE_API_BASE` 为后端公开地址：

```powershell
cd frontend
$env:VITE_API_BASE="https://api.example.com"
npm run build
```

## 关键配置

完整示例见 [deploy.env.example](deploy.env.example) 和 [backend-go/.env.example](backend-go/.env.example)。以下变量必须按部署环境调整：

| 变量 | 作用 |
| --- | --- |
| `POSTGRES_PASSWORD` / `DB_PASSWORD` | PostgreSQL 密码 |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | 扩展服务管理端账号 |
| `AUTH_SECRET` | 管理端令牌和 Telegram 绑定凭证的签名密钥，生产环境必须使用随机长字符串 |
| `APP_VERSION` | 当前扩展服务版本；v0.2 发布包应设为 `v0.2` |
| `CORS_ALLOWED_ORIGINS` | 允许访问后端的前端源，多个值以逗号分隔 |
| `FRONTEND_PUBLIC_URL` | 社交账号需要绑定时返回给用户的公开前端地址 |
| `SUB2API_BASE_URL` | Sub2API 服务地址 |
| `SUB2API_ADMIN_API_KEY` | Sub2API 管理 API Key，推荐优先使用 |
| `SUB2API_ADMIN_EMAIL` / `SUB2API_ADMIN_PASSWORD` | 未配置 API Key 时的 Sub2API 管理员登录凭据 |
| `CHECK_IN_DAILY_MAX_USERS` | 默认每日签到人数上限 |
| `SYSTEM_UPDATE_COMMAND` | 可选。后台触发系统更新时在容器内执行的命令 |

Sub2API 连接配置可在管理端“系统设置”中覆盖。使用账号密码认证时，后端会缓存和刷新 Sub2API access token。

Telegram Bot、目标群 Chat ID、加群链接、入群校验开关和绑定凭证有效期均可在管理端“系统设置”中维护。启用入群校验前，必须先将 Bot 设置为目标群管理员。

## 安全要求

- 不要提交 `.env`、生产密码、API Key、access token 或数据库连接串。
- 部署前替换示例中的 `POSTGRES_PASSWORD`、`ADMIN_PASSWORD` 和 `AUTH_SECRET`；修改 `AUTH_SECRET` 会使现有管理端令牌和未使用的 Telegram 绑定凭证失效。
- 生产环境将 `CORS_ALLOWED_ORIGINS` 设置为实际前端域名，避免使用宽泛来源。
- `FRONTEND_PUBLIC_URL` 必须是用户可访问的 HTTPS 公网地址，否则社交绑定链接无法正确返回。
- 只有后端可决定签到概率、邀请资格、奖励金额、累计充值门槛和实际入账操作；前端参数不构成信任边界。
- 配置 `SYSTEM_UPDATE_COMMAND` 或挂载 Docker Socket 前，确认宿主机权限边界；该能力可执行容器内更新命令。

## 验证

```powershell
cd backend-go
go test ./...

cd ..\frontend
npm run build
```

## 管理 API 概览

所有 `/api/admin/*` 接口除登录外均需要扩展服务管理员令牌。

- `POST /api/admin/login`
- `GET /api/admin/settings/check-in`
- `PUT /api/admin/settings/check-in`
- `GET /api/admin/recharge-activities`
- `GET /api/admin/recharge-reward-claims`
- `GET /api/admin/recharge-reward-stats`
- `GET /api/admin/invitations`
- `GET /api/admin/invitation-stats`

完整的 Sub2API 管理与机器人操作说明见 [skills/sub2api-admin/SKILL.md](skills/sub2api-admin/SKILL.md)。

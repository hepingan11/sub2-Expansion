# Sub2 Expansion v0.2

Sub2 Expansion v0.2 重点完善 Telegram 邀请绑定流程、平台独立配置和分组倍率监控体验，同时增强绑定链路的服务端校验与部署可维护性。

## 新增功能

### Telegram Bot

- 新增 Telegram Bot 长轮询接入。
- 支持 `/bind`、`/checkin`、`/invite`、`/me` 和 `/help` 指令。
- `/invite` 会生成 `https://t.me/<bot>?start=<邀请码>` 邀请深链，新用户无需在群内 `@机器人`。
- 管理后台可配置 Bot Token、Telegram API 地址、轮询间隔、目标群 Chat ID、加群链接和绑定凭证有效期。
- 可启用绑定前入群校验；Bot 会通过 Telegram `getChatMember` 检查用户是否属于指定群组。
- “连接测试”会验证 Bot Token、目标群访问权限及 Bot 管理员身份。

### 邀请绑定安全

- Telegram 用户通过入群检查后，Bot 才会签发短期绑定凭证。
- 网页登录绑定时，后端会校验签名凭证中的平台、Telegram User ID、邀请码和有效期。
- 正式写入绑定前会再次查询群成员状态，避免通过手工拼接 `platform + userid + invitecode` 绕过入群检查。
- 保留原有防重复绑定、禁止自邀、新人账号时间门槛和邀请奖励幂等处理。

### 平台独立配置

- 支持按社交平台分别配置每日签到人数上限、奖励概率档位和群链接。
- 支持按平台分别配置邀请活动时间门槛与奖励金额。
- 未设置平台专属规则时继续使用通用社交平台配置，兼容已有部署。

### 分组倍率监控

- 倍率变化图改为阶梯折线图，更准确地表达倍率在两次变更之间持续生效。
- 悬浮到任意时间点时会显示所有分组在该时刻生效的倍率。
- 分组尚无历史记录时显示“暂无”；分组较多时悬浮面板支持滚动。

## 优化与修复

- 后台新增“前端公开地址”，统一用于 Telegram 和其他社交平台的网页登录绑定链接。
- 优化前后端 Dockerfile 的依赖缓存，减少重复构建耗时。
- 优化 `sub2api-admin` CLI 的错误响应处理。
- 更新检查支持 `v0.2` 和 `v0.2.0` 两种版本格式。
- Docker 部署会自动启用内置更新脚本，不再要求手工配置 `SYSTEM_UPDATE_COMMAND`；重建任务由独立临时容器执行。
- 补充 Telegram 成员状态、绑定凭证签名、过期与篡改场景的自动测试。

## 升级说明

```bash
git fetch --tags origin
git checkout v0.2
# 将 .env 中的 APP_VERSION 更新为 v0.2
docker compose up -d --build
```

使用后台一键更新时，系统会切换到最新 Release 标签、更新 `.env` 中的 `APP_VERSION`，然后重新构建并启动服务。

升级不会自动启用 Telegram 入群校验。需要该能力时：

1. 将 Telegram Bot 设置为目标群管理员。
2. 在“系统设置 -> 公开访问地址”填写可访问的 HTTPS 前端地址。
3. 在“系统设置 -> Telegram Bot”填写目标群 Chat ID 和加群链接。
4. 启用“绑定前检查指定群成员身份”，保存后点击“连接测试”。

公开群可使用 `@群用户名` 作为 Chat ID；私有超级群通常使用 `-100...` 格式的数字 Chat ID。

## 兼容性

- 不需要修改 Sub2API 源码。
- 现有 PostgreSQL 数据、社交绑定和邀请记录会继续保留。
- 未启用 Telegram 入群校验时，现有 Telegram 绑定行为保持兼容。

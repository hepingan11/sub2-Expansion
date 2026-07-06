# 兑换码管理系统

技术栈：Java Spring Boot 3、Vite、React、MySQL。

## 功能

- 管理员登录。
- 兑换码列表、搜索、批量导入、编辑、删除。
- 后台粘贴外部兑换码并填写金额，导入为未绑定码池。
- 签到接口：同一用户每天只能签到一次，成功后按概率随机绑定一个未绑定兑换码。
- 支持每日最大签到用户数限制，超过后当天其它新用户不能再签到。
- 支持后台按兑换码金额配置获取概率。
- 兑换码字段：`code`、`userId`、`signDate`、`amount`、`status`。

## 签到概率

后台「签到设置」里可以按兑换码金额配置获取概率，默认配置为：

- 1.00 元：70%
- 3.00 元：20%
- 5.00 元：8%
- 10.00 元：2%

如果抽中的金额没有可用库存，系统会自动降级绑定任意金额的可用兑换码；如果码池完全为空，则签到返回库存不足。
所有金额概率之和必须等于 `100%`，后端会做最终校验并保存到 `system_settings`。

## 启动后端

先准备 MySQL，然后设置环境变量或直接使用默认值。默认连接信息是 `root/root`，数据库名是 `redeem_code_system`：

后端地址：`http://localhost:8080`。

如果你的本机 MySQL 不是 `root/root`，请把 `DB_USERNAME` 和 `DB_PASSWORD` 改成真实账号密码。后端启动失败时优先看 `Access denied for user`，这通常就是数据库账号密码不匹配。

如果你从旧版本升级，后端启动时会自动把 `redeem_codes.user_id`、`redeem_codes.sign_date` 修改为可空，并把旧状态兼容到新状态。


```powershell
$env:CHECK_IN_DAILY_MAX_USERS="1000"
```

## 启动前端

```powershell
cd frontend
npm install
npm run dev
```

前端地址：`http://localhost:5173`。

服务器部署时，如果前端通过 Vite 预览服务暴露 5173 端口，请使用：

```powershell
cd frontend
npm install
npm run build
$env:VITE_PROXY_TARGET="http://127.0.0.1:8080"
npm run preview -- --host 0.0.0.0 --port 5173
```

这样浏览器访问 `http://8.137.103.102:5173/api/admin/login` 时，Vite 会把 `/api` 请求转发到后端 `8080`，不会落到前端静态服务导致 404。

如果前端不走代理，而是直接请求后端，也可以在构建前设置：

```powershell
$env:VITE_API_BASE="http://8.137.103.102:8080"
npm run build
```


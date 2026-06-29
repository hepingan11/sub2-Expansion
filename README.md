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

```powershell
cd backend
$env:DB_URL="jdbc:mysql://localhost:3306/redeem_code_system?useUnicode=true&characterEncoding=utf8&connectionCollation=utf8mb4_unicode_ci&serverTimezone=Asia/Shanghai&createDatabaseIfNotExist=true"
$env:DB_USERNAME="root"
$env:DB_PASSWORD="root"
$env:ADMIN_USERNAME="admin"
$env:ADMIN_PASSWORD="admin123"
$env:AUTH_SECRET="please-change-this-secret"
mvn spring-boot:run
```

后端地址：`http://localhost:8080`。

如果你的本机 MySQL 不是 `root/root`，请把 `DB_USERNAME` 和 `DB_PASSWORD` 改成真实账号密码。后端启动失败时优先看 `Access denied for user`，这通常就是数据库账号密码不匹配。

如果你从旧版本升级，后端启动时会自动把 `redeem_codes.user_id`、`redeem_codes.sign_date` 修改为可空，并把旧状态兼容到新状态。

每日最大签到用户数默认 `1000`，首次启动会写入后台设置；之后可在后台「签到设置」里修改：

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

## 接口示例

批量导入兑换码：

```http
POST /api/admin/codes/batch-import
Authorization: Bearer <token>
Content-Type: application/json

{
  "amount": 5.00,
  "codesText": "3fc57f0bb83b974b38c96cbbf7120451\n33a385e2d5a61834118f08a6b93f18b0\n2e29074e03bf1eabe89b2c05a655e051"
}
```

返回：

```json
{
  "totalParsed": 3,
  "imported": 3,
  "duplicated": 0,
  "duplicatedCodes": []
}
```

### 签到接口

用于用户每日签到并领取一个兑换码。接口不需要管理员 `Authorization`，调用方只需要传入业务侧用户 ID。

```http
POST /api/checkins
Content-Type: application/json

{
  "userId": "10001"
}
```

请求字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `userId` | string | 是 | 用户唯一标识，不能为空。后端会自动去掉首尾空格。 |

成功返回 `200 OK`。第一次签到成功时，`alreadyCheckedIn` 为 `false`，并返回本次绑定的兑换码：

```json
{
  "success": true,
  "alreadyCheckedIn": false,
  "userId": "10001",
  "signDate": "2026-06-29",
  "code": "3FC57F0BB83B974B38C96CBBF7120451",
  "amount": 5.00,
  "message": "签到成功"
}
```

如果同一用户当天重复签到，也返回 `200 OK`，不会重新抽奖，会返回当天已绑定的同一个兑换码：

```json
{
  "success": true,
  "alreadyCheckedIn": true,
  "userId": "10001",
  "signDate": "2026-06-29",
  "code": "3FC57F0BB83B974B38C96CBBF7120451",
  "amount": 5.00,
  "message": "今日已签到"
}
```

返回字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `success` | boolean | 请求是否成功。成功签到或重复签到均为 `true`。 |
| `alreadyCheckedIn` | boolean | 是否为当天重复签到。`false` 表示本次新签到并绑定兑换码。 |
| `userId` | string | 签到用户 ID。 |
| `signDate` | string | 签到日期，格式为 `yyyy-MM-dd`，按后端服务器日期计算。 |
| `code` | string | 绑定给用户的兑换码。 |
| `amount` | number | 兑换码金额。 |
| `message` | string | 结果说明。 |

业务规则：

- 同一 `userId` 每天只能签到一次。
- 新签到会先按后台配置的金额概率抽取金额，再从该金额的未绑定码池随机绑定一个兑换码。
- 如果抽中的金额没有库存，会自动降级为任意金额的未绑定兑换码。
- 如果所有兑换码库存都不足，返回 `409 Conflict`。
- 如果当天签到人数达到后台配置的每日上限，新的用户签到返回 `409 Conflict`；当天已签到用户再次调用仍会返回原兑换码。

错误返回统一为：

```json
{
  "message": "错误原因"
}
```

常见错误：

| HTTP 状态 | 场景 | 示例 message |
| --- | --- | --- |
| `400 Bad Request` | `userId` 为空或缺失 | `userId must not be blank` |
| `409 Conflict` | 当天签到名额已满 | `今日签到名额已满` |
| `409 Conflict` | 兑换码库存不足 | `兑换码库存不足，请先在后台导入兑换码` |

管理员接口需要请求头：

```http
Authorization: Bearer <token>
```

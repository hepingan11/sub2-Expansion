# 兑换码管理系统

技术栈：Java Spring Boot 3、Vite、React、MySQL。

## 功能

- 管理员登录。
- 兑换码列表、搜索、批量导入、编辑、删除。
- 后台粘贴外部兑换码并填写金额，导入为未绑定码池。
- 签到接口：同一用户每天只能签到一次，成功后按概率随机绑定一个未绑定兑换码。
- 支持每日最大签到用户数限制，超过后当天其它新用户不能再签到。
- 兑换码字段：`code`、`userId`、`signDate`、`amount`、`status`。

## 签到概率

当前概率在 `backend/src/main/java/com/example/redeem/service/PrizeDrawService.java`：

- 1.00 元：70%
- 3.00 元：20%
- 5.00 元：8%
- 10.00 元：2%

如果抽中的金额没有可用库存，系统会自动降级绑定任意金额的可用兑换码；如果码池完全为空，则签到返回库存不足。

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

每日最大签到用户数通过环境变量配置，默认 `1000`：

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

签到：

```http
POST /api/checkins
Content-Type: application/json

{
  "userId": "10001"
}
```

返回：

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

管理员接口需要请求头：

```http
Authorization: Bearer <token>
```

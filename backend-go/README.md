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

## 启动

```powershell
cd backend-go
go mod tidy
go run .
```

默认监听 `8080`，可以用环境变量覆盖：

```powershell
$env:SERVER_PORT="8080"
$env:DB_URL="jdbc:mysql://localhost:3306/redeem_code_system?serverTimezone=Asia/Shanghai&createDatabaseIfNotExist=true"
$env:DB_USERNAME="root"
$env:DB_PASSWORD="root"
$env:ADMIN_USERNAME="admin"
$env:ADMIN_PASSWORD="admin123"
$env:AUTH_SECRET="please-change-this-secret"
$env:AUTH_TOKEN_TTL_HOURS="12"
$env:CORS_ALLOWED_ORIGINS="http://localhost:5173"
$env:CHECK_IN_DAILY_MAX_USERS="20"
go run .
```

也可以直接提供 Go MySQL DSN：

```powershell
$env:DB_DSN="root:root@tcp(localhost:3306)/redeem_code_system?charset=utf8mb4&parseTime=True&loc=Local&collation=utf8mb4_unicode_ci"
go run .
```

## 说明

- 复用 Java 后端的 MySQL 表结构，并在启动时执行等价迁移。
- 管理员 token 格式兼容原 Java HMAC token，不是 JWT。
- MySQL 连接池默认限制为 `MaxOpenConns=10`、`MaxIdleConns=5`，更适合小内存服务器。
- 兑换码金额和概率用 decimal 处理，JSON 仍返回数字格式。

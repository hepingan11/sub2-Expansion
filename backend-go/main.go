package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	statusAvailable = "AVAILABLE"
	statusAssigned  = "ASSIGNED"
	statusUsed      = "USED"
	statusVoided    = "VOIDED"

	dailyMaxUsersKey = "check_in.daily_max_users"
	prizeTiersKey    = "check_in.prize_tiers"

	sub2APIBaseURLKey       = "sub2api.base_url"
	sub2APIAdminAPIKeyKey   = "sub2api.admin_api_key"
	sub2APIJWTKey           = "sub2api.jwt"
	sub2APIAdminEmailKey    = "sub2api.admin_email"
	sub2APIAdminPasswordKey = "sub2api.admin_password"
	sub2APIAuthModeKey      = "sub2api.auth_mode"
	sub2APITimeoutKey       = "sub2api.timeout_seconds"
	sub2APIJWTExpiresAtKey  = "sub2api.jwt_expires_at"
)

const sub2APITokenRefreshBefore = 10 * time.Minute

var (
	validStatuses     = map[string]bool{statusAvailable: true, statusAssigned: true, statusUsed: true, statusVoided: true}
	defaultPrizeTiers = []PrizeTier{{Amount: MustAmount("1.00"), Probability: MustAmount("70.00")}, {Amount: MustAmount("3.00"), Probability: MustAmount("20.00")}, {Amount: MustAmount("5.00"), Probability: MustAmount("8.00")}, {Amount: MustAmount("10.00"), Probability: MustAmount("2.00")}}
	codeSplitPattern  = regexp.MustCompile(`[\,;\s]+`)
	codeAlphabet      = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
)

type Config struct {
	Port                 string
	DBURL                string
	DBUsername           string
	DBPassword           string
	AdminUsername        string
	AdminPassword        string
	AuthSecret           string
	AuthTokenTTLHours    int64
	CorsAllowedOrigins   []string
	CheckInDailyMaxUsers int
	Sub2APIBaseURL       string
	Sub2APIAdminAPIKey   string
	Sub2APIJWT           string
	Sub2APIAdminEmail    string
	Sub2APIAdminPassword string
	Sub2APITimeout       time.Duration
	Sub2APIRefreshToken  bool
	Sub2APIRefreshEvery  time.Duration
}

type App struct {
	db                 *gorm.DB
	cfg                Config
	sub2APITokenMu     sync.Mutex
	sub2APIToken       string
	sub2APITokenExpiry time.Time
}

type Amount struct {
	decimal.Decimal
}

func MustAmount(value string) Amount {
	amount, err := ParseAmount(value)
	if err != nil {
		panic(err)
	}
	return amount
}

func ParseAmount(value string) (Amount, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Amount{}, errors.New("amount is required")
	}
	d, err := decimal.NewFromString(value)
	if err != nil {
		return Amount{}, err
	}
	return Amount{d.Round(2)}, nil
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(a.Decimal.StringFixed(2)), nil
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		return errors.New("amount is required")
	}
	parsed, err := ParseAmount(raw)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

func (a Amount) Value() (driver.Value, error) {
	return a.Decimal.StringFixed(2), nil
}

func (a *Amount) Scan(value any) error {
	var raw string
	switch v := value.(type) {
	case nil:
		return errors.New("amount cannot be null")
	case []byte:
		raw = string(v)
	case string:
		raw = v
	default:
		raw = fmt.Sprint(v)
	}
	d, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	a.Decimal = d.Round(2)
	return nil
}

type LocalDate struct {
	time.Time
}

func Today() LocalDate {
	now := time.Now()
	return LocalDate{time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())}
}

func ParseLocalDate(value string) (LocalDate, error) {
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return LocalDate{}, err
	}
	return LocalDate{t}, nil
}

func (d LocalDate) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Format("2006-01-02"))
}

func (d *LocalDate) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		d.Time = time.Time{}
		return nil
	}
	parsed, err := ParseLocalDate(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d LocalDate) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Format("2006-01-02"), nil
}

func (d *LocalDate) Scan(value any) error {
	switch v := value.(type) {
	case time.Time:
		d.Time = time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.Local)
		return nil
	case []byte:
		parsed, err := ParseLocalDate(string(v))
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case string:
		parsed, err := ParseLocalDate(v)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	default:
		return fmt.Errorf("unsupported LocalDate value %T", value)
	}
}

type JSONTime struct {
	time.Time
}

func (t JSONTime) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Format("2006-01-02 15:04:05"))
}

func (t JSONTime) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *JSONTime) Scan(value any) error {
	switch v := value.(type) {
	case time.Time:
		t.Time = v
		return nil
	case []byte:
		return t.scanString(string(v))
	case string:
		return t.scanString(v)
	default:
		return fmt.Errorf("unsupported JSONTime value %T", value)
	}
}

func (t *JSONTime) scanString(value string) error {
	layouts := []string{"2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("invalid time: %s", value)
}

type RedeemCode struct {
	ID        uint64     `gorm:"primaryKey;column:id" json:"id"`
	Code      string     `gorm:"column:code;size:128;not null;uniqueIndex:uk_redeem_codes_code" json:"code"`
	UserID    *string    `gorm:"column:user_id;size:64;uniqueIndex:uk_redeem_codes_user_date;index:idx_redeem_codes_user_date" json:"userId"`
	SignDate  *LocalDate `gorm:"column:sign_date;type:date;uniqueIndex:uk_redeem_codes_user_date;index:idx_redeem_codes_user_date" json:"signDate"`
	Amount    Amount     `gorm:"column:amount;type:decimal(10,2);not null;index:idx_redeem_codes_status_amount" json:"amount"`
	Status    string     `gorm:"column:status;type:enum('AVAILABLE','ASSIGNED','USED','VOIDED');not null;index:idx_redeem_codes_status;index:idx_redeem_codes_status_amount" json:"status"`
	CreatedAt JSONTime   `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt JSONTime   `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (RedeemCode) TableName() string {
	return "redeem_codes"
}

func (r *RedeemCode) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if r.CreatedAt.Time.IsZero() {
		r.CreatedAt = now
	}
	if r.UpdatedAt.Time.IsZero() {
		r.UpdatedAt = now
	}
	return nil
}

func (r *RedeemCode) BeforeUpdate(*gorm.DB) error {
	r.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type CheckInRecord struct {
	ID           uint64    `gorm:"primaryKey;column:id"`
	UserID       string    `gorm:"column:user_id;size:64;not null;uniqueIndex:uk_check_in_records_user_date"`
	SignDate     LocalDate `gorm:"column:sign_date;type:date;not null;uniqueIndex:uk_check_in_records_user_date"`
	RedeemCodeID uint64    `gorm:"column:redeem_code_id;not null;index:idx_check_in_records_code_id"`
	CreatedAt    JSONTime  `gorm:"column:created_at;autoCreateTime"`
}

func (CheckInRecord) TableName() string {
	return "check_in_records"
}

func (r *CheckInRecord) BeforeCreate(*gorm.DB) error {
	if r.CreatedAt.Time.IsZero() {
		r.CreatedAt = JSONTime{Time: time.Now()}
	}
	return nil
}

type DailyCheckInLimit struct {
	SignDate     LocalDate `gorm:"primaryKey;column:sign_date;type:date"`
	CheckedCount int       `gorm:"column:checked_count;not null"`
	CreatedAt    JSONTime  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    JSONTime  `gorm:"column:updated_at;autoUpdateTime"`
}

func (DailyCheckInLimit) TableName() string {
	return "daily_checkin_limits"
}

func (l *DailyCheckInLimit) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if l.CreatedAt.Time.IsZero() {
		l.CreatedAt = now
	}
	if l.UpdatedAt.Time.IsZero() {
		l.UpdatedAt = now
	}
	return nil
}

func (l *DailyCheckInLimit) BeforeUpdate(*gorm.DB) error {
	l.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type SystemSetting struct {
	SettingKey   string   `gorm:"primaryKey;column:setting_key;size:100"`
	SettingValue string   `gorm:"column:setting_value;type:text;not null"`
	CreatedAt    JSONTime `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    JSONTime `gorm:"column:updated_at;autoUpdateTime"`
}

func (SystemSetting) TableName() string {
	return "system_settings"
}

func (s *SystemSetting) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if s.CreatedAt.Time.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.Time.IsZero() {
		s.UpdatedAt = now
	}
	return nil
}

func (s *SystemSetting) BeforeUpdate(*gorm.DB) error {
	s.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminLoginResponse struct {
	Token          string `json:"token"`
	ExpiresInHours int64  `json:"expiresInHours"`
}

type RedeemCodeRequest struct {
	Code     string     `json:"code"`
	UserID   string     `json:"userId"`
	SignDate *LocalDate `json:"signDate"`
	Amount   Amount     `json:"amount"`
	Status   string     `json:"status"`
}

type BatchImportCodesRequest struct {
	CodesText string `json:"codesText"`
	Amount    Amount `json:"amount"`
}

type BatchImportCodesResponse struct {
	TotalParsed     int      `json:"totalParsed"`
	Imported        int      `json:"imported"`
	Duplicated      int      `json:"duplicated"`
	DuplicatedCodes []string `json:"duplicatedCodes"`
}

type DashboardStatsResponse struct {
	Total       int64             `json:"total"`
	Available   int64             `json:"available"`
	Assigned    int64             `json:"assigned"`
	Used        int64             `json:"used"`
	Voided      int64             `json:"voided"`
	AmountStats []AmountStatEntry `json:"amountStats"`
}

type AmountStatEntry struct {
	Amount    Amount `json:"amount"`
	Total     int64  `json:"total"`
	Available int64  `json:"available"`
}

type CheckInRequest struct {
	UserID string `json:"userId"`
}

type CheckInResponse struct {
	Success          bool       `json:"success"`
	AlreadyCheckedIn bool       `json:"alreadyCheckedIn"`
	UserID           *string    `json:"userId"`
	SignDate         *LocalDate `json:"signDate"`
	Code             string     `json:"code"`
	Amount           Amount     `json:"amount"`
	Message          string     `json:"message"`
}

type PrizeTier struct {
	Amount      Amount `json:"amount"`
	Probability Amount `json:"probability"`
}

type CheckInSettingsResponse struct {
	DailyMaxUsers int           `json:"dailyMaxUsers"`
	PrizeTiers    []PrizeTier   `json:"prizeTiers"`
	Sub2API       Sub2APIConfig `json:"sub2api"`
}

type UpdateCheckInSettingsRequest struct {
	DailyMaxUsers int           `json:"dailyMaxUsers"`
	PrizeTiers    []PrizeTier   `json:"prizeTiers"`
	Sub2API       Sub2APIConfig `json:"sub2api"`
}

type Sub2APIConfig struct {
	BaseURL          string `json:"baseUrl"`
	AuthMode         string `json:"authMode"`
	AdminAPIKey      string `json:"adminApiKey,omitempty"`
	AdminAPIKeySet   bool   `json:"adminApiKeySet"`
	JWT              string `json:"jwt,omitempty"`
	JWTSet           bool   `json:"jwtSet"`
	AdminEmail       string `json:"adminEmail"`
	AdminPassword    string `json:"adminPassword,omitempty"`
	AdminPasswordSet bool   `json:"adminPasswordSet"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
}

type PageResponse[T any] struct {
	Content       []T   `json:"content"`
	TotalElements int64 `json:"totalElements"`
	TotalPages    int   `json:"totalPages"`
	Number        int   `json:"number"`
	Size          int   `json:"size"`
}

type APIError struct {
	Message string `json:"message"`
}

type sub2APIResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type sub2APIRedeemCode struct {
	Code string `json:"code"`
}

type sub2APILoginData struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func main() {
	loadDotEnv()

	cfg := loadConfig()
	db, err := openDatabase(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	app := &App{db: db, cfg: cfg}
	if err := app.migrate(); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	app.startSub2APITokenRefresher(context.Background())

	router := app.router()
	log.Printf("Go backend listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() Config {
	return Config{
		Port:                 env("SERVER_PORT", env("PORT", "8080")),
		DBURL:                env("DB_URL", "jdbc:mysql://8.137.103.102:3306/redeem_code_system?useUnicode=true&characterEncoding=utf8&connectionCollation=utf8mb4_unicode_ci&serverTimezone=Asia/Shanghai&createDatabaseIfNotExist=true"),
		DBUsername:           env("DB_USERNAME", "user"),
		DBPassword:           env("DB_PASSWORD", "123456"),
		AdminUsername:        env("ADMIN_USERNAME", "admin"),
		AdminPassword:        env("ADMIN_PASSWORD", "admin123"),
		AuthSecret:           env("AUTH_SECRET", "change-this-secret-to-a-long-random-string"),
		AuthTokenTTLHours:    envInt64("AUTH_TOKEN_TTL_HOURS", 12),
		CorsAllowedOrigins:   splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://8.137.103.102:5173")),
		CheckInDailyMaxUsers: envInt("CHECK_IN_DAILY_MAX_USERS", 20),
		Sub2APIBaseURL:       strings.TrimRight(env("SUB2API_BASE_URL", ""), "/"),
		Sub2APIAdminAPIKey:   env("SUB2API_ADMIN_API_KEY", ""),
		Sub2APIJWT:           env("SUB2API_JWT", ""),
		Sub2APIAdminEmail:    env("SUB2API_ADMIN_EMAIL", env("SUB2API_ADMIN_USERNAME", "")),
		Sub2APIAdminPassword: env("SUB2API_ADMIN_PASSWORD", ""),
		Sub2APITimeout:       time.Duration(envInt("SUB2API_TIMEOUT_SECONDS", 15)) * time.Second,
		Sub2APIRefreshToken:  envBool("SUB2API_TOKEN_REFRESH_ENABLED", true),
		Sub2APIRefreshEvery:  time.Duration(envInt("SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS", 300)) * time.Second,
	}
}

func openDatabase(cfg Config) (*gorm.DB, error) {
	dsn, createDatabaseDSN, databaseName, err := buildMySQLDSN(cfg)
	if err != nil {
		return nil, err
	}
	if createDatabaseDSN != "" && databaseName != "" {
		adminDB, err := gorm.Open(gormmysql.Open(createDatabaseDSN), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		if err := adminDB.Exec("CREATE DATABASE IF NOT EXISTS `" + strings.ReplaceAll(databaseName, "`", "``") + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
			return nil, err
		}
		sqlDB, _ := adminDB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func buildMySQLDSN(cfg Config) (dsn string, createDatabaseDSN string, databaseName string, err error) {
	if raw := env("DB_DSN", ""); raw != "" {
		return raw, "", "", nil
	}

	raw := strings.TrimPrefix(cfg.DBURL, "jdbc:")
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", "", err
	}
	databaseName = strings.TrimPrefix(parsed.Path, "/")
	if databaseName == "" {
		return "", "", "", errors.New("database name is required in DB_URL")
	}

	host := parsed.Host
	params := url.Values{}
	params.Set("charset", "utf8mb4")
	params.Set("parseTime", "True")
	params.Set("loc", "Local")
	params.Set("collation", "utf8mb4_unicode_ci")

	dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", cfg.DBUsername, cfg.DBPassword, host, databaseName, params.Encode())
	createDatabaseDSN = fmt.Sprintf("%s:%s@tcp(%s)/?%s", cfg.DBUsername, cfg.DBPassword, host, params.Encode())
	return dsn, createDatabaseDSN, databaseName, nil
}

func (app *App) migrate() error {
	sqlStatements := []string{
		`CREATE TABLE IF NOT EXISTS redeem_codes (
			id BIGINT NOT NULL AUTO_INCREMENT,
			code VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
			user_id VARCHAR(64) NULL,
			sign_date DATE NULL,
			amount DECIMAL(10, 2) NOT NULL,
			status VARCHAR(20) NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_redeem_codes_code (code),
			UNIQUE KEY uk_redeem_codes_user_date (user_id, sign_date),
			KEY idx_redeem_codes_user_date (user_id, sign_date),
			KEY idx_redeem_codes_status_amount (status, amount),
			KEY idx_redeem_codes_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS check_in_records (
			id BIGINT NOT NULL AUTO_INCREMENT,
			user_id VARCHAR(64) NOT NULL,
			sign_date DATE NOT NULL,
			redeem_code_id BIGINT NOT NULL,
			created_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_check_in_records_user_date (user_id, sign_date),
			KEY idx_check_in_records_code_id (redeem_code_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS daily_checkin_limits (
			sign_date DATE NOT NULL,
			checked_count INT NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (sign_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS system_settings (
			setting_key VARCHAR(100) NOT NULL,
			setting_value TEXT NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (setting_key)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`UPDATE redeem_codes SET status = 'ASSIGNED' WHERE status = 'ISSUED'`,
		`ALTER TABLE redeem_codes MODIFY COLUMN user_id VARCHAR(64) NULL`,
		`ALTER TABLE redeem_codes MODIFY COLUMN sign_date DATE NULL`,
		`ALTER TABLE redeem_codes MODIFY COLUMN code VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL`,
		`ALTER TABLE redeem_codes MODIFY COLUMN status ENUM('AVAILABLE','ASSIGNED','USED','VOIDED') NOT NULL`,
		`ALTER TABLE system_settings MODIFY COLUMN setting_value TEXT NOT NULL`,
	}

	for _, statement := range sqlStatements {
		if err := app.db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (app *App) router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     app.cfg.CorsAllowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"*"},
		AllowCredentials: false,
		MaxAge:           time.Hour,
	}))

	router.POST("/api/checkins", app.checkIn)

	admin := router.Group("/api/admin")
	admin.POST("/login", app.login)
	protected := router.Group("/api/admin", app.adminAuth)
	protected.GET("/codes", app.listCodes)
	protected.GET("/codes/:id", app.getCode)
	protected.POST("/codes", app.createCode)
	protected.POST("/codes/batch-import", app.batchImportCodes)
	protected.PUT("/codes/:id", app.updateCode)
	protected.DELETE("/codes/:id", app.deleteCode)
	protected.GET("/stats", app.stats)
	protected.GET("/settings/check-in", app.getCheckInSettings)
	protected.PUT("/settings/check-in", app.updateCheckInSettings)

	return router
}

func (app *App) login(c *gin.Context) {
	var req AdminLoginRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		badRequest(c, "username/password must not be blank")
		return
	}
	if req.Username != app.cfg.AdminUsername || !app.matchesPassword(req.Password, app.cfg.AdminPassword) {
		badRequest(c, "用户名或密码错误")
		return
	}
	c.JSON(http.StatusOK, AdminLoginResponse{Token: app.issueToken(req.Username), ExpiresInHours: app.cfg.AuthTokenTTLHours})
}

func (app *App) listCodes(c *gin.Context) {
	page := max(queryInt(c, "page", 0), 0)
	size := min(max(queryInt(c, "size", 10), 1), 100)
	var total int64
	var codes []RedeemCode

	query := app.applyCodeFilters(app.db.Model(&RedeemCode{}), c)
	if err := query.Count(&total).Error; err != nil {
		serverError(c, err)
		return
	}
	if err := query.Order("created_at DESC").Limit(size).Offset(page * size).Find(&codes).Error; err != nil {
		serverError(c, err)
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}
	c.JSON(http.StatusOK, PageResponse[RedeemCode]{
		Content:       codes,
		TotalElements: total,
		TotalPages:    totalPages,
		Number:        page,
		Size:          size,
	})
}

func (app *App) getCode(c *gin.Context) {
	code, ok := app.findCodeByID(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, code)
}

func (app *App) createCode(c *gin.Context) {
	var req RedeemCodeRequest
	if !bindJSON(c, &req) || !validateCodeRequest(c, &req) {
		return
	}
	code := RedeemCode{}
	if err := app.applyCodeRequest(&code, req, true); err != nil {
		conflict(c, err.Error())
		return
	}
	if err := app.db.Create(&code).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, code)
}

func (app *App) updateCode(c *gin.Context) {
	code, ok := app.findCodeByID(c)
	if !ok {
		return
	}
	var req RedeemCodeRequest
	if !bindJSON(c, &req) || !validateCodeRequest(c, &req) {
		return
	}
	if err := app.applyCodeRequest(&code, req, false); err != nil {
		conflict(c, err.Error())
		return
	}
	if err := app.db.Save(&code).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, code)
}

func (app *App) deleteCode(c *gin.Context) {
	code, ok := app.findCodeByID(c)
	if !ok {
		return
	}
	if err := app.db.Delete(&code).Error; err != nil {
		serverError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (app *App) batchImportCodes(c *gin.Context) {
	var req BatchImportCodesRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.CodesText) == "" {
		badRequest(c, "codesText must not be blank")
		return
	}
	if req.Amount.Cmp(decimal.NewFromFloat(0.01)) < 0 {
		badRequest(c, "amount must be greater than or equal to 0.01")
		return
	}
	parsedCodes := parseCodes(req.CodesText)
	if len(parsedCodes) == 0 {
		badRequest(c, "请至少粘贴一个兑换码")
		return
	}

	var existing []RedeemCode
	if err := app.db.Where("code IN ?", parsedCodes).Find(&existing).Error; err != nil {
		serverError(c, err)
		return
	}
	existingSet := map[string]bool{}
	duplicatedCodes := make([]string, 0, len(existing))
	for _, code := range existing {
		existingSet[code.Code] = true
		duplicatedCodes = append(duplicatedCodes, code.Code)
	}

	newCodes := make([]RedeemCode, 0, len(parsedCodes)-len(existingSet))
	for _, value := range parsedCodes {
		if existingSet[value] {
			continue
		}
		newCodes = append(newCodes, RedeemCode{Code: value, Amount: req.Amount, Status: statusAvailable})
	}
	if len(newCodes) > 0 {
		if err := app.db.Create(&newCodes).Error; err != nil {
			handleDBError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, BatchImportCodesResponse{
		TotalParsed:     len(parsedCodes),
		Imported:        len(newCodes),
		Duplicated:      len(existingSet),
		DuplicatedCodes: duplicatedCodes,
	})
}

func (app *App) stats(c *gin.Context) {
	countStatus := func(status string) int64 {
		var count int64
		_ = app.db.Model(&RedeemCode{}).Where("status = ?", status).Count(&count).Error
		return count
	}
	var total int64
	if err := app.db.Model(&RedeemCode{}).Count(&total).Error; err != nil {
		serverError(c, err)
		return
	}
	amountStats, err := app.amountStats()
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, DashboardStatsResponse{
		Total:       total,
		Available:   countStatus(statusAvailable),
		Assigned:    countStatus(statusAssigned),
		Used:        countStatus(statusUsed),
		Voided:      countStatus(statusVoided),
		AmountStats: amountStats,
	})
}

func (app *App) amountStats() ([]AmountStatEntry, error) {
	var result []AmountStatEntry
	err := app.db.Model(&RedeemCode{}).
		Select("amount, COUNT(*) AS total, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS available", statusAvailable).
		Group("amount").
		Order("amount ASC").
		Scan(&result).Error
	return result, err
}

func (app *App) getCheckInSettings(c *gin.Context) {
	settings, err := app.loadCheckInSettings()
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (app *App) updateCheckInSettings(c *gin.Context) {
	var req UpdateCheckInSettingsRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.DailyMaxUsers < 0 {
		badRequest(c, "每日签到上限不能小于 0")
		return
	}
	tiers, err := normalizePrizeTiers(req.PrizeTiers)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := app.saveSetting(dailyMaxUsersKey, strconv.Itoa(req.DailyMaxUsers)); err != nil {
		serverError(c, err)
		return
	}
	encoded, err := json.Marshal(tiers)
	if err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(prizeTiersKey, string(encoded)); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSub2APIConfig(req.Sub2API); err != nil {
		serverError(c, err)
		return
	}
	settings, err := app.loadCheckInSettings()
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (app *App) checkIn(c *gin.Context) {
	var req CheckInRequest
	if !bindJSON(c, &req) {
		return
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		badRequest(c, "userId must not be blank")
		return
	}

	today := Today()
	var existingRecord CheckInRecord
	err := app.db.Where("user_id = ? AND sign_date = ?", userID, today).First(&existingRecord).Error
	if err == nil {
		var code RedeemCode
		if err := app.db.First(&code, existingRecord.RedeemCodeID).Error; err == nil {
			c.JSON(http.StatusOK, toCheckInResponse(code, true, "今日已签到"))
			return
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		serverError(c, err)
		return
	}

	response, err := app.createCheckIn(c.Request.Context(), userID, today)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicateEntry(err) {
			var record CheckInRecord
			if err := app.db.Where("user_id = ? AND sign_date = ?", userID, today).First(&record).Error; err == nil {
				var code RedeemCode
				if err := app.db.First(&code, record.RedeemCodeID).Error; err == nil {
					c.JSON(http.StatusOK, toCheckInResponse(code, true, "今日已签到"))
					return
				}
			}
		}
		if isBusinessConflict(err) {
			conflict(c, err.Error())
			return
		}
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) createCheckIn(ctx context.Context, userID string, today LocalDate) (CheckInResponse, error) {
	var response CheckInResponse
	err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := app.consumeDailyQuota(tx, today); err != nil {
			return err
		}
		drawnAmount, err := app.drawAmount()
		if err != nil {
			return err
		}
		remoteCode, err := app.generateSub2APIRedeemCode(ctx, userID, today, drawnAmount)
		if err != nil {
			return err
		}

		savedCode := RedeemCode{
			Code:     remoteCode,
			UserID:   &userID,
			SignDate: &today,
			Amount:   drawnAmount,
			Status:   statusAssigned,
		}
		if err := tx.Create(&savedCode).Error; err != nil {
			return err
		}
		record := CheckInRecord{UserID: userID, SignDate: today, RedeemCodeID: savedCode.ID}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		response = toCheckInResponse(savedCode, false, "签到成功")
		return nil
	})
	return response, err
}

func (app *App) generateSub2APIRedeemCode(ctx context.Context, userID string, today LocalDate, amount Amount) (string, error) {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		return "", err
	}
	if cfg.BaseURL == "" {
		return "", businessConflict("Sub2API 未配置：请设置 SUB2API_BASE_URL")
	}
	authName, authValue, err := app.sub2APIAuthHeader(ctx, cfg)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"count": 1,
		"type":  "balance",
		"value": amount.InexactFloat64(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := cfg.BaseURL + "/api/v1/admin/redeem-codes/generate"
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Idempotency-Key", sub2APIIdempotencyKey(userID, today, amount))
	req.Header.Set(authName, authValue)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("生成 Sub2API 兑换码失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var envelope sub2APIResponse[[]sub2APIRedeemCode]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("解析 Sub2API 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("生成 Sub2API 兑换码失败: %s", message)
	}
	if len(envelope.Data) == 0 || strings.TrimSpace(envelope.Data[0].Code) == "" {
		return "", errors.New("生成 Sub2API 兑换码失败: 响应中没有兑换码")
	}
	return strings.TrimSpace(envelope.Data[0].Code), nil
}

func (app *App) sub2APIAuthHeader(ctx context.Context, cfg Sub2APIConfig) (string, string, error) {
	switch normalizeSub2APIAuthMode(cfg.AuthMode) {
	case "admin_api_key":
		if cfg.AdminAPIKey == "" {
			return "", "", businessConflict("Sub2API 未配置：当前认证方式需要 Admin API Key")
		}
		return "x-api-key", cfg.AdminAPIKey, nil
	case "jwt":
		if cfg.JWT == "" {
			return "", "", businessConflict("Sub2API 未配置：当前认证方式需要 JWT")
		}
		return "Authorization", "Bearer " + cfg.JWT, nil
	default:
		token, err := app.sub2APILogin(ctx, cfg)
		if err != nil {
			return "", "", err
		}
		return "Authorization", "Bearer " + token, nil
	}
}

func (app *App) startSub2APITokenRefresher(ctx context.Context) {
	if !app.cfg.Sub2APIRefreshToken {
		return
	}
	interval := app.cfg.Sub2APIRefreshEvery
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	go func() {
		app.refreshSub2APITokenOnce(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				app.refreshSub2APITokenOnce(ctx)
			}
		}
	}()
}

func (app *App) refreshSub2APITokenOnce(ctx context.Context) {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		log.Printf("Sub2API token refresh skipped: %v", err)
		return
	}
	if normalizeSub2APIAuthMode(cfg.AuthMode) != "password" {
		return
	}
	if cfg.BaseURL == "" || cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return
	}
	if _, err := app.sub2APILogin(ctx, cfg); err != nil {
		log.Printf("Sub2API token refresh failed: %v", err)
	}
}

func (app *App) sub2APILogin(ctx context.Context, cfg Sub2APIConfig) (string, error) {
	if cfg.BaseURL == "" {
		return "", businessConflict("Sub2API 未配置：请设置 SUB2API_BASE_URL")
	}
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return "", businessConflict("Sub2API 未配置：请设置 SUB2API_ADMIN_API_KEY、SUB2API_JWT，或 SUB2API_ADMIN_EMAIL/SUB2API_ADMIN_PASSWORD")
	}

	app.sub2APITokenMu.Lock()
	defer app.sub2APITokenMu.Unlock()
	if app.sub2APIToken != "" && time.Now().Before(app.sub2APITokenExpiry.Add(-sub2APITokenRefreshBefore)) {
		return app.sub2APIToken, nil
	}
	if token, expiresAt, ok := app.loadStoredSub2APIToken(); ok && time.Now().Before(expiresAt.Add(-sub2APITokenRefreshBefore)) {
		app.sub2APIToken = token
		app.sub2APITokenExpiry = expiresAt
		return token, nil
	}

	payload := map[string]string{
		"email":    cfg.AdminEmail,
		"password": cfg.AdminPassword,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, cfg.BaseURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("登录 Sub2API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var envelope sub2APIResponse[sub2APILoginData]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("解析 Sub2API 登录响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("登录 Sub2API 失败: %s", message)
	}
	if strings.TrimSpace(envelope.Data.AccessToken) == "" {
		return "", errors.New("登录 Sub2API 失败: 响应中没有 access_token")
	}

	expiresIn := envelope.Data.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	app.sub2APIToken = strings.TrimSpace(envelope.Data.AccessToken)
	app.sub2APITokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	if err := app.saveSub2APIToken(app.sub2APIToken, app.sub2APITokenExpiry); err != nil {
		log.Printf("save Sub2API token failed: %v", err)
	}
	return app.sub2APIToken, nil
}

func (app *App) loadStoredSub2APIToken() (string, time.Time, bool) {
	token, found, err := app.getSetting(sub2APIJWTKey)
	if err != nil {
		log.Printf("load Sub2API token failed: %v", err)
		return "", time.Time{}, false
	}
	if !found || strings.TrimSpace(token) == "" {
		return "", time.Time{}, false
	}
	rawExpiresAt, found, err := app.getSetting(sub2APIJWTExpiresAtKey)
	if err != nil {
		log.Printf("load Sub2API token expiry failed: %v", err)
		return "", time.Time{}, false
	}
	if !found {
		return "", time.Time{}, false
	}
	expiresAt, err := parseSub2APITokenExpiry(rawExpiresAt)
	if err != nil {
		log.Printf("parse Sub2API token expiry failed: %v", err)
		return "", time.Time{}, false
	}
	return strings.TrimSpace(token), expiresAt, true
}

func (app *App) saveSub2APIToken(token string, expiresAt time.Time) error {
	if strings.TrimSpace(token) == "" || expiresAt.IsZero() {
		return nil
	}
	if err := app.saveSetting(sub2APIJWTKey, strings.TrimSpace(token)); err != nil {
		return err
	}
	return app.saveSetting(sub2APIJWTExpiresAtKey, strconv.FormatInt(expiresAt.Unix(), 10))
}

func parseSub2APITokenExpiry(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty token expiry")
	}
	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unixSeconds, 0), nil
	}
	return time.Parse(time.RFC3339, value)
}

func (app *App) clearSub2APIToken() {
	app.sub2APITokenMu.Lock()
	defer app.sub2APITokenMu.Unlock()
	app.sub2APIToken = ""
	app.sub2APITokenExpiry = time.Time{}
}

func sub2APIIdempotencyKey(userID string, today LocalDate, amount Amount) string {
	sum := sha256.Sum256([]byte(userID))
	return fmt.Sprintf("checkin-%s-%x-%s", today.Format("2006-01-02"), sum[:8], amount.StringFixed(2))
}

func (app *App) consumeDailyQuota(tx *gorm.DB, today LocalDate) error {
	dailyMaxUsers, err := app.getDailyMaxUsers()
	if err != nil {
		return err
	}
	if dailyMaxUsers <= 0 {
		return businessConflict("今日签到名额已满")
	}

	var limit DailyCheckInLimit
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sign_date = ?", today).First(&limit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		limit = DailyCheckInLimit{SignDate: today, CheckedCount: 0}
		if err := tx.Create(&limit).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if limit.CheckedCount >= dailyMaxUsers {
		return businessConflict("今日签到名额已满")
	}
	return tx.Model(&DailyCheckInLimit{}).Where("sign_date = ?", today).Updates(map[string]any{
		"checked_count": gorm.Expr("checked_count + 1"),
		"updated_at":    time.Now(),
	}).Error
}

func assignRandomAvailable(tx *gorm.DB, userID string, today LocalDate, amount *Amount) (int64, error) {
	statement := `UPDATE redeem_codes
		SET user_id = ?, sign_date = ?, status = 'ASSIGNED', updated_at = CURRENT_TIMESTAMP(6)
		WHERE status = 'AVAILABLE'`
	args := []any{userID, today}
	if amount != nil {
		statement += ` AND amount = ?`
		args = append(args, *amount)
	}
	statement += ` ORDER BY RAND() LIMIT 1`
	result := tx.Exec(statement, args...)
	return result.RowsAffected, result.Error
}

func (app *App) loadCheckInSettings() (CheckInSettingsResponse, error) {
	dailyMaxUsers, err := app.getDailyMaxUsers()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	tiers, err := app.getPrizeTiers()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	sub2api, err := app.effectiveSub2APIConfig()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	sub2api.AdminAPIKeySet = sub2api.AdminAPIKey != ""
	sub2api.JWTSet = sub2api.JWT != ""
	sub2api.AdminPasswordSet = sub2api.AdminPassword != ""
	sub2api.AdminAPIKey = ""
	sub2api.JWT = ""
	sub2api.AdminPassword = ""
	return CheckInSettingsResponse{DailyMaxUsers: dailyMaxUsers, PrizeTiers: tiers, Sub2API: sub2api}, nil
}

func (app *App) effectiveSub2APIConfig() (Sub2APIConfig, error) {
	timeoutSeconds := int(app.cfg.Sub2APITimeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	cfg := Sub2APIConfig{
		BaseURL:          app.cfg.Sub2APIBaseURL,
		AuthMode:         defaultSub2APIAuthMode(app.cfg),
		AdminAPIKey:      app.cfg.Sub2APIAdminAPIKey,
		JWT:              app.cfg.Sub2APIJWT,
		AdminEmail:       app.cfg.Sub2APIAdminEmail,
		AdminPassword:    app.cfg.Sub2APIAdminPassword,
		TimeoutSeconds:   timeoutSeconds,
		AdminAPIKeySet:   app.cfg.Sub2APIAdminAPIKey != "",
		JWTSet:           app.cfg.Sub2APIJWT != "",
		AdminPasswordSet: app.cfg.Sub2APIAdminPassword != "",
	}
	var err error
	if cfg.BaseURL, err = app.settingOrDefault(sub2APIBaseURLKey, cfg.BaseURL); err != nil {
		return Sub2APIConfig{}, err
	}
	if cfg.AuthMode, err = app.settingOrDefault(sub2APIAuthModeKey, cfg.AuthMode); err != nil {
		return Sub2APIConfig{}, err
	}
	if cfg.AdminAPIKey, err = app.settingOrDefault(sub2APIAdminAPIKeyKey, cfg.AdminAPIKey); err != nil {
		return Sub2APIConfig{}, err
	}
	if cfg.JWT, err = app.settingOrDefault(sub2APIJWTKey, cfg.JWT); err != nil {
		return Sub2APIConfig{}, err
	}
	if cfg.AdminEmail, err = app.settingOrDefault(sub2APIAdminEmailKey, cfg.AdminEmail); err != nil {
		return Sub2APIConfig{}, err
	}
	if cfg.AdminPassword, err = app.settingOrDefault(sub2APIAdminPasswordKey, cfg.AdminPassword); err != nil {
		return Sub2APIConfig{}, err
	}
	timeoutValue, err := app.settingOrDefault(sub2APITimeoutKey, strconv.Itoa(cfg.TimeoutSeconds))
	if err != nil {
		return Sub2APIConfig{}, err
	}
	if parsed, parseErr := strconv.Atoi(strings.TrimSpace(timeoutValue)); parseErr == nil && parsed > 0 {
		cfg.TimeoutSeconds = parsed
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.AuthMode = normalizeSub2APIAuthMode(cfg.AuthMode)
	cfg.AdminAPIKey = strings.TrimSpace(cfg.AdminAPIKey)
	cfg.JWT = strings.TrimSpace(cfg.JWT)
	cfg.AdminEmail = strings.TrimSpace(cfg.AdminEmail)
	return cfg, nil
}

func (app *App) saveSub2APIConfig(cfg Sub2APIConfig) error {
	timeoutSeconds := cfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	values := map[string]string{
		sub2APIBaseURLKey:    strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		sub2APIAuthModeKey:   normalizeSub2APIAuthMode(cfg.AuthMode),
		sub2APIAdminEmailKey: strings.TrimSpace(cfg.AdminEmail),
		sub2APITimeoutKey:    strconv.Itoa(timeoutSeconds),
	}
	for key, value := range values {
		if err := app.saveSetting(key, value); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.AdminAPIKey) != "" {
		if err := app.saveSetting(sub2APIAdminAPIKeyKey, strings.TrimSpace(cfg.AdminAPIKey)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.JWT) != "" {
		if err := app.saveSetting(sub2APIJWTKey, strings.TrimSpace(cfg.JWT)); err != nil {
			return err
		}
	}
	if cfg.AdminPassword != "" {
		if err := app.saveSetting(sub2APIAdminPasswordKey, cfg.AdminPassword); err != nil {
			return err
		}
	}
	app.clearSub2APIToken()
	return nil
}

func defaultSub2APIAuthMode(cfg Config) string {
	if cfg.Sub2APIAdminAPIKey != "" {
		return "admin_api_key"
	}
	if cfg.Sub2APIJWT != "" {
		return "jwt"
	}
	return "password"
}

func normalizeSub2APIAuthMode(value string) string {
	switch strings.TrimSpace(value) {
	case "admin_api_key", "jwt", "password":
		return strings.TrimSpace(value)
	default:
		return "password"
	}
}

func (app *App) settingOrDefault(key, fallback string) (string, error) {
	value, found, err := app.getSetting(key)
	if err != nil {
		return "", err
	}
	if !found {
		return fallback, nil
	}
	return value, nil
}

func (app *App) getDailyMaxUsers() (int, error) {
	value, found, err := app.getSetting(dailyMaxUsersKey)
	if err != nil {
		return 0, err
	}
	if !found {
		defaultValue := max(app.cfg.CheckInDailyMaxUsers, 0)
		return defaultValue, app.saveSetting(dailyMaxUsersKey, strconv.Itoa(defaultValue))
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		defaultValue := max(app.cfg.CheckInDailyMaxUsers, 0)
		return defaultValue, app.saveSetting(dailyMaxUsersKey, strconv.Itoa(defaultValue))
	}
	return parsed, nil
}

func (app *App) getPrizeTiers() ([]PrizeTier, error) {
	value, found, err := app.getSetting(prizeTiersKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return defaultPrizeTiers, app.savePrizeTiers(defaultPrizeTiers)
	}
	var tiers []PrizeTier
	if err := json.Unmarshal([]byte(value), &tiers); err != nil {
		return defaultPrizeTiers, app.savePrizeTiers(defaultPrizeTiers)
	}
	normalized, err := normalizePrizeTiers(tiers)
	if err != nil {
		return defaultPrizeTiers, app.savePrizeTiers(defaultPrizeTiers)
	}
	return normalized, nil
}

func (app *App) drawAmount() (Amount, error) {
	tiers, err := app.getPrizeTiers()
	if err != nil {
		return Amount{}, err
	}
	roll, err := secureInt(10000)
	if err != nil {
		roll = mathrand.Intn(10000) + 1
	}
	cumulative := 0
	for _, tier := range tiers {
		cumulative += int(tier.Probability.Mul(decimal.NewFromInt(100)).IntPart())
		if roll <= cumulative {
			return tier.Amount, nil
		}
	}
	return tiers[len(tiers)-1].Amount, nil
}

func secureInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("max must be positive")
	}
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return 0, err
	}
	var value uint64
	for _, b := range randomBytes {
		value = (value << 8) | uint64(b)
	}
	return int(value%uint64(max)) + 1, nil
}

func (app *App) savePrizeTiers(tiers []PrizeTier) error {
	encoded, err := json.Marshal(tiers)
	if err != nil {
		return err
	}
	return app.saveSetting(prizeTiersKey, string(encoded))
}

func (app *App) getSetting(key string) (string, bool, error) {
	var setting SystemSetting
	err := app.db.First(&setting, "setting_key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return setting.SettingValue, true, nil
}

func (app *App) saveSetting(key, value string) error {
	setting := SystemSetting{SettingKey: key, SettingValue: value}
	return app.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "setting_key"}},
		DoUpdates: clause.Assignments(map[string]any{"setting_value": value, "updated_at": time.Now()}),
	}).Create(&setting).Error
}

func normalizePrizeTiers(tiers []PrizeTier) ([]PrizeTier, error) {
	if len(tiers) == 0 {
		return nil, errors.New("请至少配置一个兑换码金额概率")
	}
	merged := map[string]Amount{}
	for _, tier := range tiers {
		if tier.Amount.Cmp(decimal.Zero) <= 0 {
			return nil, errors.New("金额必须大于 0")
		}
		if tier.Probability.Cmp(decimal.Zero) <= 0 || tier.Probability.Cmp(decimal.NewFromInt(100)) > 0 {
			return nil, errors.New("概率必须大于 0 且不超过 100")
		}
		amount := Amount{tier.Amount.Round(2)}
		probability := Amount{tier.Probability.Round(2)}
		key := amount.StringFixed(2)
		if existing, ok := merged[key]; ok {
			merged[key] = Amount{existing.Add(probability.Decimal)}
		} else {
			merged[key] = probability
		}
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, _ := decimal.NewFromString(keys[i])
		right, _ := decimal.NewFromString(keys[j])
		return left.LessThan(right)
	})
	normalized := make([]PrizeTier, 0, len(keys))
	total := decimal.Zero
	for _, key := range keys {
		amount, _ := ParseAmount(key)
		probability := Amount{merged[key].Round(2)}
		total = total.Add(probability.Decimal)
		normalized = append(normalized, PrizeTier{Amount: amount, Probability: probability})
	}
	if !total.Round(2).Equal(decimal.NewFromInt(100)) {
		return nil, errors.New("所有金额概率之和必须等于 100%")
	}
	return normalized, nil
}

func (app *App) applyCodeFilters(query *gorm.DB, c *gin.Context) *gorm.DB {
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR user_id LIKE ?", pattern, pattern)
	}
	if userID := strings.TrimSpace(c.Query("userId")); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if startDate := strings.TrimSpace(c.Query("startDate")); startDate != "" {
		if parsed, err := ParseLocalDate(startDate); err == nil {
			query = query.Where("sign_date >= ?", parsed)
		}
	}
	if endDate := strings.TrimSpace(c.Query("endDate")); endDate != "" {
		if parsed, err := ParseLocalDate(endDate); err == nil {
			query = query.Where("sign_date <= ?", parsed)
		}
	}
	return query
}

func (app *App) findCodeByID(c *gin.Context) (RedeemCode, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "invalid id")
		return RedeemCode{}, false
	}
	var code RedeemCode
	if err := app.db.First(&code, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "Redeem code not found: " + c.Param("id")})
			return RedeemCode{}, false
		}
		serverError(c, err)
		return RedeemCode{}, false
	}
	return code, true
}

func validateCodeRequest(c *gin.Context, req *RedeemCodeRequest) bool {
	if req.Amount.Cmp(decimal.NewFromFloat(0.01)) < 0 {
		badRequest(c, "amount must be greater than or equal to 0.01")
		return false
	}
	if req.Status == "" {
		req.Status = statusAvailable
	}
	if !validStatuses[req.Status] {
		badRequest(c, "status is invalid")
		return false
	}
	return true
}

func (app *App) applyCodeRequest(code *RedeemCode, req RedeemCodeRequest, creating bool) error {
	normalizedCode := strings.TrimSpace(req.Code)
	if creating && normalizedCode == "" {
		generated, err := app.uniqueCode()
		if err != nil {
			return err
		}
		normalizedCode = generated
	}
	if normalizedCode != "" {
		code.Code = normalizedCode
	}
	code.Amount = req.Amount
	code.Status = req.Status
	if code.Status == statusAvailable {
		code.UserID = nil
		code.SignDate = nil
	} else {
		userID := strings.TrimSpace(req.UserID)
		if userID == "" {
			code.UserID = nil
		} else {
			code.UserID = &userID
		}
		code.SignDate = req.SignDate
	}
	return nil
}

func (app *App) uniqueCode() (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		code, err := randomCode()
		if err != nil {
			return "", err
		}
		var count int64
		if err := app.db.Model(&RedeemCode{}).Where("code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("Unable to generate a unique redeem code")
}

func randomCode() (string, error) {
	bytes := make([]byte, 14)
	randomBytes := make([]byte, 14)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	for i, value := range randomBytes {
		bytes[i] = codeAlphabet[int(value)%len(codeAlphabet)]
	}
	return "RC" + string(bytes), nil
}

func parseCodes(text string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, rawCode := range codeSplitPattern.Split(text, -1) {
		code := strings.TrimSpace(rawCode)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	return result
}

func toCheckInResponse(code RedeemCode, alreadyCheckedIn bool, message string) CheckInResponse {
	return CheckInResponse{
		Success:          true,
		AlreadyCheckedIn: alreadyCheckedIn,
		UserID:           code.UserID,
		SignDate:         code.SignDate,
		Code:             code.Code,
		Amount:           code.Amount,
		Message:          message,
	}
}

func (app *App) adminAuth(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Next()
		return
	}
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, APIError{Message: "Missing admin token"})
		return
	}
	if !app.verifyToken(strings.TrimPrefix(auth, "Bearer ")) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, APIError{Message: "Invalid admin token"})
		return
	}
	c.Next()
}

func (app *App) issueToken(username string) string {
	expiresAt := time.Now().Add(time.Duration(app.cfg.AuthTokenTTLHours) * time.Hour).Unix()
	payload := base64.RawURLEncoding.EncodeToString([]byte(username)) + "." + strconv.FormatInt(expiresAt, 10)
	return payload + "." + app.sign(payload)
}

func (app *App) verifyToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(app.sign(payload)), []byte(parts[2])) {
		return false
	}
	expiresAt, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && expiresAt > time.Now().Unix()
}

func (app *App) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(app.cfg.AuthSecret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (app *App) matchesPassword(rawPassword, configuredPassword string) bool {
	if strings.TrimSpace(configuredPassword) == "" {
		return false
	}
	if strings.HasPrefix(configuredPassword, "$2a$") || strings.HasPrefix(configuredPassword, "$2b$") || strings.HasPrefix(configuredPassword, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(configuredPassword), []byte(rawPassword)) == nil
	}
	return rawPassword == configuredPassword
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		badRequest(c, "请求参数无效")
		return false
	}
	return true
}

type BusinessConflict string

func businessConflict(message string) error {
	return BusinessConflict(message)
}

func (e BusinessConflict) Error() string {
	return string(e)
}

func isBusinessConflict(err error) bool {
	var target BusinessConflict
	return errors.As(err, &target)
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, APIError{Message: message})
}

func conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, APIError{Message: message})
}

func serverError(c *gin.Context, err error) {
	log.Printf("server error: %v", err)
	c.JSON(http.StatusInternalServerError, APIError{Message: "服务器内部错误"})
}

func handleDBError(c *gin.Context, err error) {
	if isDuplicateEntry(err) {
		conflict(c, "数据冲突：兑换码或用户签到记录可能已存在")
		return
	}
	serverError(c, err)
}

func isDuplicateEntry(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func loadDotEnv() {
	loaded := map[string]bool{}
	for _, path := range dotenvPaths() {
		cleanPath := filepath.Clean(path)
		if loaded[cleanPath] {
			continue
		}
		loaded[cleanPath] = true
		if err := loadDotEnvFile(cleanPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("load %s: %v", cleanPath, err)
		}
	}
}

func dotenvPaths() []string {
	paths := []string{".env", filepath.Join("backend-go", ".env")}
	if exePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), ".env"))
	}
	return paths
}

func loadDotEnvFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for index, line := range strings.Split(string(content), "\n") {
		key, value, ok, err := parseDotEnvLine(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("line %d: %w", index+1, err)
			}
		}
	}
	return nil
}

func parseDotEnvLine(line string) (key string, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	line = strings.TrimPrefix(line, "export ")
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false, fmt.Errorf("expected KEY=VALUE")
	}
	key = strings.TrimSpace(parts[0])
	if !isDotEnvKey(key) {
		return "", "", false, fmt.Errorf("invalid key %q", key)
	}
	value = strings.TrimSpace(parts[1])
	if len(value) >= 2 {
		quote := value[0]
		if quote == '"' || quote == '\'' {
			if value[len(value)-1] != quote {
				return "", "", false, fmt.Errorf("unterminated quoted value for %s", key)
			}
			value = value[1 : len(value)-1]
			if quote == '"' {
				if unquoted, unquoteErr := strconv.Unquote(`"` + value + `"`); unquoteErr == nil {
					value = unquoted
				}
			}
			return key, value, true, nil
		}
	}
	value = strings.TrimSpace(stripDotEnvComment(value))
	return key, value, true, nil
}

func isDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for index, char := range key {
		valid := char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || index > 0 && char >= '0' && char <= '9'
		if !valid {
			return false
		}
	}
	return true
}

func stripDotEnvComment(value string) string {
	for index, char := range value {
		if char == '#' && (index == 0 || value[index-1] == ' ' || value[index-1] == '\t') {
			return value[:index]
		}
	}
	return value
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, ""))
	if err != nil {
		return fallback
	}
	return value
}

func envInt64(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(env(key, ""), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(env(key, "")))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

package app

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDatabase(cfg Config) (*gorm.DB, error) {
	dsn, err := buildPostgresDSN(cfg)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
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

func buildPostgresDSN(cfg Config) (string, error) {
	if raw := env("DB_DSN", ""); raw != "" {
		return raw, nil
	}

	raw := strings.TrimPrefix(cfg.DBURL, "jdbc:")
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", errors.New("DB_URL must be a postgres/postgresql URL or set DB_DSN")
	}
	if cfg.DBUsername != "" {
		parsed.User = url.UserPassword(cfg.DBUsername, cfg.DBPassword)
	}
	query := parsed.Query()
	if query.Get("sslmode") == "" {
		query.Set("sslmode", "disable")
	}
	if query.Get("TimeZone") == "" && query.Get("timezone") == "" {
		query.Set("TimeZone", "Asia/Shanghai")
	}
	parsed.RawQuery = query.Encode()
	parsed.RawQuery = strings.ReplaceAll(parsed.RawQuery, "TimeZone=Asia%2FShanghai", "TimeZone=Asia/Shanghai")
	parsed.RawQuery = strings.ReplaceAll(parsed.RawQuery, "timezone=Asia%2FShanghai", "timezone=Asia/Shanghai")
	return parsed.String(), nil
}

func (app *App) migrate() error {
	sqlStatements := []string{
		`CREATE TABLE IF NOT EXISTS redeem_codes (
			id BIGSERIAL PRIMARY KEY,
			code VARCHAR(128) NOT NULL,
			user_id VARCHAR(64) NULL,
			sign_date DATE NULL,
			amount DECIMAL(10, 2) NOT NULL,
			status VARCHAR(20) NOT NULL CHECK (status IN ('AVAILABLE','ASSIGNED','USED','VOIDED')),
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_redeem_codes_code ON redeem_codes (code)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_redeem_codes_user_date ON redeem_codes (user_id, sign_date)`,
		`CREATE INDEX IF NOT EXISTS idx_redeem_codes_user_date ON redeem_codes (user_id, sign_date)`,
		`CREATE INDEX IF NOT EXISTS idx_redeem_codes_status_amount ON redeem_codes (status, amount)`,
		`CREATE INDEX IF NOT EXISTS idx_redeem_codes_status ON redeem_codes (status)`,
		`CREATE TABLE IF NOT EXISTS check_in_records (
			id BIGSERIAL PRIMARY KEY,
			user_id VARCHAR(64) NOT NULL,
			sign_date DATE NOT NULL,
			redeem_code_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_check_in_records_user_date ON check_in_records (user_id, sign_date)`,
		`CREATE INDEX IF NOT EXISTS idx_check_in_records_code_id ON check_in_records (redeem_code_id)`,
		`CREATE TABLE IF NOT EXISTS daily_checkin_limits (
			sign_date DATE PRIMARY KEY,
			checked_count INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS system_settings (
			setting_key VARCHAR(100) PRIMARY KEY,
			setting_value TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS favorite_sites (
			id BIGSERIAL PRIMARY KEY,
			icon VARCHAR(500) NOT NULL DEFAULT '',
			url VARCHAR(500) NOT NULL,
			name VARCHAR(100) NOT NULL,
			description VARCHAR(500) NOT NULL DEFAULT '',
			sort_order INT NOT NULL DEFAULT 0,
			group_name VARCHAR(100) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_favorite_sites_url ON favorite_sites (url)`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_sites_group_sort ON favorite_sites (group_name, sort_order)`,
	}

	for _, statement := range sqlStatements {
		if err := app.db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func isDuplicateEntry(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

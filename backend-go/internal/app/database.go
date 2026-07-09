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
		`ALTER TABLE check_in_records ADD COLUMN IF NOT EXISTS check_in_method VARCHAR(20) NOT NULL DEFAULT 'direct'`,
		`ALTER TABLE check_in_records ADD COLUMN IF NOT EXISTS platform_type VARCHAR(40) NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_check_in_records_method_date ON check_in_records (check_in_method, sign_date)`,
		`CREATE TABLE IF NOT EXISTS daily_checkin_limits (
			sign_date DATE PRIMARY KEY,
			checked_count INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS daily_checkin_method_limits (
			sign_date DATE NOT NULL,
			check_in_method VARCHAR(20) NOT NULL,
			checked_count INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (sign_date, check_in_method)
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
		`CREATE TABLE IF NOT EXISTS recharge_activities (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(120) NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			start_at TIMESTAMPTZ NULL,
			end_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_recharge_activities_enabled ON recharge_activities (enabled)`,
		`CREATE TABLE IF NOT EXISTS recharge_reward_tiers (
			id BIGSERIAL PRIMARY KEY,
			activity_id BIGINT NOT NULL,
			threshold_amount DECIMAL(10, 2) NOT NULL,
			reward_amount DECIMAL(10, 2) NOT NULL,
			sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_recharge_reward_tiers_activity ON recharge_reward_tiers (activity_id, sort_order, id)`,
		`CREATE TABLE IF NOT EXISTS recharge_reward_claims (
			id BIGSERIAL PRIMARY KEY,
			activity_id BIGINT NOT NULL,
			tier_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			threshold_amount DECIMAL(10, 2) NOT NULL,
			reward_amount DECIMAL(10, 2) NOT NULL,
			status VARCHAR(20) NOT NULL,
			redeem_code VARCHAR(128) NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_recharge_reward_claim_user_tier ON recharge_reward_claims (activity_id, tier_id, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_recharge_reward_claims_user ON recharge_reward_claims (user_id, activity_id)`,
		`CREATE INDEX IF NOT EXISTS idx_recharge_reward_claims_status ON recharge_reward_claims (status)`,
		`CREATE TABLE IF NOT EXISTS social_account_bindings (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			platform VARCHAR(40) NOT NULL,
			external_user_id VARCHAR(128) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_social_bindings_user_platform ON social_account_bindings (user_id, platform)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_social_bindings_platform_external ON social_account_bindings (platform, external_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_social_bindings_user ON social_account_bindings (user_id)`,
		`CREATE TABLE IF NOT EXISTS sub2api_group_rate_snapshots (
			group_id VARCHAR(100) PRIMARY KEY,
			group_name VARCHAR(200) NOT NULL,
			rate_multiplier DECIMAL(18, 6) NOT NULL,
			raw_json TEXT NOT NULL,
			last_seen_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sub2api_group_rate_snapshots_seen ON sub2api_group_rate_snapshots (last_seen_at DESC)`,
		`CREATE TABLE IF NOT EXISTS sub2api_group_rate_logs (
			id BIGSERIAL PRIMARY KEY,
			group_id VARCHAR(100) NOT NULL,
			group_name VARCHAR(200) NOT NULL,
			old_rate DECIMAL(18, 6) NOT NULL,
			new_rate DECIMAL(18, 6) NOT NULL,
			source VARCHAR(40) NOT NULL,
			public_visible BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sub2api_group_rate_logs_group_time ON sub2api_group_rate_logs (group_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_sub2api_group_rate_logs_public ON sub2api_group_rate_logs (public_visible, created_at)`,
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

func duplicateConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName
	}
	return ""
}

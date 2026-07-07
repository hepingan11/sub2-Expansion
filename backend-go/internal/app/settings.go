package app

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	mathrand "math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

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
	sub2api.AdminPasswordSet = sub2api.AdminPassword != ""
	sub2api.AdminAPIKey = ""
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
		AdminEmail:       app.cfg.Sub2APIAdminEmail,
		AdminPassword:    app.cfg.Sub2APIAdminPassword,
		TimeoutSeconds:   timeoutSeconds,
		AdminAPIKeySet:   app.cfg.Sub2APIAdminAPIKey != "",
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
	return "password"
}

func normalizeSub2APIAuthMode(value string) string {
	switch strings.TrimSpace(value) {
	case "admin_api_key", "password":
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
	tx := app.db.Where("setting_key = ?", key).Limit(1).Find(&setting)
	if tx.Error != nil {
		return "", false, tx.Error
	}
	if tx.RowsAffected == 0 {
		return "", false, nil
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

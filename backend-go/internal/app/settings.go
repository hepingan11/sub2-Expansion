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
	"golang.org/x/crypto/bcrypt"
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
	dailyLimitMode := normalizeDailyLimitMode(req.DailyLimitMode)
	if req.DirectDailyMaxUsers < 0 || req.SocialDailyMaxUsers < 0 {
		badRequest(c, "daily check-in limits must be greater than or equal to 0")
		return
	}
	directInput := req.DirectPrizeTiers
	if len(directInput) == 0 {
		directInput = req.PrizeTiers
	}
	directTiers, err := normalizePrizeTiers(directInput)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	socialInput := req.SocialPrizeTiers
	if len(socialInput) == 0 {
		socialInput = directTiers
	}
	socialTiers, err := normalizePrizeTiers(socialInput)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := app.saveSetting(dailyMaxUsersKey, strconv.Itoa(req.DailyMaxUsers)); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(dailyLimitModeKey, dailyLimitMode); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(directDailyMaxUsersKey, strconv.Itoa(req.DirectDailyMaxUsers)); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(socialDailyMaxUsersKey, strconv.Itoa(req.SocialDailyMaxUsers)); err != nil {
		serverError(c, err)
		return
	}
	if err := app.savePrizeTiers(prizeTiersKey, directTiers); err != nil {
		serverError(c, err)
		return
	}
	if err := app.savePrizeTiers(socialPrizeTiersKey, socialTiers); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(checkInGroupLinkKey, strings.TrimSpace(req.GroupLink)); err != nil {
		serverError(c, err)
		return
	}
	adminConfig := req.Admin
	if strings.TrimSpace(adminConfig.Username) == "" && adminConfig.Password == "" {
		currentAdmin, err := app.effectiveAdminConfig()
		if err != nil {
			serverError(c, err)
			return
		}
		adminConfig.Username = currentAdmin.Username
	}
	if err := app.saveAdminConfig(adminConfig); err != nil {
		if isBusinessConflict(err) {
			conflict(c, err.Error())
			return
		}
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
	dailyLimitMode, err := app.getDailyLimitMode()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	directDailyMaxUsers, err := app.getMethodDailyMaxUsers(directDailyMaxUsersKey, dailyMaxUsers)
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	socialDailyMaxUsers, err := app.getMethodDailyMaxUsers(socialDailyMaxUsersKey, dailyMaxUsers)
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	directTiers, err := app.getPrizeTiers(prizeTiersKey, defaultPrizeTiers)
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	socialTiers, err := app.getPrizeTiers(socialPrizeTiersKey, directTiers)
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	groupLink, err := app.settingOrDefault(checkInGroupLinkKey, "")
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	admin, err := app.effectiveAdminConfig()
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
	admin.PasswordSet = admin.Password != ""
	admin.Password = ""
	return CheckInSettingsResponse{
		DailyMaxUsers:       dailyMaxUsers,
		DailyLimitMode:      dailyLimitMode,
		DirectDailyMaxUsers: directDailyMaxUsers,
		SocialDailyMaxUsers: socialDailyMaxUsers,
		PrizeTiers:          directTiers,
		DirectPrizeTiers:    directTiers,
		SocialPrizeTiers:    socialTiers,
		GroupLink:           groupLink,
		Admin:               admin,
		Sub2API:             sub2api,
	}, nil
}

func (app *App) effectiveAdminConfig() (AdminConfig, error) {
	cfg := AdminConfig{
		Username:    app.cfg.AdminUsername,
		Password:    app.cfg.AdminPassword,
		PasswordSet: strings.TrimSpace(app.cfg.AdminPassword) != "",
	}
	var err error
	if cfg.Username, err = app.settingOrDefault(adminUsernameKey, cfg.Username); err != nil {
		return AdminConfig{}, err
	}
	if cfg.Password, err = app.settingOrDefault(adminPasswordHashKey, cfg.Password); err != nil {
		return AdminConfig{}, err
	}
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.PasswordSet = strings.TrimSpace(cfg.Password) != ""
	return cfg, nil
}

func (app *App) saveAdminConfig(cfg AdminConfig) error {
	username := strings.TrimSpace(cfg.Username)
	if username == "" {
		return businessConflict("admin username must not be blank")
	}
	if err := app.saveSetting(adminUsernameKey, username); err != nil {
		return err
	}
	if cfg.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := app.saveSetting(adminPasswordHashKey, string(hashed)); err != nil {
			return err
		}
	}
	return nil
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

func (app *App) getDailyLimitMode() (string, error) {
	value, found, err := app.getSetting(dailyLimitModeKey)
	if err != nil {
		return "", err
	}
	if !found {
		mode := "shared"
		return mode, app.saveSetting(dailyLimitModeKey, mode)
	}
	mode := normalizeDailyLimitMode(value)
	if mode != strings.TrimSpace(value) {
		return mode, app.saveSetting(dailyLimitModeKey, mode)
	}
	return mode, nil
}

func normalizeDailyLimitMode(value string) string {
	switch strings.TrimSpace(value) {
	case "separate":
		return "separate"
	default:
		return "shared"
	}
}

func (app *App) getMethodDailyMaxUsers(key string, fallback int) (int, error) {
	value, found, err := app.getSetting(key)
	if err != nil {
		return 0, err
	}
	if !found {
		return fallback, app.saveSetting(key, strconv.Itoa(fallback))
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback, app.saveSetting(key, strconv.Itoa(fallback))
	}
	return parsed, nil
}

func (app *App) getPrizeTiers(key string, fallback []PrizeTier) ([]PrizeTier, error) {
	value, found, err := app.getSetting(key)
	if err != nil {
		return nil, err
	}
	if !found {
		return fallback, app.savePrizeTiers(key, fallback)
	}
	var tiers []PrizeTier
	if err := json.Unmarshal([]byte(value), &tiers); err != nil {
		return fallback, app.savePrizeTiers(key, fallback)
	}
	normalized, err := normalizePrizeTiers(tiers)
	if err != nil {
		return fallback, app.savePrizeTiers(key, fallback)
	}
	return normalized, nil
}

func (app *App) drawAmount(checkInMethod string) (Amount, error) {
	key := prizeTiersKey
	fallback := defaultPrizeTiers
	if checkInMethod == checkInMethodSocial {
		key = socialPrizeTiersKey
		if directTiers, err := app.getPrizeTiers(prizeTiersKey, defaultPrizeTiers); err == nil {
			fallback = directTiers
		}
	}
	tiers, err := app.getPrizeTiers(key, fallback)
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

func (app *App) savePrizeTiers(key string, tiers []PrizeTier) error {
	encoded, err := json.Marshal(tiers)
	if err != nil {
		return err
	}
	return app.saveSetting(key, string(encoded))
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

package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	invitation, err := normalizeInvitationConfig(req.Invitation)
	if err != nil {
		badRequest(c, err.Error())
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
	if err := app.saveSetting(frontendPublicURLKey, normalizeFrontendPublicURL(req.FrontendPublicURL)); err != nil {
		serverError(c, err)
		return
	}
	if req.TokenUsageRankingEnabled != nil {
		if err := app.saveSetting(tokenUsageRankingKey, strconv.FormatBool(*req.TokenUsageRankingEnabled)); err != nil {
			serverError(c, err)
			return
		}
	}
	if err := app.saveSetting(invitationAfterTimeKey, invitation.AfterTime); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(invitationAmountKey, invitation.Amount.StringFixed(2)); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveInvitationGuideConfig(req.InvitationGuide); err != nil {
		badRequest(c, err.Error())
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
	if err := app.saveTelegramConfig(req.Telegram); err != nil {
		serverError(c, err)
		return
	}
	app.restartTelegramBot(context.Background())
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
	frontendPublicURL, err := app.frontendPublicURL()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	tokenUsageRankingEnabled, err := app.tokenUsageRankingEnabled()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	invitation, err := app.loadInvitationConfig()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	invitationGuide, err := app.loadInvitationGuideConfig()
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
	telegram, err := app.safeTelegramConfigForAdmin()
	if err != nil {
		return CheckInSettingsResponse{}, err
	}
	return CheckInSettingsResponse{
		DailyMaxUsers:            dailyMaxUsers,
		DailyLimitMode:           dailyLimitMode,
		DirectDailyMaxUsers:      directDailyMaxUsers,
		SocialDailyMaxUsers:      socialDailyMaxUsers,
		PrizeTiers:               directTiers,
		DirectPrizeTiers:         directTiers,
		SocialPrizeTiers:         socialTiers,
		GroupLink:                groupLink,
		FrontendPublicURL:        frontendPublicURL,
		TokenUsageRankingEnabled: tokenUsageRankingEnabled,
		Admin:                    admin,
		Sub2API:                  sub2api,
		Invitation:               invitation,
		InvitationGuide:          invitationGuide,
		Telegram:                 telegram,
	}, nil
}

func (app *App) loadInvitationGuideConfig() (InvitationGuideConfig, error) {
	qqGroupNumber, err := app.settingOrDefault(invitationQQGroupKey, "799128896")
	if err != nil {
		return InvitationGuideConfig{}, err
	}
	qqBotMention, err := app.settingOrDefault(invitationQQBotKey, "@咕咕嘎嘎")
	if err != nil {
		return InvitationGuideConfig{}, err
	}
	return InvitationGuideConfig{
		QQGroupNumber: strings.TrimSpace(qqGroupNumber),
		QQBotMention:  strings.TrimSpace(qqBotMention),
	}, nil
}

func (app *App) saveInvitationGuideConfig(config InvitationGuideConfig) error {
	config.QQGroupNumber = strings.TrimSpace(config.QQGroupNumber)
	config.QQBotMention = strings.TrimSpace(config.QQBotMention)
	if config.QQGroupNumber == "" && config.QQBotMention == "" {
		current, err := app.loadInvitationGuideConfig()
		if err != nil {
			return err
		}
		config = current
	}
	if config.QQGroupNumber == "" {
		return errors.New("QQ 邀请教程群号不能为空")
	}
	if config.QQBotMention == "" {
		return errors.New("QQ 邀请教程机器人称呼不能为空")
	}
	if utf8.RuneCountInString(config.QQGroupNumber) > 100 || utf8.RuneCountInString(config.QQBotMention) > 100 {
		return errors.New("QQ 邀请教程配置不能超过 100 个字符")
	}
	if err := app.saveSetting(invitationQQGroupKey, config.QQGroupNumber); err != nil {
		return err
	}
	return app.saveSetting(invitationQQBotKey, config.QQBotMention)
}

func normalizeInvitationConfig(input InvitationConfig) (InvitationConfig, error) {
	input.AfterTime = strings.TrimSpace(input.AfterTime)
	input.Amount = Amount{input.Amount.Round(2)}
	if input.Amount.Cmp(decimal.Zero) < 0 {
		return InvitationConfig{}, errors.New("邀请奖励金额不能小于 0")
	}
	if input.AfterTime == "" {
		if input.Amount.Cmp(decimal.Zero) > 0 {
			return InvitationConfig{}, errors.New("设置邀请奖励金额时必须同时设置新人账号时间门槛")
		}
		return InvitationConfig{Amount: MustAmount("0.00")}, nil
	}
	parsed, err := time.Parse(time.RFC3339, input.AfterTime)
	if err != nil {
		return InvitationConfig{}, errors.New("新人账号时间门槛必须是有效的 RFC3339 时间")
	}
	if input.Amount.Cmp(decimal.Zero) <= 0 {
		return InvitationConfig{}, errors.New("启用邀请功能时奖励金额必须大于 0")
	}
	input.AfterTime = parsed.UTC().Format(time.RFC3339)
	return input, nil
}

func (app *App) loadInvitationConfig() (InvitationConfig, error) {
	afterTime, err := app.settingOrDefault(invitationAfterTimeKey, "")
	if err != nil {
		return InvitationConfig{}, err
	}
	amountText, err := app.settingOrDefault(invitationAmountKey, "0.00")
	if err != nil {
		return InvitationConfig{}, err
	}
	amount, err := ParseAmount(amountText)
	if err != nil {
		amount = MustAmount("0.00")
	}
	config, err := normalizeInvitationConfig(InvitationConfig{AfterTime: afterTime, Amount: amount})
	if err != nil {
		return InvitationConfig{Amount: MustAmount("0.00")}, nil
	}
	return config, nil
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

func (app *App) effectiveTelegramConfig() (TelegramConfig, error) {
	pollInterval := int(app.cfg.TelegramBotPollEvery / time.Second)
	if pollInterval <= 0 {
		pollInterval = 2
	}
	cfg := TelegramConfig{
		Enabled:                app.cfg.TelegramBotEnabled,
		BotToken:               strings.TrimSpace(app.cfg.TelegramBotToken),
		BotTokenSet:            strings.TrimSpace(app.cfg.TelegramBotToken) != "",
		APIBaseURL:             strings.TrimRight(strings.TrimSpace(app.cfg.TelegramBotAPIBaseURL), "/"),
		PollIntervalSeconds:    pollInterval,
		BindingTokenTTLMinutes: 10,
	}
	var err error
	if enabled, found, err := app.getSetting(telegramEnabledKey); err != nil {
		return TelegramConfig{}, err
	} else if found {
		cfg.Enabled = parseBoolSetting(enabled, cfg.Enabled)
	}
	if cfg.BotToken, err = app.settingOrDefault(telegramBotTokenKey, cfg.BotToken); err != nil {
		return TelegramConfig{}, err
	}
	if cfg.APIBaseURL, err = app.settingOrDefault(telegramBotAPIBaseURLKey, cfg.APIBaseURL); err != nil {
		return TelegramConfig{}, err
	}
	if interval, found, err := app.getSetting(telegramBotPollIntervalSeconds); err != nil {
		return TelegramConfig{}, err
	} else if found {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(interval)); parseErr == nil && parsed > 0 {
			cfg.PollIntervalSeconds = parsed
		}
	}
	if enabled, found, err := app.getSetting(telegramMembershipCheckKey); err != nil {
		return TelegramConfig{}, err
	} else if found {
		cfg.MembershipCheckEnabled = parseBoolSetting(enabled, false)
	}
	if cfg.RequiredGroupChatID, err = app.settingOrDefault(telegramRequiredGroupChatIDKey, ""); err != nil {
		return TelegramConfig{}, err
	}
	if cfg.GroupJoinURL, err = app.settingOrDefault(telegramGroupJoinURLKey, ""); err != nil {
		return TelegramConfig{}, err
	}
	if cfg.BotUsername, err = app.settingOrDefault(telegramBotUsernameKey, ""); err != nil {
		return TelegramConfig{}, err
	}
	if ttl, found, err := app.getSetting(telegramBindingTokenTTLKey); err != nil {
		return TelegramConfig{}, err
	} else if found {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(ttl)); parseErr == nil && parsed > 0 {
			cfg.BindingTokenTTLMinutes = parsed
		}
	}
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	cfg.BotTokenSet = cfg.BotToken != ""
	cfg.APIBaseURL = strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/")
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = "https://api.telegram.org"
	}
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 2
	}
	cfg.RequiredGroupChatID = strings.TrimSpace(cfg.RequiredGroupChatID)
	cfg.GroupJoinURL = strings.TrimSpace(cfg.GroupJoinURL)
	cfg.BotUsername = strings.TrimPrefix(strings.TrimSpace(cfg.BotUsername), "@")
	if cfg.BindingTokenTTLMinutes <= 0 {
		cfg.BindingTokenTTLMinutes = 10
	}
	cfg.Connected = cfg.Enabled && cfg.BotTokenSet
	return cfg, nil
}

func (app *App) safeTelegramConfigForAdmin() (TelegramConfig, error) {
	cfg, err := app.effectiveTelegramConfig()
	if err != nil {
		return TelegramConfig{}, err
	}
	cfg.BotToken = ""
	return cfg, nil
}

func (app *App) saveTelegramConfig(cfg TelegramConfig) error {
	if cfg.BindingTokenTTLMinutes == 0 {
		cfg.BindingTokenTTLMinutes = 10
	}
	if cfg.MembershipCheckEnabled && strings.TrimSpace(cfg.RequiredGroupChatID) == "" {
		return errors.New("启用 Telegram 入群校验前请填写目标群 Chat ID")
	}
	if cfg.MembershipCheckEnabled && strings.TrimSpace(cfg.GroupJoinURL) == "" {
		return errors.New("启用 Telegram 入群校验前请填写加群链接")
	}
	if cfg.MembershipCheckEnabled {
		groupJoinURL, err := url.ParseRequestURI(strings.TrimSpace(cfg.GroupJoinURL))
		if err != nil || groupJoinURL.Host == "" || groupJoinURL.Scheme != "https" && groupJoinURL.Scheme != "http" {
			return errors.New("Telegram 加群链接必须是有效的 HTTP 或 HTTPS 地址")
		}
	}
	if cfg.BindingTokenTTLMinutes < 1 || cfg.BindingTokenTTLMinutes > 1440 {
		return errors.New("Telegram 绑定凭证有效期必须在 1 到 1440 分钟之间")
	}
	values := map[string]string{
		telegramEnabledKey:             strconv.FormatBool(cfg.Enabled),
		telegramBotAPIBaseURLKey:       strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/"),
		telegramBotPollIntervalSeconds: strconv.Itoa(max(cfg.PollIntervalSeconds, 1)),
		telegramMembershipCheckKey:     strconv.FormatBool(cfg.MembershipCheckEnabled),
		telegramRequiredGroupChatIDKey: strings.TrimSpace(cfg.RequiredGroupChatID),
		telegramGroupJoinURLKey:        strings.TrimSpace(cfg.GroupJoinURL),
		telegramBindingTokenTTLKey:     strconv.Itoa(cfg.BindingTokenTTLMinutes),
	}
	if values[telegramBotAPIBaseURLKey] == "" {
		values[telegramBotAPIBaseURLKey] = "https://api.telegram.org"
	}
	for key, value := range values {
		if err := app.saveSetting(key, value); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.BotToken) != "" {
		if err := app.saveSetting(telegramBotTokenKey, strings.TrimSpace(cfg.BotToken)); err != nil {
			return err
		}
	}
	return nil
}

func parseBoolSetting(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
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

func (app *App) frontendPublicURL() (string, error) {
	return app.settingOrDefault(frontendPublicURLKey, normalizeFrontendPublicURL(app.cfg.FrontendPublicURL))
}

func normalizeFrontendPublicURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
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
	normalized, err := parsePrizeTiers(value)
	if err != nil {
		return fallback, app.savePrizeTiers(key, fallback)
	}
	return normalized, nil
}

func (app *App) drawAmount(checkInMethod, platformType string) (Amount, error) {
	key := prizeTiersKey
	fallback := defaultPrizeTiers
	if checkInMethod == checkInMethodSocial {
		if strings.TrimSpace(platformType) != "" {
			platformConfig, _, err := app.platformCheckInConfig(platformType)
			if err != nil {
				return Amount{}, err
			}
			return drawAmountFromTiers(platformConfig.PrizeTiers)
		}
		key = socialPrizeTiersKey
		if directTiers, err := app.getPrizeTiers(prizeTiersKey, defaultPrizeTiers); err == nil {
			fallback = directTiers
		}
	}
	tiers, err := app.getPrizeTiers(key, fallback)
	if err != nil {
		return Amount{}, err
	}
	return drawAmountFromTiers(tiers)
}

func drawAmountFromTiers(tiers []PrizeTier) (Amount, error) {
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

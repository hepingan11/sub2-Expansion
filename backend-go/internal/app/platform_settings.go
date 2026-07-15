package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type PlatformSettingsResponse struct {
	Platform     string           `json:"platform"`
	CheckIn      PlatformCheckIn  `json:"checkIn"`
	Invitation   InvitationConfig `json:"invitation"`
	Effective    bool             `json:"effective"`
	FallbackFrom string           `json:"fallbackFrom,omitempty"`
}

type PlatformSettingsRequest struct {
	CheckIn    PlatformCheckIn  `json:"checkIn"`
	Invitation InvitationConfig `json:"invitation"`
}

type PlatformCheckIn struct {
	DailyMaxUsers int         `json:"dailyMaxUsers"`
	PrizeTiers    []PrizeTier `json:"prizeTiers"`
	GroupLink     string      `json:"groupLink"`
}

func (app *App) getPlatformSettings(c *gin.Context) {
	platform, err := normalizePlatformName(c.Param("platform"))
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	settings, err := app.loadPlatformSettings(platform)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (app *App) updatePlatformSettings(c *gin.Context) {
	platform, err := normalizePlatformName(c.Param("platform"))
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	var req PlatformSettingsRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.CheckIn.DailyMaxUsers < 0 {
		badRequest(c, "daily check-in limit must be greater than or equal to 0")
		return
	}
	tiers, err := normalizePrizeTiers(req.CheckIn.PrizeTiers)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	invitation, err := normalizeInvitationConfig(req.Invitation)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := app.saveSetting(platformSettingKey(platform, "check_in.daily_max_users"), strconv.Itoa(req.CheckIn.DailyMaxUsers)); err != nil {
		serverError(c, err)
		return
	}
	if err := app.savePrizeTiers(platformSettingKey(platform, "check_in.prize_tiers"), tiers); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(platformSettingKey(platform, "check_in.group_link"), strings.TrimSpace(req.CheckIn.GroupLink)); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(platformSettingKey(platform, "invitation.after_time"), invitation.AfterTime); err != nil {
		serverError(c, err)
		return
	}
	if err := app.saveSetting(platformSettingKey(platform, "invitation.amount"), invitation.Amount.StringFixed(2)); err != nil {
		serverError(c, err)
		return
	}
	settings, err := app.loadPlatformSettings(platform)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (app *App) loadPlatformSettings(platform string) (PlatformSettingsResponse, error) {
	platform, err := normalizePlatformName(platform)
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	sharedDaily, err := app.getDailyMaxUsers()
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	fallbackDaily, err := app.getMethodDailyMaxUsers(socialDailyMaxUsersKey, sharedDaily)
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	directTiers, err := app.getPrizeTiers(prizeTiersKey, defaultPrizeTiers)
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	fallbackTiers, err := app.getPrizeTiers(socialPrizeTiersKey, directTiers)
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	fallbackGroupLink, err := app.settingOrDefault(checkInGroupLinkKey, "")
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	fallbackInvitation, err := app.loadInvitationConfig()
	if err != nil {
		return PlatformSettingsResponse{}, err
	}

	checkIn, checkInOverridden, err := app.loadPlatformCheckIn(platform, PlatformCheckIn{
		DailyMaxUsers: fallbackDaily,
		PrizeTiers:    fallbackTiers,
		GroupLink:     fallbackGroupLink,
	})
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	invitation, invitationOverridden, err := app.loadPlatformInvitationConfig(platform, fallbackInvitation)
	if err != nil {
		return PlatformSettingsResponse{}, err
	}
	resp := PlatformSettingsResponse{
		Platform:     platform,
		CheckIn:      checkIn,
		Invitation:   invitation,
		Effective:    checkInOverridden || invitationOverridden,
		FallbackFrom: "social",
	}
	if resp.Effective {
		resp.FallbackFrom = ""
	}
	return resp, nil
}

func (app *App) platformCheckInConfig(platform string) (PlatformCheckIn, bool, error) {
	settings, err := app.loadPlatformSettings(platform)
	if err != nil {
		return PlatformCheckIn{}, false, err
	}
	return settings.CheckIn, settings.Effective, nil
}

func (app *App) invitationConfigForPlatform(platform string) (InvitationConfig, error) {
	fallback, err := app.loadInvitationConfig()
	if err != nil {
		return InvitationConfig{}, err
	}
	if strings.TrimSpace(platform) == "" {
		return fallback, nil
	}
	platform, err = normalizePlatformName(platform)
	if err != nil {
		return InvitationConfig{}, err
	}
	config, _, err := app.loadPlatformInvitationConfig(platform, fallback)
	return config, err
}

func (app *App) loadPlatformCheckIn(platform string, fallback PlatformCheckIn) (PlatformCheckIn, bool, error) {
	keyPrefix := platformSettingKey(platform, "check_in.")
	config := fallback
	overridden := false
	if value, found, err := app.getSetting(keyPrefix + "daily_max_users"); err != nil {
		return PlatformCheckIn{}, false, err
	} else if found {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(value))
		if parseErr == nil && parsed >= 0 {
			config.DailyMaxUsers = parsed
			overridden = true
		}
	}
	if value, found, err := app.getSetting(keyPrefix + "group_link"); err != nil {
		return PlatformCheckIn{}, false, err
	} else if found {
		config.GroupLink = strings.TrimSpace(value)
		overridden = true
	}
	if value, found, err := app.getSetting(keyPrefix + "prize_tiers"); err != nil {
		return PlatformCheckIn{}, false, err
	} else if found {
		tiers, tierErr := parsePrizeTiers(value)
		if tierErr != nil {
			return PlatformCheckIn{}, false, tierErr
		}
		config.PrizeTiers = tiers
		overridden = true
	}
	return config, overridden, nil
}

func (app *App) loadPlatformInvitationConfig(platform string, fallback InvitationConfig) (InvitationConfig, bool, error) {
	afterTime, afterFound, err := app.getSetting(platformSettingKey(platform, "invitation.after_time"))
	if err != nil {
		return InvitationConfig{}, false, err
	}
	amountText, amountFound, err := app.getSetting(platformSettingKey(platform, "invitation.amount"))
	if err != nil {
		return InvitationConfig{}, false, err
	}
	if !afterFound && !amountFound {
		return fallback, false, nil
	}
	if !afterFound {
		afterTime = fallback.AfterTime
	}
	amount := fallback.Amount
	if amountFound {
		parsed, parseErr := ParseAmount(amountText)
		if parseErr != nil {
			return InvitationConfig{}, true, parseErr
		}
		amount = parsed
	}
	config, err := normalizeInvitationConfig(InvitationConfig{AfterTime: afterTime, Amount: amount})
	return config, true, err
}

func normalizePlatformName(platform string) (string, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return "", errors.New("platform is required")
	}
	if len(platform) > 40 {
		return "", errors.New("platform is too long")
	}
	for _, ch := range platform {
		valid := ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-'
		if !valid {
			return "", errors.New("platform may only contain letters, numbers, underscore, or dash")
		}
	}
	return platform, nil
}

func platformSettingKey(platform, suffix string) string {
	return platformSettingsPrefix + platform + "." + suffix
}

func platformMethodKey(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return checkInMethodSocial
	}
	value := checkInMethodSocial + ":" + platform
	if len(value) > 20 {
		return value[:20]
	}
	return value
}

func parsePrizeTiers(value string) ([]PrizeTier, error) {
	var tiers []PrizeTier
	if err := json.Unmarshal([]byte(value), &tiers); err != nil {
		return nil, err
	}
	return normalizePrizeTiers(tiers)
}

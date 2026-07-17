package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SocialBindingRequest struct {
	Platform     string `json:"platform"`
	UserID       string `json:"userId"`
	InviteCode   string `json:"inviteCode"`
	BindingToken string `json:"bindingToken"`
}

type SocialBindingResponse struct {
	ID             uint64                   `json:"id"`
	Platform       string                   `json:"platform"`
	ExternalUserID string                   `json:"externalUserId"`
	Bound          bool                     `json:"bound"`
	AlreadyBound   bool                     `json:"alreadyBound"`
	Message        string                   `json:"message"`
	Invitation     *InvitationBindingResult `json:"invitation,omitempty"`
}

func (app *App) bindSocialAccount(c *gin.Context) {
	user, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}

	var req SocialBindingRequest
	if !bindJSON(c, &req) {
		return
	}
	platform, externalUserID, err := normalizeSocialBinding(req)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if platform == telegramPlatform {
		cfg, err := app.effectiveTelegramConfig()
		if err != nil {
			serverError(c, err)
			return
		}
		if cfg.MembershipCheckEnabled {
			if err := app.verifyTelegramBindingToken(strings.TrimSpace(req.BindingToken), externalUserID, req.InviteCode); err != nil {
				conflict(c, err.Error())
				return
			}
			telegramUserID, err := strconv.ParseInt(externalUserID, 10, 64)
			if err != nil {
				badRequest(c, "Telegram userId 格式无效")
				return
			}
			member, err := app.telegramGetChatMember(c.Request.Context(), cfg, telegramUserID)
			if err != nil {
				c.JSON(http.StatusBadGateway, APIError{Message: "Telegram 群成员校验失败：" + err.Error()})
				return
			}
			if !telegramMemberIsActive(member) {
				conflict(c, "请先加入指定 Telegram 群组，再从 Bot 重新获取绑定链接")
				return
			}
		}
	}

	resp, err := app.bindSocialAccountForUser(user.ID, platform, externalUserID)
	if err != nil {
		if isBusinessConflict(err) {
			conflict(c, err.Error())
			return
		}
		serverError(c, err)
		return
	}
	inviteCode := strings.TrimSpace(req.InviteCode)
	if inviteCode != "" {
		if resp.ExternalUserID != externalUserID {
			conflict(c, "当前账号已绑定该平台的其他用户，不能完成邀请绑定")
			return
		}
		invitation, err := app.bindInvitation(c.Request.Context(), user, inviteCode, platform, externalUserID)
		if err != nil {
			if isBusinessConflict(err) {
				conflict(c, err.Error())
				return
			}
			respondSub2APIError(c, err)
			return
		}
		resp.Invitation = &invitation
	}
	c.JSON(http.StatusOK, resp)
}

func (app *App) listSocialBindingsForUser(userID int64) ([]SocialAccountBinding, error) {
	var bindings []SocialAccountBinding
	if err := app.db.
		Where("user_id = ?", userID).
		Order("platform ASC, id ASC").
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

func (app *App) bindSocialAccountForUser(userID int64, platform, externalUserID string) (SocialBindingResponse, error) {
	var binding SocialAccountBinding
	err := app.db.Where("platform = ? AND external_user_id = ?", platform, externalUserID).First(&binding).Error
	if err == nil {
		if binding.UserID != userID {
			return SocialBindingResponse{}, businessConflict("social account is already bound to another user")
		}
		return SocialBindingResponse{
			ID:             binding.ID,
			Platform:       binding.Platform,
			ExternalUserID: binding.ExternalUserID,
			Bound:          false,
			AlreadyBound:   true,
			Message:        "social account already bound to this user",
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return SocialBindingResponse{}, err
	}

	err = app.db.Where("user_id = ? AND platform = ?", userID, platform).First(&binding).Error
	if err == nil {
		return SocialBindingResponse{
			ID:             binding.ID,
			Platform:       binding.Platform,
			ExternalUserID: binding.ExternalUserID,
			Bound:          false,
			AlreadyBound:   true,
			Message:        "account already has a binding for this platform",
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return SocialBindingResponse{}, err
	}

	binding = SocialAccountBinding{
		UserID:         userID,
		Platform:       platform,
		ExternalUserID: externalUserID,
	}
	if err := app.db.Create(&binding).Error; err != nil {
		switch duplicateConstraintName(err) {
		case "uk_social_bindings_platform_external":
			return SocialBindingResponse{}, businessConflict("social account is already bound to another user")
		case "uk_social_bindings_user_platform":
			return SocialBindingResponse{}, businessConflict("social account binding already exists")
		default:
			if isDuplicateEntry(err) {
				return SocialBindingResponse{}, businessConflict("social account binding already exists")
			}
		}
		return SocialBindingResponse{}, err
	}
	return SocialBindingResponse{
		ID:             binding.ID,
		Platform:       binding.Platform,
		ExternalUserID: binding.ExternalUserID,
		Bound:          true,
		AlreadyBound:   false,
		Message:        "social account bound",
	}, nil
}

func normalizeSocialBinding(req SocialBindingRequest) (string, string, error) {
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	externalUserID := strings.TrimSpace(req.UserID)
	if platform == "" {
		return "", "", errors.New("platform is required")
	}
	if externalUserID == "" {
		return "", "", errors.New("userId is required")
	}
	if len(platform) > 40 {
		return "", "", errors.New("platform is too long")
	}
	if len(externalUserID) > 128 {
		return "", "", errors.New("userId is too long")
	}
	for _, ch := range platform {
		valid := ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-'
		if !valid {
			return "", "", errors.New("platform may only contain letters, numbers, underscore, or dash")
		}
	}
	return platform, externalUserID, nil
}

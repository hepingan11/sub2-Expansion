package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SocialBindingRequest struct {
	Platform string `json:"platform"`
	UserID   string `json:"userId"`
}

type SocialBindingResponse struct {
	ID             uint64 `json:"id"`
	Platform       string `json:"platform"`
	ExternalUserID string `json:"externalUserId"`
	Bound          bool   `json:"bound"`
	AlreadyBound   bool   `json:"alreadyBound"`
	Message        string `json:"message"`
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

	resp, err := app.bindSocialAccountForUser(user.ID, platform, externalUserID)
	if err != nil {
		if isBusinessConflict(err) {
			conflict(c, err.Error())
			return
		}
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (app *App) bindSocialAccountForUser(userID int64, platform, externalUserID string) (SocialBindingResponse, error) {
	var binding SocialAccountBinding
	err := app.db.Where("user_id = ? AND platform = ?", userID, platform).First(&binding).Error
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

	err = app.db.Where("platform = ? AND external_user_id = ?", platform, externalUserID).First(&binding).Error
	if err == nil {
		return SocialBindingResponse{}, businessConflict("social account is already bound to another user")
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
		if isDuplicateEntry(err) {
			return SocialBindingResponse{}, businessConflict("social account binding already exists")
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

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const invitationCodeLength = 8

type UserInvitationResponse struct {
	Code              string    `json:"code"`
	SuccessfulInvites int64     `json:"successfulInvites"`
	TotalReward       Amount    `json:"totalReward"`
	RewardAmount      Amount    `json:"rewardAmount"`
	AfterTime         string    `json:"afterTime"`
	Enabled           bool      `json:"enabled"`
	InvitedByCode     string    `json:"invitedByCode,omitempty"`
	InvitedAt         *JSONTime `json:"invitedAt,omitempty"`
}

type InvitationBindingResult struct {
	Bound        bool   `json:"bound"`
	AlreadyBound bool   `json:"alreadyBound"`
	InviteCode   string `json:"inviteCode"`
	RewardAmount Amount `json:"rewardAmount"`
	Message      string `json:"message"`
}

func (app *App) listAdminInvitations(c *gin.Context) {
	page := max(queryInt(c, "page", 0), 0)
	size := min(max(queryInt(c, "size", 20), 1), 100)
	query := app.db.Model(&InvitationBinding{})

	if status := strings.ToUpper(strings.TrimSpace(c.Query("status"))); status != "" {
		switch status {
		case inviteRewardPending, inviteRewarded, inviteRewardFailed:
			query = query.Where("reward_status = ?", status)
		default:
			badRequest(c, "invalid invitation status")
			return
		}
	}
	if platform := strings.ToLower(strings.TrimSpace(c.Query("platform"))); platform != "" {
		query = query.Where("platform = ?", platform)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		if userID, err := strconv.ParseInt(keyword, 10, 64); err == nil && userID > 0 {
			query = query.Where(
				"inviter_user_id = ? OR invitee_user_id = ? OR invite_code ILIKE ? OR external_user_id ILIKE ? OR platform ILIKE ?",
				userID, userID, like, like, like,
			)
		} else {
			query = query.Where("invite_code ILIKE ? OR external_user_id ILIKE ? OR platform ILIKE ?", like, like, like)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		serverError(c, err)
		return
	}
	var records []InvitationBinding
	if err := query.Order("updated_at DESC, id DESC").Limit(size).Offset(page * size).Find(&records).Error; err != nil {
		serverError(c, err)
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}
	c.JSON(http.StatusOK, PageResponse[InvitationBinding]{
		Content:       records,
		TotalElements: total,
		TotalPages:    totalPages,
		Number:        page,
		Size:          size,
	})
}

func (app *App) getAdminInvitationStats(c *gin.Context) {
	today := Today()
	start := LocalDate{Time: today.AddDate(0, 0, -29)}
	tomorrow := today.AddDate(0, 0, 1)

	type dailyRow struct {
		RewardDate LocalDate `gorm:"column:reward_date"`
		Amount     Amount    `gorm:"column:amount"`
		Users      int64     `gorm:"column:users"`
	}
	rows := []dailyRow{}
	if err := app.db.Model(&InvitationBinding{}).
		Select("rewarded_at::date AS reward_date, COALESCE(SUM(reward_amount), 0) AS amount, COUNT(*) AS users").
		Where("reward_status = ? AND rewarded_at >= ? AND rewarded_at < ?", inviteRewarded, start.Time, tomorrow).
		Group("rewarded_at::date").
		Order("reward_date ASC").
		Scan(&rows).Error; err != nil {
		serverError(c, err)
		return
	}

	byDate := make(map[string]dailyRow, len(rows))
	for _, row := range rows {
		byDate[row.RewardDate.Format("2006-01-02")] = row
	}
	daily := make([]DailyInvitationStat, 0, 30)
	for day := start.Time; !day.After(today.Time); day = day.AddDate(0, 0, 1) {
		date := LocalDate{Time: day}
		row, ok := byDate[date.Format("2006-01-02")]
		if !ok {
			row = dailyRow{Amount: Amount{Decimal: decimal.Zero}}
		}
		daily = append(daily, DailyInvitationStat{RewardDate: date, Amount: row.Amount, Users: row.Users})
	}

	c.JSON(http.StatusOK, InvitationStatsResponse{Daily: daily})
}

func (app *App) getUserInvitation(c *gin.Context) {
	user, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}
	response, err := app.userInvitationResponse(user.ID)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) generateUserInvitationCode(c *gin.Context) {
	user, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}
	if _, err := app.ensureInvitationCode(user.ID); err != nil {
		serverError(c, err)
		return
	}
	response, err := app.userInvitationResponse(user.ID)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) userInvitationResponse(userID int64) (UserInvitationResponse, error) {
	config, err := app.loadInvitationConfig()
	if err != nil {
		return UserInvitationResponse{}, err
	}
	response := UserInvitationResponse{
		RewardAmount: config.Amount,
		AfterTime:    config.AfterTime,
		Enabled:      config.AfterTime != "" && config.Amount.Cmp(decimal.Zero) > 0,
		TotalReward:  MustAmount("0.00"),
	}
	var code InvitationCode
	if err := app.db.Where("user_id = ?", userID).First(&code).Error; err == nil {
		response.Code = code.Code
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return UserInvitationResponse{}, err
	}
	type rewardSummary struct {
		Count int64
		Total decimal.Decimal
	}
	var summary rewardSummary
	if err := app.db.Model(&InvitationBinding{}).
		Select("COUNT(*) AS count, COALESCE(SUM(reward_amount), 0) AS total").
		Where("inviter_user_id = ? AND reward_status = ?", userID, inviteRewarded).
		Scan(&summary).Error; err != nil {
		return UserInvitationResponse{}, err
	}
	response.SuccessfulInvites = summary.Count
	response.TotalReward = Amount{summary.Total}
	var binding InvitationBinding
	if err := app.db.Where("invitee_user_id = ? AND reward_status = ?", userID, inviteRewarded).First(&binding).Error; err == nil {
		response.InvitedByCode = binding.InviteCode
		response.InvitedAt = &binding.CreatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return UserInvitationResponse{}, err
	}
	return response, nil
}

func (app *App) ensureInvitationCode(userID int64) (InvitationCode, error) {
	var existing InvitationCode
	if err := app.db.Where("user_id = ?", userID).First(&existing).Error; err == nil {
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return InvitationCode{}, err
	}
	for attempt := 0; attempt < 12; attempt++ {
		code, err := randomInvitationCode()
		if err != nil {
			return InvitationCode{}, err
		}
		candidate := InvitationCode{UserID: userID, Code: code}
		if err := app.db.Create(&candidate).Error; err == nil {
			return candidate, nil
		} else if isDuplicateEntry(err) {
			if lookupErr := app.db.Where("user_id = ?", userID).First(&existing).Error; lookupErr == nil {
				return existing, nil
			}
			continue
		} else {
			return InvitationCode{}, err
		}
	}
	return InvitationCode{}, errors.New("failed to generate a unique invitation code")
}

func randomInvitationCode() (string, error) {
	result := make([]byte, invitationCodeLength)
	limit := big.NewInt(int64(len(codeAlphabet)))
	for i := range result {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		result[i] = codeAlphabet[index.Int64()]
	}
	return string(result), nil
}

func normalizeInvitationCode(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != invitationCodeLength {
		return "", errors.New("邀请码格式无效")
	}
	for _, ch := range value {
		if !strings.ContainsRune(string(codeAlphabet), ch) {
			return "", errors.New("邀请码格式无效")
		}
	}
	return value, nil
}

func (app *App) bindInvitation(ctx context.Context, invitee sub2APIUserSnapshot, inviteCode, platform, externalUserID string) (InvitationBindingResult, error) {
	codeText, err := normalizeInvitationCode(inviteCode)
	if err != nil {
		return InvitationBindingResult{}, businessConflict(err.Error())
	}
	config, err := app.invitationConfigForPlatform(platform)
	if err != nil {
		return InvitationBindingResult{}, err
	}
	if config.AfterTime == "" || config.Amount.Cmp(decimal.Zero) <= 0 {
		return InvitationBindingResult{}, businessConflict("邀请活动尚未启用")
	}
	afterTime, _ := time.Parse(time.RFC3339, config.AfterTime)
	if invitee.CreatedAt.IsZero() {
		return InvitationBindingResult{}, businessConflict("无法读取新人账号创建时间")
	}
	if !invitee.CreatedAt.After(afterTime) {
		return InvitationBindingResult{}, businessConflict("该账号创建时间早于邀请活动门槛，不符合奖励条件")
	}
	var code InvitationCode
	if err := app.db.Where("code = ?", codeText).First(&code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return InvitationBindingResult{}, businessConflict("邀请码不存在")
		}
		return InvitationBindingResult{}, err
	}
	if code.UserID == invitee.ID {
		return InvitationBindingResult{}, businessConflict("不能使用自己的邀请码")
	}

	var binding InvitationBinding
	err = app.db.Where("invitee_user_id = ?", invitee.ID).First(&binding).Error
	if err == nil {
		if binding.InviterUserID != code.UserID {
			return InvitationBindingResult{}, businessConflict("当前账号已经绑定过其他邀请人")
		}
		if binding.RewardStatus == inviteRewarded {
			return InvitationBindingResult{AlreadyBound: true, InviteCode: binding.InviteCode, RewardAmount: binding.RewardAmount, Message: "邀请关系已绑定，奖励已发放"}, nil
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		binding = InvitationBinding{
			InvitationCodeID: code.ID,
			InviteCode:       code.Code,
			InviterUserID:    code.UserID,
			InviteeUserID:    invitee.ID,
			Platform:         platform,
			ExternalUserID:   externalUserID,
			InviteeCreatedAt: JSONTime{Time: invitee.CreatedAt},
			RewardAmount:     config.Amount,
			RewardStatus:     inviteRewardPending,
		}
		if err := app.db.Create(&binding).Error; err != nil {
			if !isDuplicateEntry(err) {
				return InvitationBindingResult{}, err
			}
			if err := app.db.Where("invitee_user_id = ?", invitee.ID).First(&binding).Error; err != nil {
				return InvitationBindingResult{}, err
			}
		}
	} else {
		return InvitationBindingResult{}, err
	}

	if err := app.db.Model(&InvitationBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
		"reward_status": inviteRewardPending,
		"error_message": "",
		"updated_at":    time.Now(),
	}).Error; err != nil {
		return InvitationBindingResult{}, err
	}
	idempotencyKey := fmt.Sprintf("invitation-reward-%d", invitee.ID)
	notes := fmt.Sprintf("Invitation reward: invitee_user_id=%d invite_code=%s", invitee.ID, code.Code)
	if err := app.addSub2APIUserBalance(ctx, code.UserID, binding.RewardAmount, idempotencyKey, notes); err != nil {
		message := err.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		_ = app.db.Model(&InvitationBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
			"reward_status": inviteRewardFailed,
			"error_message": message,
			"updated_at":    time.Now(),
		}).Error
		return InvitationBindingResult{}, err
	}
	now := JSONTime{Time: time.Now()}
	if err := app.db.Model(&InvitationBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
		"reward_status": inviteRewarded,
		"rewarded_at":   now.Time,
		"error_message": "",
		"updated_at":    now.Time,
	}).Error; err != nil {
		return InvitationBindingResult{}, err
	}
	return InvitationBindingResult{Bound: true, InviteCode: code.Code, RewardAmount: binding.RewardAmount, Message: "邀请绑定成功，奖励已发放给邀请人"}, nil
}

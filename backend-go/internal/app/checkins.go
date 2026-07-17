package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (app *App) checkIn(c *gin.Context) {
	var req CheckInRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.PlatformType) != "" || strings.TrimSpace(req.Platform) != "" {
		app.respondSocialCheckIn(c, req)
		return
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		badRequest(c, "userId must not be blank")
		return
	}

	app.respondCheckIn(c, userID, nil, checkInMethodDirect, "")
}

func (app *App) socialCheckIn(c *gin.Context) {
	var req CheckInRequest
	if !bindJSON(c, &req) {
		return
	}
	app.respondSocialCheckIn(c, req)
}

func (app *App) respondSocialCheckIn(c *gin.Context, req CheckInRequest) {
	platform := strings.TrimSpace(req.PlatformType)
	if platform == "" {
		platform = strings.TrimSpace(req.Platform)
	}
	platform, externalUserID, err := normalizeSocialBinding(SocialBindingRequest{Platform: platform, UserID: req.UserID})
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	inviteCode := ""
	if strings.TrimSpace(req.InviteCode) != "" {
		inviteCode, err = normalizeInvitationCode(req.InviteCode)
		if err != nil {
			badRequest(c, err.Error())
			return
		}
	}

	var binding SocialAccountBinding
	if err := app.db.Where("platform = ? AND external_user_id = ?", platform, externalUserID).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, SocialBindingRequiredResponse{
				Message:        "社交平台账号未绑定，请先登录并绑定账号",
				Code:           "SOCIAL_ACCOUNT_NOT_BOUND",
				Platform:       platform,
				UserID:         externalUserID,
				ExternalUserID: externalUserID,
				BindingURL:     app.socialBindingURL(c, platform, externalUserID, inviteCode),
				InviteCode:     inviteCode,
			})
			return
		}
		serverError(c, err)
		return
	}

	canonicalUserID := strconv.FormatInt(binding.UserID, 10)
	app.respondCheckIn(c, canonicalUserID, &binding.UserID, checkInMethodSocial, platform)
}

func (app *App) socialBindingURL(c *gin.Context, platform, externalUserID, inviteCode string) string {
	baseURL, err := app.frontendPublicURL()
	if err != nil {
		baseURL = ""
	}
	if baseURL == "" {
		baseURL = requestPublicOrigin(c)
	}
	return buildSocialBindingURL(baseURL, platform, externalUserID, inviteCode)
}

func buildSocialBindingURL(baseURL, platform, externalUserID, inviteCode string) string {
	return buildSocialBindingURLWithToken(baseURL, platform, externalUserID, inviteCode, "")
}

func buildSocialBindingURLWithToken(baseURL, platform, externalUserID, inviteCode, bindingToken string) string {
	values := url.Values{}
	values.Set("platform", platform)
	values.Set("userid", externalUserID)
	if inviteCode != "" {
		values.Set("invitecode", inviteCode)
	}
	if bindingToken != "" {
		values.Set("bindingtoken", bindingToken)
	}
	prefix := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if prefix == "" {
		return "/?" + values.Encode()
	}
	return prefix + "/?" + values.Encode()
}

func requestPublicOrigin(c *gin.Context) string {
	host := firstHeaderValue(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = firstHeaderValue(c.Request.Host)
	}
	if !isValidPublicHost(host) {
		return ""
	}
	proto := strings.ToLower(firstHeaderValue(c.GetHeader("X-Forwarded-Proto")))
	if proto != "http" && proto != "https" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	return proto + "://" + host
}

func firstHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.Index(value, ","); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func isValidPublicHost(host string) bool {
	if host == "" || len(host) > 255 {
		return false
	}
	return !strings.ContainsAny(host, " \t\r\n/\\")
}

func (app *App) getUserCheckInStatus(c *gin.Context) {
	user, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}
	userID := strconv.FormatInt(user.ID, 10)
	today := Today()
	if response, found, err := app.todayCheckInResponse(userID, today); err != nil {
		serverError(c, err)
		return
	} else if found {
		if err := app.attachPublicCheckInSettings(&response); err != nil {
			serverError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
		return
	}
	response := CheckInResponse{
		Success:          true,
		AlreadyCheckedIn: false,
		UserID:           &userID,
		SignDate:         &today,
		Amount:           Amount{Decimal: decimal.Zero},
		Message:          "今日尚未签到",
	}
	if err := app.attachPublicCheckInSettings(&response); err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) userCheckIn(c *gin.Context) {
	user, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}
	app.respondCheckIn(c, strconv.FormatInt(user.ID, 10), &user.ID, checkInMethodDirect, "")
}

func (app *App) respondCheckIn(c *gin.Context, userID string, autoRedeemUserID *int64, checkInMethod, platformType string) {
	today := Today()
	if response, found, err := app.todayCheckInResponse(userID, today); err != nil {
		serverError(c, err)
		return
	} else if found {
		if err := app.attachPublicCheckInSettings(&response); err != nil {
			serverError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
		return
	}

	response, err := app.createCheckIn(c.Request.Context(), userID, today, autoRedeemUserID, checkInMethod, platformType)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicateEntry(err) {
			if response, found, err := app.todayCheckInResponse(userID, today); err == nil && found {
				if settingsErr := app.attachPublicCheckInSettings(&response); settingsErr != nil {
					serverError(c, settingsErr)
					return
				}
				c.JSON(http.StatusOK, response)
				return
			}
		}
		if isBusinessConflict(err) {
			conflict(c, err.Error())
			return
		}
		var upstreamErr upstreamAPIError
		if errors.As(err, &upstreamErr) {
			respondSub2APIError(c, err)
			return
		}
		serverError(c, err)
		return
	}
	if err := app.attachPublicCheckInSettings(&response); err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) todayCheckInResponse(userID string, today LocalDate) (CheckInResponse, bool, error) {
	var existingRecord CheckInRecord
	tx := app.db.Where("user_id = ? AND sign_date = ?", userID, today).Limit(1).Find(&existingRecord)
	if tx.Error != nil {
		return CheckInResponse{}, false, tx.Error
	}
	if tx.RowsAffected == 0 {
		return CheckInResponse{}, false, nil
	}

	var code RedeemCode
	if err := app.db.First(&code, existingRecord.RedeemCodeID).Error; err != nil {
		return CheckInResponse{}, false, err
	}
	response := toCheckInResponse(code, true, "already checked in today", existingRecord.CheckInMethod, existingRecord.PlatformType)
	return response, true, nil
}

func (app *App) createCheckIn(ctx context.Context, userID string, today LocalDate, autoRedeemUserID *int64, checkInMethod, platformType string) (CheckInResponse, error) {
	var response CheckInResponse
	err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := app.consumeDailyQuota(tx, today, checkInMethod, platformType); err != nil {
			return err
		}
		drawnAmount, err := app.drawAmount(checkInMethod, platformType)
		if err != nil {
			return err
		}

		status := statusAssigned
		message := "签到成功"
		remoteCode := ""
		if autoRedeemUserID != nil {
			remoteCode = sub2APIIdempotencyKey(userID, today, drawnAmount)
			notes := "Daily check-in reward"
			if checkInMethod == checkInMethodSocial {
				notes = "Social check-in reward: " + platformType
			}
			if err := app.addSub2APIUserBalance(ctx, *autoRedeemUserID, drawnAmount, remoteCode, notes); err != nil {
				return err
			}
			status = statusUsed
			message = "签到成功，奖励已自动入账"
		} else {
			generatedCode, err := app.generateSub2APIRedeemCode(ctx, userID, today, drawnAmount)
			if err != nil {
				return err
			}
			remoteCode = generatedCode
		}

		savedCode := RedeemCode{
			Code:     remoteCode,
			UserID:   &userID,
			SignDate: &today,
			Amount:   drawnAmount,
			Status:   status,
		}
		if err := tx.Create(&savedCode).Error; err != nil {
			return err
		}
		record := CheckInRecord{
			UserID:        userID,
			SignDate:      today,
			RedeemCodeID:  savedCode.ID,
			CheckInMethod: checkInMethod,
			PlatformType:  platformType,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		response = toCheckInResponse(savedCode, false, message, checkInMethod, platformType)
		return nil
	})
	return response, err
}

func (app *App) attachPublicCheckInSettings(response *CheckInResponse) error {
	directTiers, err := app.getPrizeTiers(prizeTiersKey, defaultPrizeTiers)
	if err != nil {
		return err
	}
	socialTiers, err := app.getPrizeTiers(socialPrizeTiersKey, directTiers)
	if err != nil {
		return err
	}
	groupLink := app.checkInGroupLink()
	if response.CheckInMethod == checkInMethodSocial && strings.TrimSpace(response.PlatformType) != "" {
		if platformConfig, _, err := app.platformCheckInConfig(response.PlatformType); err != nil {
			return err
		} else {
			groupLink = platformConfig.GroupLink
			socialTiers = platformConfig.PrizeTiers
		}
	}
	response.GroupLink = groupLink
	response.SocialPrizeTiers = socialTiers
	return nil
}

func (app *App) checkInGroupLink() string {
	value, _, err := app.getSetting(checkInGroupLinkKey)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func (app *App) consumeDailyQuota(tx *gorm.DB, today LocalDate, checkInMethod, platformType string) error {
	mode, err := app.getDailyLimitMode()
	if err != nil {
		return err
	}
	if mode == "separate" {
		return app.consumeMethodDailyQuota(tx, today, checkInMethod, platformType)
	}
	return app.consumeSharedDailyQuota(tx, today)
}

func (app *App) consumeSharedDailyQuota(tx *gorm.DB, today LocalDate) error {
	dailyMaxUsers, err := app.getDailyMaxUsers()
	if err != nil {
		return err
	}
	if dailyMaxUsers <= 0 {
		return businessConflict("今日签到名额已满")
	}

	limit := DailyCheckInLimit{SignDate: today, CheckedCount: 0}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&limit).Error; err != nil {
		return err
	}

	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sign_date = ?", today).First(&limit).Error
	if err != nil {
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

func (app *App) consumeMethodDailyQuota(tx *gorm.DB, today LocalDate, checkInMethod, platformType string) error {
	if checkInMethod == "" {
		checkInMethod = checkInMethodDirect
	}
	limitKey := checkInMethod
	dailyMaxUsers, err := app.dailyMaxUsersForMethod(checkInMethod)
	if err != nil {
		return err
	}
	countQuery := tx.Model(&CheckInRecord{}).Where("sign_date = ? AND check_in_method = ?", today, checkInMethod)
	if checkInMethod == checkInMethodSocial && strings.TrimSpace(platformType) != "" {
		if platformConfig, effective, err := app.platformCheckInConfig(platformType); err != nil {
			return err
		} else if effective {
			dailyMaxUsers = platformConfig.DailyMaxUsers
			limitKey = platformMethodKey(platformType)
			countQuery = countQuery.Where("platform_type = ?", platformType)
		}
	}
	if dailyMaxUsers <= 0 {
		return businessConflict("daily check-in quota is full")
	}

	var existingCount int64
	if err := countQuery.Count(&existingCount).Error; err != nil {
		return err
	}

	limit := DailyCheckInMethodLimit{SignDate: today, CheckInMethod: limitKey, CheckedCount: int(existingCount)}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&limit).Error; err != nil {
		return err
	}

	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("sign_date = ? AND check_in_method = ?", today, limitKey).
		First(&limit).Error
	if err != nil {
		return err
	}
	if limit.CheckedCount >= dailyMaxUsers {
		return businessConflict("daily check-in quota is full")
	}
	return tx.Model(&DailyCheckInMethodLimit{}).
		Where("sign_date = ? AND check_in_method = ?", today, limitKey).
		Updates(map[string]any{
			"checked_count": gorm.Expr("checked_count + 1"),
			"updated_at":    time.Now(),
		}).Error
}

func (app *App) dailyMaxUsersForMethod(checkInMethod string) (int, error) {
	shared, err := app.getDailyMaxUsers()
	if err != nil {
		return 0, err
	}
	if checkInMethod == checkInMethodSocial {
		return app.getMethodDailyMaxUsers(socialDailyMaxUsersKey, shared)
	}
	return app.getMethodDailyMaxUsers(directDailyMaxUsersKey, shared)
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

func toCheckInResponse(code RedeemCode, alreadyCheckedIn bool, message, checkInMethod, platformType string) CheckInResponse {
	if checkInMethod == "" {
		checkInMethod = checkInMethodDirect
	}
	return CheckInResponse{
		Success:          true,
		AlreadyCheckedIn: alreadyCheckedIn,
		UserID:           code.UserID,
		SignDate:         code.SignDate,
		Code:             code.Code,
		Amount:           code.Amount,
		CheckInMethod:    checkInMethod,
		PlatformType:     platformType,
		Message:          message,
	}
}

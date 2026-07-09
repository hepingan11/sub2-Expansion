package app

import (
	"context"
	"errors"
	"net/http"
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

	var binding SocialAccountBinding
	if err := app.db.Where("platform = ? AND external_user_id = ?", platform, externalUserID).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "social account binding not found"})
			return
		}
		serverError(c, err)
		return
	}

	canonicalUserID := strconv.FormatInt(binding.UserID, 10)
	app.respondCheckIn(c, canonicalUserID, &binding.UserID, checkInMethodSocial, platform)
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
		c.JSON(http.StatusOK, response)
		return
	}
	c.JSON(http.StatusOK, CheckInResponse{
		Success:          true,
		AlreadyCheckedIn: false,
		UserID:           &userID,
		SignDate:         &today,
		Amount:           Amount{Decimal: decimal.Zero},
		Message:          "今日尚未签到",
	})
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
		c.JSON(http.StatusOK, response)
		return
	}

	response, err := app.createCheckIn(c.Request.Context(), userID, today, autoRedeemUserID, checkInMethod, platformType)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicateEntry(err) {
			if response, found, err := app.todayCheckInResponse(userID, today); err == nil && found {
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
	return toCheckInResponse(code, true, "already checked in today", existingRecord.CheckInMethod, existingRecord.PlatformType), true, nil
}

func (app *App) createCheckIn(ctx context.Context, userID string, today LocalDate, autoRedeemUserID *int64, checkInMethod, platformType string) (CheckInResponse, error) {
	var response CheckInResponse
	err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := app.consumeDailyQuota(tx, today, checkInMethod); err != nil {
			return err
		}
		drawnAmount, err := app.drawAmount(checkInMethod)
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

func (app *App) consumeDailyQuota(tx *gorm.DB, today LocalDate, checkInMethod string) error {
	mode, err := app.getDailyLimitMode()
	if err != nil {
		return err
	}
	if mode == "separate" {
		return app.consumeMethodDailyQuota(tx, today, checkInMethod)
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

func (app *App) consumeMethodDailyQuota(tx *gorm.DB, today LocalDate, checkInMethod string) error {
	if checkInMethod == "" {
		checkInMethod = checkInMethodDirect
	}
	dailyMaxUsers, err := app.dailyMaxUsersForMethod(checkInMethod)
	if err != nil {
		return err
	}
	if dailyMaxUsers <= 0 {
		return businessConflict("daily check-in quota is full")
	}

	var existingCount int64
	if err := tx.Model(&CheckInRecord{}).
		Where("sign_date = ? AND check_in_method = ?", today, checkInMethod).
		Count(&existingCount).Error; err != nil {
		return err
	}

	limit := DailyCheckInMethodLimit{SignDate: today, CheckInMethod: checkInMethod, CheckedCount: int(existingCount)}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&limit).Error; err != nil {
		return err
	}

	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("sign_date = ? AND check_in_method = ?", today, checkInMethod).
		First(&limit).Error
	if err != nil {
		return err
	}
	if limit.CheckedCount >= dailyMaxUsers {
		return businessConflict("daily check-in quota is full")
	}
	return tx.Model(&DailyCheckInMethodLimit{}).
		Where("sign_date = ? AND check_in_method = ?", today, checkInMethod).
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

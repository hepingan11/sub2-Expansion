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
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		badRequest(c, "userId must not be blank")
		return
	}

	app.respondCheckIn(c, userID, nil)
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
	app.respondCheckIn(c, strconv.FormatInt(user.ID, 10), &user.ID)
}

func (app *App) respondCheckIn(c *gin.Context, userID string, autoRedeemUserID *int64) {
	today := Today()
	if response, found, err := app.todayCheckInResponse(userID, today); err != nil {
		serverError(c, err)
		return
	} else if found {
		c.JSON(http.StatusOK, response)
		return
	}

	response, err := app.createCheckIn(c.Request.Context(), userID, today, autoRedeemUserID)
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
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) todayCheckInResponse(userID string, today LocalDate) (CheckInResponse, bool, error) {
	var existingRecord CheckInRecord
	err := app.db.Where("user_id = ? AND sign_date = ?", userID, today).First(&existingRecord).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CheckInResponse{}, false, nil
	}
	if err != nil {
		return CheckInResponse{}, false, err
	}

	var code RedeemCode
	if err := app.db.First(&code, existingRecord.RedeemCodeID).Error; err != nil {
		return CheckInResponse{}, false, err
	}
	return toCheckInResponse(code, true, "今日已签到"), true, nil
}

func (app *App) createCheckIn(ctx context.Context, userID string, today LocalDate, autoRedeemUserID *int64) (CheckInResponse, error) {
	var response CheckInResponse
	err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := app.consumeDailyQuota(tx, today); err != nil {
			return err
		}
		drawnAmount, err := app.drawAmount()
		if err != nil {
			return err
		}

		status := statusAssigned
		message := "签到成功"
		remoteCode := ""
		if autoRedeemUserID != nil {
			remoteCode = sub2APIIdempotencyKey(userID, today, drawnAmount)
			if err := app.createAndRedeemSub2APIBalance(ctx, *autoRedeemUserID, drawnAmount, remoteCode, "Daily check-in reward"); err != nil {
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
		record := CheckInRecord{UserID: userID, SignDate: today, RedeemCodeID: savedCode.ID}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		response = toCheckInResponse(savedCode, false, message)
		return nil
	})
	return response, err
}

func (app *App) consumeDailyQuota(tx *gorm.DB, today LocalDate) error {
	dailyMaxUsers, err := app.getDailyMaxUsers()
	if err != nil {
		return err
	}
	if dailyMaxUsers <= 0 {
		return businessConflict("今日签到名额已满")
	}

	var limit DailyCheckInLimit
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sign_date = ?", today).First(&limit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		limit = DailyCheckInLimit{SignDate: today, CheckedCount: 0}
		if err := tx.Create(&limit).Error; err != nil {
			return err
		}
	} else if err != nil {
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

func toCheckInResponse(code RedeemCode, alreadyCheckedIn bool, message string) CheckInResponse {
	return CheckInResponse{
		Success:          true,
		AlreadyCheckedIn: alreadyCheckedIn,
		UserID:           code.UserID,
		SignDate:         code.SignDate,
		Code:             code.Code,
		Amount:           code.Amount,
		Message:          message,
	}
}

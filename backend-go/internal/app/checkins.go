package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

	today := Today()
	var existingRecord CheckInRecord
	err := app.db.Where("user_id = ? AND sign_date = ?", userID, today).First(&existingRecord).Error
	if err == nil {
		var code RedeemCode
		if err := app.db.First(&code, existingRecord.RedeemCodeID).Error; err == nil {
			c.JSON(http.StatusOK, toCheckInResponse(code, true, "今日已签到"))
			return
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		serverError(c, err)
		return
	}

	response, err := app.createCheckIn(c.Request.Context(), userID, today)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicateEntry(err) {
			var record CheckInRecord
			if err := app.db.Where("user_id = ? AND sign_date = ?", userID, today).First(&record).Error; err == nil {
				var code RedeemCode
				if err := app.db.First(&code, record.RedeemCodeID).Error; err == nil {
					c.JSON(http.StatusOK, toCheckInResponse(code, true, "今日已签到"))
					return
				}
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

func (app *App) createCheckIn(ctx context.Context, userID string, today LocalDate) (CheckInResponse, error) {
	var response CheckInResponse
	err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := app.consumeDailyQuota(tx, today); err != nil {
			return err
		}
		drawnAmount, err := app.drawAmount()
		if err != nil {
			return err
		}
		remoteCode, err := app.generateSub2APIRedeemCode(ctx, userID, today, drawnAmount)
		if err != nil {
			return err
		}

		savedCode := RedeemCode{
			Code:     remoteCode,
			UserID:   &userID,
			SignDate: &today,
			Amount:   drawnAmount,
			Status:   statusAssigned,
		}
		if err := tx.Create(&savedCode).Error; err != nil {
			return err
		}
		record := CheckInRecord{UserID: userID, SignDate: today, RedeemCodeID: savedCode.ID}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		response = toCheckInResponse(savedCode, false, "签到成功")
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

package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func (app *App) getCheckInStats(c *gin.Context) {
	today := Today()
	start := LocalDate{Time: today.AddDate(0, 0, -29)}

	rows := []DailyCheckInStatEntry{}
	if err := app.db.Table("check_in_records AS cir").
		Select("cir.sign_date, COALESCE(SUM(rc.amount), 0) AS amount, COUNT(*) AS users").
		Joins("LEFT JOIN redeem_codes AS rc ON rc.id = cir.redeem_code_id").
		Where("cir.sign_date >= ? AND cir.sign_date <= ?", start, today).
		Group("cir.sign_date").
		Order("cir.sign_date ASC").
		Scan(&rows).Error; err != nil {
		serverError(c, err)
		return
	}

	byDate := map[string]DailyCheckInStatEntry{}
	for _, row := range rows {
		byDate[row.SignDate.Format("2006-01-02")] = row
	}

	daily := make([]DailyCheckInStatEntry, 0, 30)
	for day := start.Time; !day.After(today.Time); day = day.AddDate(0, 0, 1) {
		signDate := LocalDate{Time: day}
		key := signDate.Format("2006-01-02")
		entry, ok := byDate[key]
		if !ok {
			entry = DailyCheckInStatEntry{
				SignDate: signDate,
				Amount:   Amount{Decimal: decimal.Zero},
				Users:    0,
			}
		}
		daily = append(daily, entry)
	}

	todayEntry := byDate[today.Format("2006-01-02")]
	c.JSON(http.StatusOK, CheckInStatsResponse{
		TodayAmount: todayEntry.Amount,
		TodayUsers:  todayEntry.Users,
		Daily:       daily,
	})
}

package app

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func (app *App) listCodes(c *gin.Context) {
	page := max(queryInt(c, "page", 0), 0)
	size := min(max(queryInt(c, "size", 10), 1), 100)
	var total int64
	codes := []RedeemCode{}

	query := app.applyCodeFilters(app.db.Model(&RedeemCode{}), c)
	if err := query.Count(&total).Error; err != nil {
		serverError(c, err)
		return
	}
	if err := query.Order("created_at DESC").Limit(size).Offset(page * size).Find(&codes).Error; err != nil {
		serverError(c, err)
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}
	c.JSON(http.StatusOK, PageResponse[RedeemCode]{
		Content:       codes,
		TotalElements: total,
		TotalPages:    totalPages,
		Number:        page,
		Size:          size,
	})
}

func (app *App) getCode(c *gin.Context) {
	code, ok := app.findCodeByID(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, code)
}

func (app *App) createCode(c *gin.Context) {
	var req RedeemCodeRequest
	if !bindJSON(c, &req) || !validateCodeRequest(c, &req) {
		return
	}
	code := RedeemCode{}
	if err := app.applyCodeRequest(&code, req, true); err != nil {
		conflict(c, err.Error())
		return
	}
	if err := app.db.Create(&code).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, code)
}

func (app *App) updateCode(c *gin.Context) {
	code, ok := app.findCodeByID(c)
	if !ok {
		return
	}
	var req RedeemCodeRequest
	if !bindJSON(c, &req) || !validateCodeRequest(c, &req) {
		return
	}
	if err := app.applyCodeRequest(&code, req, false); err != nil {
		conflict(c, err.Error())
		return
	}
	if err := app.db.Save(&code).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, code)
}

func (app *App) deleteCode(c *gin.Context) {
	code, ok := app.findCodeByID(c)
	if !ok {
		return
	}
	if err := app.db.Delete(&code).Error; err != nil {
		serverError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (app *App) batchImportCodes(c *gin.Context) {
	var req BatchImportCodesRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.CodesText) == "" {
		badRequest(c, "codesText must not be blank")
		return
	}
	if req.Amount.Cmp(decimal.NewFromFloat(0.01)) < 0 {
		badRequest(c, "amount must be greater than or equal to 0.01")
		return
	}
	parsedCodes := parseCodes(req.CodesText)
	if len(parsedCodes) == 0 {
		badRequest(c, "请至少粘贴一个兑换码")
		return
	}

	var existing []RedeemCode
	if err := app.db.Where("code IN ?", parsedCodes).Find(&existing).Error; err != nil {
		serverError(c, err)
		return
	}
	existingSet := map[string]bool{}
	duplicatedCodes := make([]string, 0, len(existing))
	for _, code := range existing {
		existingSet[code.Code] = true
		duplicatedCodes = append(duplicatedCodes, code.Code)
	}

	newCodes := make([]RedeemCode, 0, len(parsedCodes)-len(existingSet))
	for _, value := range parsedCodes {
		if existingSet[value] {
			continue
		}
		newCodes = append(newCodes, RedeemCode{Code: value, Amount: req.Amount, Status: statusAvailable})
	}
	if len(newCodes) > 0 {
		if err := app.db.Create(&newCodes).Error; err != nil {
			handleDBError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, BatchImportCodesResponse{
		TotalParsed:     len(parsedCodes),
		Imported:        len(newCodes),
		Duplicated:      len(existingSet),
		DuplicatedCodes: duplicatedCodes,
	})
}

func (app *App) stats(c *gin.Context) {
	countStatus := func(status string) int64 {
		var count int64
		_ = app.db.Model(&RedeemCode{}).Where("status = ?", status).Count(&count).Error
		return count
	}
	var total int64
	if err := app.db.Model(&RedeemCode{}).Count(&total).Error; err != nil {
		serverError(c, err)
		return
	}
	amountStats, err := app.amountStats()
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, DashboardStatsResponse{
		Total:       total,
		Available:   countStatus(statusAvailable),
		Assigned:    countStatus(statusAssigned),
		Used:        countStatus(statusUsed),
		Voided:      countStatus(statusVoided),
		AmountStats: amountStats,
	})
}

func (app *App) amountStats() ([]AmountStatEntry, error) {
	result := []AmountStatEntry{}
	err := app.db.Model(&RedeemCode{}).
		Select("amount, COUNT(*) AS total, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS available", statusAvailable).
		Group("amount").
		Order("amount ASC").
		Scan(&result).Error
	return result, err
}

func (app *App) applyCodeFilters(query *gorm.DB, c *gin.Context) *gorm.DB {
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR user_id LIKE ?", pattern, pattern)
	}
	if userID := strings.TrimSpace(c.Query("userId")); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if startDate := strings.TrimSpace(c.Query("startDate")); startDate != "" {
		if parsed, err := ParseLocalDate(startDate); err == nil {
			query = query.Where("sign_date >= ?", parsed)
		}
	}
	if endDate := strings.TrimSpace(c.Query("endDate")); endDate != "" {
		if parsed, err := ParseLocalDate(endDate); err == nil {
			query = query.Where("sign_date <= ?", parsed)
		}
	}
	return query
}

func (app *App) findCodeByID(c *gin.Context) (RedeemCode, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "invalid id")
		return RedeemCode{}, false
	}
	var code RedeemCode
	if err := app.db.First(&code, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "Redeem code not found: " + c.Param("id")})
			return RedeemCode{}, false
		}
		serverError(c, err)
		return RedeemCode{}, false
	}
	return code, true
}

func validateCodeRequest(c *gin.Context, req *RedeemCodeRequest) bool {
	if req.Amount.Cmp(decimal.NewFromFloat(0.01)) < 0 {
		badRequest(c, "amount must be greater than or equal to 0.01")
		return false
	}
	if req.Status == "" {
		req.Status = statusAvailable
	}
	if !validStatuses[req.Status] {
		badRequest(c, "status is invalid")
		return false
	}
	return true
}

func (app *App) applyCodeRequest(code *RedeemCode, req RedeemCodeRequest, creating bool) error {
	normalizedCode := strings.TrimSpace(req.Code)
	if creating && normalizedCode == "" {
		generated, err := app.uniqueCode()
		if err != nil {
			return err
		}
		normalizedCode = generated
	}
	if normalizedCode != "" {
		code.Code = normalizedCode
	}
	code.Amount = req.Amount
	code.Status = req.Status
	if code.Status == statusAvailable {
		code.UserID = nil
		code.SignDate = nil
	} else {
		userID := strings.TrimSpace(req.UserID)
		if userID == "" {
			code.UserID = nil
		} else {
			code.UserID = &userID
		}
		code.SignDate = req.SignDate
	}
	return nil
}

func (app *App) uniqueCode() (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		code, err := randomCode()
		if err != nil {
			return "", err
		}
		var count int64
		if err := app.db.Model(&RedeemCode{}).Where("code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("Unable to generate a unique redeem code")
}

func randomCode() (string, error) {
	bytes := make([]byte, 14)
	randomBytes := make([]byte, 14)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	for i, value := range randomBytes {
		bytes[i] = codeAlphabet[int(value)%len(codeAlphabet)]
	}
	return "RC" + string(bytes), nil
}

func parseCodes(text string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, rawCode := range codeSplitPattern.Split(text, -1) {
		code := strings.TrimSpace(rawCode)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	return result
}

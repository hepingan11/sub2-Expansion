package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const stalePendingClaimAfter = 10 * time.Minute

type sub2APIUserSnapshot struct {
	ID             int64     `json:"id"`
	TotalRecharged float64   `json:"total_recharged"`
	CreatedAt      time.Time `json:"created_at"`
}

func (app *App) listRechargeActivities(c *gin.Context) {
	activities, err := app.loadRechargeActivities()
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, activities)
}

func (app *App) createRechargeActivity(c *gin.Context) {
	var req RechargeActivityRequest
	if !bindJSON(c, &req) {
		return
	}
	activity, tiers, err := normalizeRechargeActivityRequest(req)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}
		for i := range tiers {
			tiers[i].ActivityID = activity.ID
			if err := tx.Create(&tiers[i]).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		serverError(c, err)
		return
	}
	resp, err := app.loadRechargeActivity(activity.ID)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (app *App) updateRechargeActivity(c *gin.Context) {
	id, ok := pathUint64(c, "id")
	if !ok {
		return
	}
	var req RechargeActivityRequest
	if !bindJSON(c, &req) {
		return
	}
	activity, tiers, err := normalizeRechargeActivityRequest(req)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	activity.ID = id

	if err := app.db.Transaction(func(tx *gorm.DB) error {
		var existing RechargeActivity
		if err := tx.First(&existing, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Model(&existing).Updates(map[string]any{
			"name":        activity.Name,
			"description": activity.Description,
			"enabled":     activity.Enabled,
			"start_at":    activity.StartAt,
			"end_at":      activity.EndAt,
			"updated_at":  time.Now(),
		}).Error; err != nil {
			return err
		}
		return app.replaceRechargeRewardTiers(tx, id, tiers)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "activity not found"})
			return
		}
		serverError(c, err)
		return
	}
	resp, err := app.loadRechargeActivity(id)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (app *App) deleteRechargeActivity(c *gin.Context) {
	id, ok := pathUint64(c, "id")
	if !ok {
		return
	}
	if err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&RechargeRewardTier{}, "activity_id = ?", id).Error; err != nil {
			return err
		}
		result := tx.Delete(&RechargeActivity{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "activity not found"})
			return
		}
		serverError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (app *App) listAdminRechargeRewardClaims(c *gin.Context) {
	page := max(queryInt(c, "page", 0), 0)
	size := min(max(queryInt(c, "size", 10), 1), 100)
	query := app.db.Model(&RechargeRewardClaim{})

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if userIDText := strings.TrimSpace(c.Query("userId")); userIDText != "" {
		userID, err := strconv.ParseInt(userIDText, 10, 64)
		if err != nil || userID <= 0 {
			badRequest(c, "invalid userId")
			return
		}
		query = query.Where("user_id = ?", userID)
	}
	if activityIDText := strings.TrimSpace(c.Query("activityId")); activityIDText != "" {
		activityID, err := strconv.ParseUint(activityIDText, 10, 64)
		if err != nil || activityID == 0 {
			badRequest(c, "invalid activityId")
			return
		}
		query = query.Where("activity_id = ?", activityID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		if userID, err := strconv.ParseInt(keyword, 10, 64); err == nil && userID > 0 {
			query = query.Where("user_id = ? OR redeem_code LIKE ?", userID, like)
		} else {
			query = query.Where("redeem_code LIKE ?", like)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		serverError(c, err)
		return
	}

	var claims []RechargeRewardClaim
	if err := query.Order("updated_at DESC, id DESC").Limit(size).Offset(page * size).Find(&claims).Error; err != nil {
		serverError(c, err)
		return
	}

	responses, err := app.rechargeRewardClaimResponses(claims)
	if err != nil {
		serverError(c, err)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}
	c.JSON(http.StatusOK, PageResponse[AdminRechargeRewardClaimResponse]{
		Content:       responses,
		TotalElements: total,
		TotalPages:    totalPages,
		Number:        page,
		Size:          size,
	})
}

func (app *App) listUserRechargeRewards(c *gin.Context) {
	user, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}
	activities, err := app.loadActiveRechargeActivities()
	if err != nil {
		serverError(c, err)
		return
	}
	claims, err := app.loadRechargeClaims(user.ID)
	if err != nil {
		serverError(c, err)
		return
	}
	total := Amount{decimal.NewFromFloat(user.TotalRecharged).Round(2)}
	resp := UserRechargeRewardsResponse{
		TotalRecharged: total,
		Activities:     make([]UserRechargeActivityResponse, 0, len(activities)),
	}
	for _, item := range activities {
		activityResp := UserRechargeActivityResponse{
			ID:          item.Activity.ID,
			Name:        item.Activity.Name,
			Description: item.Activity.Description,
			StartAt:     item.Activity.StartAt,
			EndAt:       item.Activity.EndAt,
			Tiers:       make([]UserRechargeRewardTierResponse, 0, len(item.Tiers)),
		}
		for _, tier := range item.Tiers {
			claim := claims[tier.ID]
			activityResp.Tiers = append(activityResp.Tiers, userRechargeTierResponse(tier, total, claim))
		}
		resp.Activities = append(resp.Activities, activityResp)
	}
	c.JSON(http.StatusOK, resp)
}

func (app *App) claimRechargeReward(c *gin.Context) {
	user, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}
	activityID, ok := pathUint64(c, "activityId")
	if !ok {
		return
	}
	tierID, ok := pathUint64(c, "tierId")
	if !ok {
		return
	}

	var activity RechargeActivity
	var tier RechargeRewardTier
	if err := app.db.First(&activity, "id = ?", activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, APIError{Message: "activity not found"})
		return
	}
	if !rechargeActivityAvailable(activity, time.Now()) {
		conflict(c, "activity is not available")
		return
	}
	if err := app.db.First(&tier, "id = ? AND activity_id = ?", tierID, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, APIError{Message: "tier not found"})
		return
	}
	total := Amount{decimal.NewFromFloat(user.TotalRecharged).Round(2)}
	if total.Cmp(tier.ThresholdAmount.Decimal) < 0 {
		conflict(c, "recharge threshold not reached")
		return
	}

	claim, err := app.reserveRechargeRewardClaim(user.ID, activity, tier)
	if err != nil {
		if isBusinessConflict(err) {
			conflict(c, err.Error())
			return
		}
		serverError(c, err)
		return
	}

	redeemCode := rechargeRewardCode(activityID, tierID, user.ID)
	if err := app.createAndRedeemSub2APIBalance(c.Request.Context(), user.ID, tier.RewardAmount, redeemCode, fmt.Sprintf("Recharge reward: %s", activity.Name)); err != nil {
		_ = app.markRechargeRewardClaimFailed(claim.ID, err.Error())
		respondSub2APIError(c, err)
		return
	}
	if err := app.markRechargeRewardClaimClaimed(claim.ID, redeemCode); err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, ClaimRechargeRewardResponse{ClaimID: claim.ID, RedeemCode: redeemCode, RewardAmount: tier.RewardAmount})
}

type rechargeActivityWithTiers struct {
	Activity RechargeActivity
	Tiers    []RechargeRewardTier
}

func (app *App) loadRechargeActivities() ([]RechargeActivityResponse, error) {
	var activities []RechargeActivity
	if err := app.db.Order("id DESC").Find(&activities).Error; err != nil {
		return nil, err
	}
	result := make([]RechargeActivityResponse, 0, len(activities))
	for _, activity := range activities {
		resp, err := app.loadRechargeActivity(activity.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, resp)
	}
	return result, nil
}

func (app *App) loadRechargeActivity(id uint64) (RechargeActivityResponse, error) {
	var activity RechargeActivity
	if err := app.db.First(&activity, "id = ?", id).Error; err != nil {
		return RechargeActivityResponse{}, err
	}
	var tiers []RechargeRewardTier
	if err := app.db.Where("activity_id = ?", id).Order("sort_order ASC, threshold_amount ASC, id ASC").Find(&tiers).Error; err != nil {
		return RechargeActivityResponse{}, err
	}
	return rechargeActivityResponse(activity, tiers), nil
}

func (app *App) loadActiveRechargeActivities() ([]rechargeActivityWithTiers, error) {
	var activities []RechargeActivity
	if err := app.db.Where("enabled = ?", true).Order("id DESC").Find(&activities).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	result := make([]rechargeActivityWithTiers, 0, len(activities))
	for _, activity := range activities {
		if !rechargeActivityAvailable(activity, now) {
			continue
		}
		var tiers []RechargeRewardTier
		if err := app.db.Where("activity_id = ?", activity.ID).Order("sort_order ASC, threshold_amount ASC, id ASC").Find(&tiers).Error; err != nil {
			return nil, err
		}
		result = append(result, rechargeActivityWithTiers{Activity: activity, Tiers: tiers})
	}
	return result, nil
}

func (app *App) loadRechargeClaims(userID int64) (map[uint64]RechargeRewardClaim, error) {
	var claims []RechargeRewardClaim
	if err := app.db.Where("user_id = ?", userID).Find(&claims).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64]RechargeRewardClaim, len(claims))
	for _, claim := range claims {
		result[claim.TierID] = claim
	}
	return result, nil
}

func (app *App) replaceRechargeRewardTiers(tx *gorm.DB, activityID uint64, tiers []RechargeRewardTier) error {
	var existing []RechargeRewardTier
	if err := tx.Where("activity_id = ?", activityID).Find(&existing).Error; err != nil {
		return err
	}
	existingIDs := map[uint64]bool{}
	for _, tier := range existing {
		existingIDs[tier.ID] = true
	}
	keptIDs := map[uint64]bool{}
	for i := range tiers {
		tiers[i].ActivityID = activityID
		if tiers[i].ID > 0 && existingIDs[tiers[i].ID] {
			keptIDs[tiers[i].ID] = true
			if err := tx.Model(&RechargeRewardTier{}).Where("id = ? AND activity_id = ?", tiers[i].ID, activityID).Updates(map[string]any{
				"threshold_amount": tiers[i].ThresholdAmount,
				"reward_amount":    tiers[i].RewardAmount,
				"sort_order":       tiers[i].SortOrder,
				"updated_at":       time.Now(),
			}).Error; err != nil {
				return err
			}
			continue
		}
		tiers[i].ID = 0
		if err := tx.Create(&tiers[i]).Error; err != nil {
			return err
		}
		keptIDs[tiers[i].ID] = true
	}
	for id := range existingIDs {
		if !keptIDs[id] {
			if err := tx.Delete(&RechargeRewardTier{}, "id = ? AND activity_id = ?", id, activityID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (app *App) reserveRechargeRewardClaim(userID int64, activity RechargeActivity, tier RechargeRewardTier) (RechargeRewardClaim, error) {
	var claim RechargeRewardClaim
	err := app.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("activity_id = ? AND tier_id = ? AND user_id = ?", activity.ID, tier.ID, userID).First(&claim).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if claim.Status == claimClaimed {
				return businessConflict("reward already claimed")
			}
			if claim.Status == claimPending && time.Since(claim.UpdatedAt.Time) < stalePendingClaimAfter {
				return businessConflict("reward claim is being processed")
			}
			claim.Status = claimPending
			claim.ThresholdAmount = tier.ThresholdAmount
			claim.RewardAmount = tier.RewardAmount
			claim.ErrorMessage = ""
			return tx.Save(&claim).Error
		}
		claim = RechargeRewardClaim{
			ActivityID:      activity.ID,
			TierID:          tier.ID,
			UserID:          userID,
			ThresholdAmount: tier.ThresholdAmount,
			RewardAmount:    tier.RewardAmount,
			Status:          claimPending,
		}
		return tx.Create(&claim).Error
	})
	return claim, err
}

func (app *App) markRechargeRewardClaimClaimed(claimID uint64, redeemCode string) error {
	return app.db.Model(&RechargeRewardClaim{}).Where("id = ?", claimID).Updates(map[string]any{
		"status":        claimClaimed,
		"redeem_code":   redeemCode,
		"error_message": "",
		"updated_at":    time.Now(),
	}).Error
}

func (app *App) markRechargeRewardClaimFailed(claimID uint64, message string) error {
	if len(message) > 1000 {
		message = message[:1000]
	}
	return app.db.Model(&RechargeRewardClaim{}).Where("id = ?", claimID).Updates(map[string]any{
		"status":        claimFailed,
		"error_message": message,
		"updated_at":    time.Now(),
	}).Error
}

func (app *App) createAndRedeemSub2APIBalance(ctx context.Context, userID int64, reward Amount, code string, notes string) error {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		return err
	}
	if cfg.BaseURL == "" {
		return businessConflict("Sub2API is not configured: set SUB2API_BASE_URL")
	}
	authName, authValue, err := app.sub2APIAuthHeader(ctx, cfg)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"code":    code,
		"type":    "balance",
		"value":   reward.InexactFloat64(),
		"user_id": userID,
		"notes":   notes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, cfg.BaseURL+"/api/v1/admin/redeem-codes/create-and-redeem", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Idempotency-Key", rechargeRewardIdempotencyKey(code))
	req.Header.Set(authName, authValue)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Sub2API reward recharge failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("parse Sub2API reward response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = resp.Status
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			log.Printf("Sub2API create-and-redeem failed: status=%d user_id=%d reward=%s code=%s message=%q body=%q",
				resp.StatusCode,
				userID,
				reward.StringFixed(2),
				code,
				message,
				truncateLogText(string(respBody), 1024),
			)
		}
		return upstreamAPIError{statusCode: resp.StatusCode, message: message}
	}
	return nil
}

func (app *App) addSub2APIUserBalance(ctx context.Context, userID int64, reward Amount, idempotencyKey string, notes string) error {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		return err
	}
	if cfg.BaseURL == "" {
		return businessConflict("Sub2API is not configured: set SUB2API_BASE_URL")
	}
	authName, authValue, err := app.sub2APIAuthHeader(ctx, cfg)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"balance":   reward.InexactFloat64(),
		"operation": "add",
		"notes":     notes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint := fmt.Sprintf("%s/api/v1/admin/users/%d/balance", cfg.BaseURL, userID)
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	req.Header.Set(authName, authValue)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Sub2API balance update failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("parse Sub2API balance update response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = resp.Status
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			log.Printf("Sub2API balance update failed: status=%d user_id=%d reward=%s idempotency_key=%s message=%q body=%q",
				resp.StatusCode,
				userID,
				reward.StringFixed(2),
				idempotencyKey,
				message,
				truncateLogText(string(respBody), 1024),
			)
		}
		return upstreamAPIError{statusCode: resp.StatusCode, message: message}
	}
	return nil
}

func truncateLogText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func normalizeRechargeActivityRequest(req RechargeActivityRequest) (RechargeActivity, []RechargeRewardTier, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return RechargeActivity{}, nil, errors.New("activity name is required")
	}
	startAt, err := parseOptionalJSONTime(req.StartAt)
	if err != nil {
		return RechargeActivity{}, nil, err
	}
	endAt, err := parseOptionalJSONTime(req.EndAt)
	if err != nil {
		return RechargeActivity{}, nil, err
	}
	if startAt != nil && endAt != nil && !endAt.After(startAt.Time) {
		return RechargeActivity{}, nil, errors.New("end time must be after start time")
	}
	if len(req.Tiers) == 0 {
		return RechargeActivity{}, nil, errors.New("at least one reward tier is required")
	}
	tiers := make([]RechargeRewardTier, 0, len(req.Tiers))
	for index, tier := range req.Tiers {
		if tier.ThresholdAmount.Cmp(decimal.Zero) <= 0 {
			return RechargeActivity{}, nil, errors.New("threshold amount must be greater than 0")
		}
		if tier.RewardAmount.Cmp(decimal.Zero) <= 0 {
			return RechargeActivity{}, nil, errors.New("reward amount must be greater than 0")
		}
		sortOrder := tier.Sort
		if sortOrder == 0 {
			sortOrder = index
		}
		tiers = append(tiers, RechargeRewardTier{
			ID:              tier.ID,
			ThresholdAmount: Amount{tier.ThresholdAmount.Round(2)},
			RewardAmount:    Amount{tier.RewardAmount.Round(2)},
			SortOrder:       sortOrder,
		})
	}
	sort.SliceStable(tiers, func(i, j int) bool {
		if tiers[i].SortOrder == tiers[j].SortOrder {
			return tiers[i].ThresholdAmount.Cmp(tiers[j].ThresholdAmount.Decimal) < 0
		}
		return tiers[i].SortOrder < tiers[j].SortOrder
	})
	return RechargeActivity{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Enabled:     req.Enabled,
		StartAt:     startAt,
		EndAt:       endAt,
	}, tiers, nil
}

func parseOptionalJSONTime(value string) (*JSONTime, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return &JSONTime{Time: parsed}, nil
		}
	}
	return nil, fmt.Errorf("invalid time: %s", value)
}

func rechargeActivityResponse(activity RechargeActivity, tiers []RechargeRewardTier) RechargeActivityResponse {
	resp := RechargeActivityResponse{
		ID:          activity.ID,
		Name:        activity.Name,
		Description: activity.Description,
		Enabled:     activity.Enabled,
		StartAt:     activity.StartAt,
		EndAt:       activity.EndAt,
		CreatedAt:   activity.CreatedAt,
		UpdatedAt:   activity.UpdatedAt,
		Tiers:       make([]RechargeRewardTierResponse, 0, len(tiers)),
	}
	for _, tier := range tiers {
		resp.Tiers = append(resp.Tiers, rechargeTierResponse(tier))
	}
	return resp
}

func rechargeTierResponse(tier RechargeRewardTier) RechargeRewardTierResponse {
	return RechargeRewardTierResponse{
		ID:              tier.ID,
		ActivityID:      tier.ActivityID,
		ThresholdAmount: tier.ThresholdAmount,
		RewardAmount:    tier.RewardAmount,
		Sort:            tier.SortOrder,
		CreatedAt:       tier.CreatedAt,
		UpdatedAt:       tier.UpdatedAt,
	}
}

func userRechargeTierResponse(tier RechargeRewardTier, total Amount, claim RechargeRewardClaim) UserRechargeRewardTierResponse {
	resp := UserRechargeRewardTierResponse{
		ID:              tier.ID,
		ThresholdAmount: tier.ThresholdAmount,
		RewardAmount:    tier.RewardAmount,
		Eligible:        total.Cmp(tier.ThresholdAmount.Decimal) >= 0,
		ClaimStatus:     "",
	}
	if claim.ID > 0 {
		resp.ClaimStatus = claim.Status
		resp.Claimed = claim.Status == claimClaimed
		resp.RedeemCode = claim.RedeemCode
		resp.ClaimedAt = claim.UpdatedAt
	}
	return resp
}

func (app *App) rechargeRewardClaimResponses(claims []RechargeRewardClaim) ([]AdminRechargeRewardClaimResponse, error) {
	activityIDs := make([]uint64, 0, len(claims))
	tierIDs := make([]uint64, 0, len(claims))
	activitySeen := map[uint64]bool{}
	tierSeen := map[uint64]bool{}
	for _, claim := range claims {
		if !activitySeen[claim.ActivityID] {
			activityIDs = append(activityIDs, claim.ActivityID)
			activitySeen[claim.ActivityID] = true
		}
		if !tierSeen[claim.TierID] {
			tierIDs = append(tierIDs, claim.TierID)
			tierSeen[claim.TierID] = true
		}
	}

	activitiesByID := map[uint64]RechargeActivity{}
	if len(activityIDs) > 0 {
		var activities []RechargeActivity
		if err := app.db.Where("id IN ?", activityIDs).Find(&activities).Error; err != nil {
			return nil, err
		}
		for _, activity := range activities {
			activitiesByID[activity.ID] = activity
		}
	}

	tiersByID := map[uint64]RechargeRewardTier{}
	if len(tierIDs) > 0 {
		var tiers []RechargeRewardTier
		if err := app.db.Where("id IN ?", tierIDs).Find(&tiers).Error; err != nil {
			return nil, err
		}
		for _, tier := range tiers {
			tiersByID[tier.ID] = tier
		}
	}

	result := make([]AdminRechargeRewardClaimResponse, 0, len(claims))
	for _, claim := range claims {
		activityName := ""
		if activity, ok := activitiesByID[claim.ActivityID]; ok {
			activityName = activity.Name
		}
		tierSort := 0
		if tier, ok := tiersByID[claim.TierID]; ok {
			tierSort = tier.SortOrder
		}
		result = append(result, AdminRechargeRewardClaimResponse{
			ID:              claim.ID,
			ActivityID:      claim.ActivityID,
			ActivityName:    activityName,
			TierID:          claim.TierID,
			TierSort:        tierSort,
			UserID:          claim.UserID,
			ThresholdAmount: claim.ThresholdAmount,
			RewardAmount:    claim.RewardAmount,
			Status:          claim.Status,
			RedeemCode:      claim.RedeemCode,
			ErrorMessage:    claim.ErrorMessage,
			CreatedAt:       claim.CreatedAt,
			UpdatedAt:       claim.UpdatedAt,
		})
	}
	return result, nil
}

func rechargeActivityAvailable(activity RechargeActivity, now time.Time) bool {
	if !activity.Enabled {
		return false
	}
	if activity.StartAt != nil && !activity.StartAt.Time.IsZero() && now.Before(activity.StartAt.Time) {
		return false
	}
	if activity.EndAt != nil && !activity.EndAt.Time.IsZero() && now.After(activity.EndAt.Time) {
		return false
	}
	return true
}

func sub2APIUserFromContext(c *gin.Context) (sub2APIUserSnapshot, bool) {
	value, ok := c.Get("sub2apiUser")
	if !ok {
		return sub2APIUserSnapshot{}, false
	}
	raw, ok := value.(json.RawMessage)
	if !ok || len(raw) == 0 {
		return sub2APIUserSnapshot{}, false
	}
	var user sub2APIUserSnapshot
	if err := json.Unmarshal(raw, &user); err != nil || user.ID <= 0 {
		return sub2APIUserSnapshot{}, false
	}
	return user, true
}

func pathUint64(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param(key)), 10, 64)
	if err != nil || value == 0 {
		badRequest(c, "invalid "+key)
		return 0, false
	}
	return value, true
}

func rechargeRewardCode(activityID, tierID uint64, userID int64) string {
	return fmt.Sprintf("rr_%d_%d_%d", activityID, tierID, userID)
}

func rechargeRewardIdempotencyKey(code string) string {
	return "recharge-reward-" + code
}

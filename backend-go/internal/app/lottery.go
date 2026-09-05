package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	lotteryPreviewLimit    = 200
	lotteryMaxOrders       = 100000
	lotteryStaleAwardAfter = 10 * time.Minute
	lotteryAwardNone       = "NONE"
	lotteryAwardPending    = "PENDING"
	lotteryAwardProcessing = "PROCESSING"
	lotteryAwarded         = "AWARDED"
	lotteryAwardFailed     = "FAILED"
)

type sub2APIPaymentOrder struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	UserEmail   string     `json:"user_email"`
	UserName    string     `json:"user_name"`
	Amount      float64    `json:"amount"`
	Status      string     `json:"status"`
	OrderType   string     `json:"order_type"`
	PaidAt      *time.Time `json:"paid_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type sub2APIPaginatedOrders struct {
	Items    []sub2APIPaymentOrder `json:"items"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Pages    int                   `json:"pages"`
}

type lotteryCandidate struct {
	UserID         int64
	UserName       string
	UserEmail      string
	RechargeAmount Amount
	OrderCount     int
}

func (app *App) previewLotteryEligibility(c *gin.Context) {
	start, end, minimum, err := parseLotteryPeriod(c.Query("periodStart"), c.Query("periodEnd"), c.Query("minRecharge"))
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	candidates, err := app.loadLotteryCandidates(c.Request.Context(), start, end, minimum)
	if err != nil {
		respondSub2APIError(c, err)
		return
	}
	visible := candidates
	truncated := len(visible) > lotteryPreviewLimit
	if truncated {
		visible = visible[:lotteryPreviewLimit]
	}
	c.JSON(http.StatusOK, LotteryEligibilityResponse{
		PeriodStart: JSONTime{Time: start}, PeriodEnd: JSONTime{Time: end}, MinRecharge: minimum,
		EligibleCount: len(candidates), Candidates: lotteryCandidateResponses(visible), Truncated: truncated,
	})
}

func (app *App) createLotteryDraw(c *gin.Context) {
	var req LotteryDrawRequest
	if !bindJSON(c, &req) {
		return
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.Name = strings.TrimSpace(req.Name)
	if req.RequestID == "" || len(req.RequestID) > 64 {
		badRequest(c, "requestId is required and must not exceed 64 characters")
		return
	}
	if req.Name == "" || len([]rune(req.Name)) > 120 {
		badRequest(c, "抽奖名称不能为空且不能超过 120 个字符")
		return
	}
	start, end, minimum, err := parseLotteryPeriod(req.PeriodStart, req.PeriodEnd, req.MinRecharge.StringFixed(2))
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if req.WinnerCount < 1 || req.WinnerCount > 1000 {
		badRequest(c, "中奖人数必须在 1 到 1000 之间")
		return
	}
	if !req.PrizeAmount.Decimal.IsPositive() || req.PrizeAmount.Cmp(decimal.NewFromInt(1000000)) > 0 {
		badRequest(c, "每人中奖金额必须大于 0 且不能超过 1000000")
		return
	}
	if existing, err := app.loadLotteryDrawByRequestID(req.RequestID); err == nil {
		c.JSON(http.StatusOK, existing)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		serverError(c, err)
		return
	}

	candidates, err := app.loadLotteryCandidates(c.Request.Context(), start, end, minimum)
	if err != nil {
		respondSub2APIError(c, err)
		return
	}
	if req.WinnerCount > len(candidates) {
		conflict(c, fmt.Sprintf("符合条件的用户只有 %d 人，不能抽取 %d 人", len(candidates), req.WinnerCount))
		return
	}
	winnerIndexes, err := selectRandomCandidateIndexes(len(candidates), req.WinnerCount, rand.Reader)
	if err != nil {
		serverError(c, err)
		return
	}
	winnerSet := make(map[int]bool, len(winnerIndexes))
	for _, index := range winnerIndexes {
		winnerSet[index] = true
	}

	draw := LotteryDraw{RequestID: req.RequestID, Name: req.Name, PeriodStart: JSONTime{Time: start}, PeriodEnd: JSONTime{Time: end}, MinRecharge: minimum, EligibleCount: len(candidates), WinnerCount: req.WinnerCount, PrizeAmount: req.PrizeAmount}
	err = app.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&draw).Error; err != nil {
			return err
		}
		entries := make([]LotteryEntry, 0, len(candidates))
		for index, candidate := range candidates {
			awardStatus := lotteryAwardNone
			if winnerSet[index] {
				awardStatus = lotteryAwardPending
			}
			entries = append(entries, LotteryEntry{DrawID: draw.ID, UserID: candidate.UserID, UserName: candidate.UserName, UserEmail: candidate.UserEmail, RechargeAmount: candidate.RechargeAmount, OrderCount: candidate.OrderCount, Winner: winnerSet[index], AwardStatus: awardStatus})
		}
		return tx.CreateInBatches(entries, 500).Error
	})
	if err != nil {
		if isDuplicateEntry(err) {
			if existing, loadErr := app.loadLotteryDrawByRequestID(req.RequestID); loadErr == nil {
				c.JSON(http.StatusOK, existing)
				return
			}
		}
		handleDBError(c, err)
		return
	}
	app.awardLotteryWinners(c.Request.Context(), draw)
	response, err := app.loadLotteryDraw(draw.ID)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) retryLotteryAwards(c *gin.Context) {
	id, ok := pathUint64(c, "id")
	if !ok {
		return
	}
	var draw LotteryDraw
	if err := app.db.First(&draw, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "抽奖记录不存在"})
			return
		}
		serverError(c, err)
		return
	}
	app.awardLotteryWinners(c.Request.Context(), draw)
	response, err := app.loadLotteryDraw(draw.ID)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (app *App) awardLotteryWinners(ctx context.Context, draw LotteryDraw) {
	var entries []LotteryEntry
	staleBefore := time.Now().Add(-lotteryStaleAwardAfter)
	if err := app.db.Where("draw_id = ? AND winner = ? AND (award_status IN ? OR (award_status = ? AND updated_at < ?))", draw.ID, true, []string{lotteryAwardPending, lotteryAwardFailed}, lotteryAwardProcessing, staleBefore).Order("id ASC").Find(&entries).Error; err != nil {
		return
	}
	for _, entry := range entries {
		reserved := app.db.Model(&LotteryEntry{}).
			Where("id = ? AND (award_status IN ? OR (award_status = ? AND updated_at < ?))", entry.ID, []string{lotteryAwardPending, lotteryAwardFailed}, lotteryAwardProcessing, staleBefore).
			Updates(map[string]any{"award_status": lotteryAwardProcessing, "award_error": "", "updated_at": time.Now()})
		if reserved.Error != nil || reserved.RowsAffected != 1 {
			continue
		}
		key := fmt.Sprintf("lottery-%d-user-%d", draw.ID, entry.UserID)
		notes := fmt.Sprintf("Lottery prize: %s (#%d)", draw.Name, draw.ID)
		if err := app.addSub2APIUserBalance(ctx, entry.UserID, draw.PrizeAmount, key, notes); err != nil {
			message := app.describeLotteryAwardFailure(ctx, entry.UserID, draw.PrizeAmount, err)
			if len(message) > 1000 {
				message = message[:1000]
			}
			app.db.Model(&LotteryEntry{}).Where("id = ?", entry.ID).Updates(map[string]any{"award_status": lotteryAwardFailed, "award_error": message, "updated_at": time.Now()})
			continue
		}
		now := time.Now()
		app.db.Model(&LotteryEntry{}).Where("id = ?", entry.ID).Updates(map[string]any{"award_status": lotteryAwarded, "award_error": "", "awarded_at": now, "updated_at": now})
	}
}

func (app *App) describeLotteryAwardFailure(ctx context.Context, userID int64, prize Amount, awardErr error) string {
	var user struct {
		Balance float64 `json:"balance"`
	}
	path := fmt.Sprintf("/api/v1/admin/users/%d", userID)
	if err := app.sub2APIAdminJSON(ctx, http.MethodGet, path, nil, &user); err != nil {
		return awardErr.Error()
	}
	current := decimal.NewFromFloat(user.Balance)
	if current.IsNegative() && current.Add(prize.Decimal).IsNegative() {
		minimum := current.Abs().Mul(decimal.NewFromInt(100)).Ceil().Div(decimal.NewFromInt(100))
		return fmt.Sprintf("用户当前余额为 %s，本次发放 %s 后仍为负数；Sub2API 拒绝该余额调整，中奖金额至少需要 %s", current.String(), prize.StringFixed(2), minimum.StringFixed(2))
	}
	return awardErr.Error()
}

func (app *App) listLotteryDraws(c *gin.Context) {
	var draws []LotteryDraw
	if err := app.db.Order("created_at DESC, id DESC").Limit(50).Find(&draws).Error; err != nil {
		serverError(c, err)
		return
	}
	result := make([]LotteryDrawResponse, 0, len(draws))
	for _, draw := range draws {
		item, err := app.loadLotteryDraw(draw.ID)
		if err != nil {
			serverError(c, err)
			return
		}
		result = append(result, item)
	}
	c.JSON(http.StatusOK, result)
}

func parseLotteryPeriod(startText, endText, minimumText string) (time.Time, time.Time, Amount, error) {
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(startText))
	if err != nil {
		return time.Time{}, time.Time{}, Amount{}, errors.New("开始时间格式无效")
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(endText))
	if err != nil {
		return time.Time{}, time.Time{}, Amount{}, errors.New("结束时间格式无效")
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, Amount{}, errors.New("结束时间必须晚于开始时间")
	}
	if end.Sub(start) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, Amount{}, errors.New("统计时间范围不能超过 366 天")
	}
	if end.After(time.Now().Add(time.Minute)) {
		return time.Time{}, time.Time{}, Amount{}, errors.New("结束时间不能晚于当前时间")
	}
	minimum, err := ParseAmount(minimumText)
	if err != nil || !minimum.Decimal.IsPositive() {
		return time.Time{}, time.Time{}, Amount{}, errors.New("最低充值金额必须大于 0")
	}
	return start, end, minimum, nil
}

func (app *App) loadLotteryCandidates(ctx context.Context, start, end time.Time, minimum Amount) ([]lotteryCandidate, error) {
	orders, err := app.loadCompletedBalanceOrders(ctx)
	if err != nil {
		return nil, err
	}
	byUser := make(map[int64]*lotteryCandidate)
	for _, order := range orders {
		if order.UserID <= 0 || order.PaidAt == nil || order.PaidAt.Before(start) || !order.PaidAt.Before(end) {
			continue
		}
		candidate := byUser[order.UserID]
		if candidate == nil {
			candidate = &lotteryCandidate{UserID: order.UserID, UserName: strings.TrimSpace(order.UserName), UserEmail: strings.TrimSpace(order.UserEmail), RechargeAmount: Amount{Decimal: decimal.Zero}}
			byUser[order.UserID] = candidate
		}
		candidate.RechargeAmount = Amount{Decimal: candidate.RechargeAmount.Decimal.Add(decimal.NewFromFloat(order.Amount)).Round(2)}
		candidate.OrderCount++
	}
	result := make([]lotteryCandidate, 0, len(byUser))
	for _, candidate := range byUser {
		if candidate.RechargeAmount.Cmp(minimum.Decimal) >= 0 {
			result = append(result, *candidate)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UserID < result[j].UserID })
	return result, nil
}

func (app *App) loadCompletedBalanceOrders(ctx context.Context) ([]sub2APIPaymentOrder, error) {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" {
		return nil, businessConflict("Sub2API 未配置：请设置服务地址")
	}
	authName, authValue, err := app.sub2APIAuthHeader(ctx, cfg)
	if err != nil {
		return nil, err
	}
	orders := make([]sub2APIPaymentOrder, 0)
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	for page := 1; ; page++ {
		query := url.Values{"page": {fmt.Sprint(page)}, "page_size": {"100"}, "status": {"COMPLETED"}, "order_type": {"balance"}}
		requestCtx, cancel := context.WithTimeout(ctx, timeout)
		req, requestErr := http.NewRequestWithContext(requestCtx, http.MethodGet, cfg.BaseURL+"/api/v1/admin/payment/orders?"+query.Encode(), nil)
		if requestErr != nil {
			cancel()
			return nil, requestErr
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set(authName, authValue)
		resp, requestErr := http.DefaultClient.Do(req)
		if requestErr != nil {
			cancel()
			return nil, fmt.Errorf("查询 Sub2API 充值订单失败: %w", requestErr)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		cancel()
		if readErr != nil {
			return nil, readErr
		}
		var envelope sub2APIResponse[sub2APIPaginatedOrders]
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("解析 Sub2API 充值订单失败: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
			message := strings.TrimSpace(envelope.Message)
			if message == "" {
				message = resp.Status
			}
			return nil, fmt.Errorf("查询 Sub2API 充值订单失败: %s", message)
		}
		orders = append(orders, envelope.Data.Items...)
		if len(orders) > lotteryMaxOrders {
			return nil, businessConflict("成功充值订单超过 100000 条，当前无法保证抽奖数据完整")
		}
		if page >= envelope.Data.Pages || len(envelope.Data.Items) == 0 {
			break
		}
	}
	return orders, nil
}

func selectRandomCandidateIndexes(candidateCount, winnerCount int, source io.Reader) ([]int, error) {
	indexes := make([]int, candidateCount)
	for i := range indexes {
		indexes[i] = i
	}
	for i := 0; i < winnerCount; i++ {
		offset, err := rand.Int(source, big.NewInt(int64(candidateCount-i)))
		if err != nil {
			return nil, err
		}
		j := i + int(offset.Int64())
		indexes[i], indexes[j] = indexes[j], indexes[i]
	}
	return indexes[:winnerCount], nil
}

func (app *App) loadLotteryDrawByRequestID(requestID string) (LotteryDrawResponse, error) {
	var draw LotteryDraw
	if err := app.db.First(&draw, "request_id = ?", requestID).Error; err != nil {
		return LotteryDrawResponse{}, err
	}
	return app.loadLotteryDraw(draw.ID)
}

func (app *App) loadLotteryDraw(id uint64) (LotteryDrawResponse, error) {
	var draw LotteryDraw
	if err := app.db.First(&draw, "id = ?", id).Error; err != nil {
		return LotteryDrawResponse{}, err
	}
	var winners []LotteryEntry
	if err := app.db.Where("draw_id = ? AND winner = ?", id, true).Order("id ASC").Find(&winners).Error; err != nil {
		return LotteryDrawResponse{}, err
	}
	items := make([]LotteryWinnerResponse, 0, len(winners))
	for _, winner := range winners {
		items = append(items, LotteryWinnerResponse{
			LotteryCandidateResponse: LotteryCandidateResponse{UserID: winner.UserID, UserName: winner.UserName, UserEmail: winner.UserEmail, RechargeAmount: winner.RechargeAmount, OrderCount: winner.OrderCount},
			PrizeAmount:              draw.PrizeAmount, AwardStatus: winner.AwardStatus, AwardError: winner.AwardError, AwardedAt: winner.AwardedAt,
		})
	}
	return LotteryDrawResponse{ID: draw.ID, RequestID: draw.RequestID, Name: draw.Name, PeriodStart: draw.PeriodStart, PeriodEnd: draw.PeriodEnd, MinRecharge: draw.MinRecharge, EligibleCount: draw.EligibleCount, WinnerCount: draw.WinnerCount, PrizeAmount: draw.PrizeAmount, Winners: items, CreatedAt: draw.CreatedAt}, nil
}

func lotteryCandidateResponses(candidates []lotteryCandidate) []LotteryCandidateResponse {
	result := make([]LotteryCandidateResponse, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, LotteryCandidateResponse{UserID: candidate.UserID, UserName: candidate.UserName, UserEmail: candidate.UserEmail, RechargeAmount: candidate.RechargeAmount, OrderCount: candidate.OrderCount})
	}
	return result
}

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	userTokenRankingLimit         = 10
	userTokenRankingUpstreamLimit = 50
	userTokenRankingTimezone      = "Asia/Shanghai"
)

type sub2APIUserUsageTrendPoint struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}

type sub2APIUserUsageTrendResponse struct {
	Trend []sub2APIUserUsageTrendPoint `json:"trend"`
}

type UserTokenUsageRankingItem struct {
	Rank          int    `json:"rank"`
	DisplayName   string `json:"displayName"`
	Tokens        int64  `json:"tokens"`
	Requests      int64  `json:"requests"`
	IsCurrentUser bool   `json:"isCurrentUser"`
}

type UserTokenUsageRankingResponse struct {
	Enabled   bool                        `json:"enabled"`
	Date      string                      `json:"date"`
	Timezone  string                      `json:"timezone"`
	UpdatedAt string                      `json:"updatedAt"`
	Ranking   []UserTokenUsageRankingItem `json:"ranking"`
}

func (app *App) getUserTokenUsageRanking(c *gin.Context) {
	currentUser, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}
	enabled, err := app.tokenUsageRankingEnabled()
	if err != nil {
		serverError(c, err)
		return
	}
	if !enabled {
		c.JSON(http.StatusOK, UserTokenUsageRankingResponse{
			Enabled:  false,
			Timezone: userTokenRankingTimezone,
			Ranking:  []UserTokenUsageRankingItem{},
		})
		return
	}

	location, err := time.LoadLocation(userTokenRankingTimezone)
	if err != nil {
		serverError(c, err)
		return
	}
	now := time.Now().In(location)
	date := now.Format("2006-01-02")

	response, err := app.fetchUserTokenUsageRanking(c.Request.Context(), date, currentUser.ID)
	if err != nil {
		respondSub2APIError(c, err)
		return
	}
	response.UpdatedAt = now.Format(time.RFC3339)
	c.JSON(http.StatusOK, response)
}

func (app *App) fetchUserTokenUsageRanking(ctx context.Context, date string, currentUserID int64) (UserTokenUsageRankingResponse, error) {
	query := url.Values{}
	query.Set("start_date", date)
	query.Set("end_date", date)
	query.Set("timezone", userTokenRankingTimezone)
	query.Set("granularity", "day")
	query.Set("limit", fmt.Sprintf("%d", userTokenRankingUpstreamLimit))

	var upstream sub2APIUserUsageTrendResponse
	path := "/api/v1/admin/dashboard/users-trend?" + query.Encode()
	if err := app.sub2APIAdminJSON(ctx, http.MethodGet, path, nil, &upstream); err != nil {
		return UserTokenUsageRankingResponse{}, err
	}

	return UserTokenUsageRankingResponse{
		Enabled:  true,
		Date:     date,
		Timezone: userTokenRankingTimezone,
		Ranking:  normalizeUserTokenUsageRanking(upstream.Trend, currentUserID, userTokenRankingLimit),
	}, nil
}

func (app *App) tokenUsageRankingEnabled() (bool, error) {
	value, found, err := app.getSetting(tokenUsageRankingKey)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return parseBoolSetting(value, true), nil
}

func normalizeUserTokenUsageRanking(points []sub2APIUserUsageTrendPoint, currentUserID int64, limit int) []UserTokenUsageRankingItem {
	if limit <= 0 {
		limit = userTokenRankingLimit
	}

	byUserID := make(map[int64]sub2APIUserUsageTrendPoint, len(points))
	for _, point := range points {
		if point.UserID <= 0 {
			continue
		}
		if point.Tokens < 0 {
			point.Tokens = 0
		}
		if point.Requests < 0 {
			point.Requests = 0
		}
		existing := byUserID[point.UserID]
		existing.UserID = point.UserID
		existing.Tokens += point.Tokens
		existing.Requests += point.Requests
		if strings.TrimSpace(existing.Username) == "" {
			existing.Username = point.Username
		}
		if strings.TrimSpace(existing.Email) == "" {
			existing.Email = point.Email
		}
		byUserID[point.UserID] = existing
	}

	rows := make([]sub2APIUserUsageTrendPoint, 0, len(byUserID))
	for _, point := range byUserID {
		rows = append(rows, point)
	}
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].Tokens != rows[right].Tokens {
			return rows[left].Tokens > rows[right].Tokens
		}
		if rows[left].Requests != rows[right].Requests {
			return rows[left].Requests > rows[right].Requests
		}
		return rows[left].UserID < rows[right].UserID
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}

	ranking := make([]UserTokenUsageRankingItem, 0, len(rows))
	for index, row := range rows {
		ranking = append(ranking, UserTokenUsageRankingItem{
			Rank:          index + 1,
			DisplayName:   maskedRankingIdentity(row.Username, row.Email, row.UserID),
			Tokens:        row.Tokens,
			Requests:      row.Requests,
			IsCurrentUser: row.UserID == currentUserID,
		})
	}
	return ranking
}

func maskedRankingIdentity(username, email string, userID int64) string {
	if value := strings.TrimSpace(username); value != "" {
		return maskText(value)
	}
	if value := strings.TrimSpace(email); value != "" {
		parts := strings.SplitN(value, "@", 2)
		if len(parts) == 2 && parts[1] != "" {
			return maskText(parts[0]) + "@" + parts[1]
		}
		return maskText(value)
	}
	return fmt.Sprintf("用户 #%d", userID)
}

func maskText(value string) string {
	value = strings.TrimSpace(value)
	count := utf8.RuneCountInString(value)
	if count <= 1 {
		return value + "***"
	}
	runes := []rune(value)
	if count == 2 {
		return string(runes[0]) + "***"
	}
	return string(runes[0]) + "***" + string(runes[count-1])
}

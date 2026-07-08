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
	"gorm.io/gorm/clause"
)

const (
	groupRateSourcePolling       = "polling"
	groupRateSourceManualRefresh = "manual_refresh"
	defaultGroupRateRefreshSec   = 300
	minGroupRateRefreshSec       = 60
	maxGroupRateRefreshSec       = 86400
)

type Sub2APIGroupRateMonitorSettings struct {
	Enabled                bool     `json:"enabled"`
	RefreshIntervalSeconds int      `json:"refreshIntervalSeconds"`
	MonitoredGroupIDs      []string `json:"monitoredGroupIds"`
	PublicGroupIDs         []string `json:"publicGroupIds"`
}

type Sub2APIGroupRateMonitorResponse struct {
	Settings Sub2APIGroupRateMonitorSettings `json:"settings"`
	Groups   []Sub2APIGroupRateGroupResponse `json:"groups"`
	Series   []Sub2APIGroupRateSeries        `json:"series"`
}

type Sub2APIGroupRateGroupResponse struct {
	GroupID        string  `json:"groupId"`
	GroupName      string  `json:"groupName"`
	RateMultiplier float64 `json:"rateMultiplier"`
	Monitored      bool    `json:"monitored"`
	PublicVisible  bool    `json:"publicVisible"`
	LastSeenAt     string  `json:"lastSeenAt"`
}

type Sub2APIGroupRateSeries struct {
	GroupID       string                  `json:"groupId"`
	GroupName     string                  `json:"groupName"`
	PublicVisible bool                    `json:"publicVisible"`
	Points        []Sub2APIGroupRatePoint `json:"points"`
}

type Sub2APIGroupRatePoint struct {
	Time string  `json:"time"`
	Rate float64 `json:"rate"`
}

type sub2APIGroupRate struct {
	GroupID        string
	GroupName      string
	RateMultiplier decimal.Decimal
	RawJSON        string
}

func (app *App) getSub2APIGroupRateMonitor(c *gin.Context) {
	settings, err := app.loadSub2APIGroupRateMonitorSettings()
	if err != nil {
		serverError(c, err)
		return
	}
	groups, err := app.listSub2APIGroupRateGroups(settings)
	if err != nil {
		serverError(c, err)
		return
	}
	series, err := app.buildSub2APIGroupRateSeries(settings, false, 30)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, Sub2APIGroupRateMonitorResponse{Settings: settings, Groups: groups, Series: series})
}

func (app *App) updateSub2APIGroupRateMonitor(c *gin.Context) {
	var req Sub2APIGroupRateMonitorSettings
	if !bindJSON(c, &req) {
		return
	}
	settings := normalizeSub2APIGroupRateMonitorSettings(req)
	if err := app.saveSub2APIGroupRateMonitorSettings(settings); err != nil {
		serverError(c, err)
		return
	}
	groups, err := app.listSub2APIGroupRateGroups(settings)
	if err != nil {
		serverError(c, err)
		return
	}
	series, err := app.buildSub2APIGroupRateSeries(settings, false, 30)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, Sub2APIGroupRateMonitorResponse{Settings: settings, Groups: groups, Series: series})
}

func (app *App) refreshSub2APIGroupRates(c *gin.Context) {
	if err := app.syncSub2APIGroupRates(c.Request.Context(), groupRateSourceManualRefresh); err != nil {
		respondSub2APIError(c, err)
		return
	}
	app.getSub2APIGroupRateMonitor(c)
}

func (app *App) getPublicSub2APIGroupRateSeries(c *gin.Context) {
	settings, err := app.loadSub2APIGroupRateMonitorSettings()
	if err != nil {
		serverError(c, err)
		return
	}
	series, err := app.buildSub2APIGroupRateSeries(settings, true, 30)
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, series)
}

func (app *App) startSub2APIGroupRateMonitor(ctx context.Context) {
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				settings, err := app.loadSub2APIGroupRateMonitorSettings()
				if err != nil {
					log.Printf("Sub2API group rate monitor settings failed: %v", err)
					timer.Reset(time.Duration(defaultGroupRateRefreshSec) * time.Second)
					continue
				}
				if settings.Enabled {
					if err := app.syncSub2APIGroupRates(ctx, groupRateSourcePolling); err != nil {
						log.Printf("Sub2API group rate monitor sync failed: %v", err)
					}
				}
				timer.Reset(time.Duration(settings.RefreshIntervalSeconds) * time.Second)
			}
		}
	}()
}

func (app *App) syncSub2APIGroupRates(ctx context.Context, source string) error {
	app.sub2APIGroupRateMonitorMu.Lock()
	defer app.sub2APIGroupRateMonitorMu.Unlock()

	settings, err := app.loadSub2APIGroupRateMonitorSettings()
	if err != nil {
		return err
	}
	if source == groupRateSourcePolling && !settings.Enabled {
		return nil
	}

	groups, err := app.fetchSub2APIGroupRates(ctx)
	if err != nil {
		return err
	}
	monitored := stringSet(settings.MonitoredGroupIDs)
	public := stringSet(settings.PublicGroupIDs)
	monitorAll := len(monitored) == 0
	now := JSONTime{Time: time.Now()}

	for _, group := range groups {
		var snapshot Sub2APIGroupRateSnapshot
		tx := app.db.Where("group_id = ?", group.GroupID).Limit(1).Find(&snapshot)
		if tx.Error != nil {
			return tx.Error
		}
		exists := tx.RowsAffected > 0
		changed := exists && !snapshot.RateMultiplier.Equal(group.RateMultiplier)

		if changed && (monitorAll || monitored[group.GroupID]) {
			logEntry := Sub2APIGroupRateLog{
				GroupID:       group.GroupID,
				GroupName:     group.GroupName,
				OldRate:       snapshot.RateMultiplier,
				NewRate:       group.RateMultiplier,
				Source:        source,
				PublicVisible: public[group.GroupID],
				CreatedAt:     now,
			}
			if err := app.db.Create(&logEntry).Error; err != nil {
				return err
			}
		}

		nextSnapshot := Sub2APIGroupRateSnapshot{
			GroupID:        group.GroupID,
			GroupName:      group.GroupName,
			RateMultiplier: group.RateMultiplier,
			RawJSON:        group.RawJSON,
			LastSeenAt:     now,
		}
		if err := app.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "group_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"group_name":      nextSnapshot.GroupName,
				"rate_multiplier": nextSnapshot.RateMultiplier,
				"raw_json":        nextSnapshot.RawJSON,
				"last_seen_at":    nextSnapshot.LastSeenAt.Time,
				"updated_at":      time.Now(),
			}),
		}).Create(&nextSnapshot).Error; err != nil {
			return err
		}
	}
	return nil
}

func (app *App) fetchSub2APIGroupRates(ctx context.Context) ([]sub2APIGroupRate, error) {
	var raw json.RawMessage
	if err := app.sub2APIAdminJSON(ctx, http.MethodGet, "/api/v1/admin/groups/all", nil, &raw); err != nil {
		return nil, err
	}
	return parseSub2APIGroupRates(raw)
}

func (app *App) sub2APIAdminJSON(ctx context.Context, method, path string, payload any, out any) error {
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

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, method, cfg.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(authName, authValue)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Sub2API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("parse Sub2API response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = resp.Status
		}
		return upstreamAPIError{statusCode: resp.StatusCode, message: message}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func (app *App) loadSub2APIGroupRateMonitorSettings() (Sub2APIGroupRateMonitorSettings, error) {
	raw, found, err := app.getSetting(sub2APIGroupRateMonitorKey)
	if err != nil {
		return Sub2APIGroupRateMonitorSettings{}, err
	}
	if !found || strings.TrimSpace(raw) == "" {
		return defaultSub2APIGroupRateMonitorSettings(), nil
	}
	var settings Sub2APIGroupRateMonitorSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return defaultSub2APIGroupRateMonitorSettings(), nil
	}
	return normalizeSub2APIGroupRateMonitorSettings(settings), nil
}

func (app *App) saveSub2APIGroupRateMonitorSettings(settings Sub2APIGroupRateMonitorSettings) error {
	encoded, err := json.Marshal(normalizeSub2APIGroupRateMonitorSettings(settings))
	if err != nil {
		return err
	}
	return app.saveSetting(sub2APIGroupRateMonitorKey, string(encoded))
}

func defaultSub2APIGroupRateMonitorSettings() Sub2APIGroupRateMonitorSettings {
	return Sub2APIGroupRateMonitorSettings{
		Enabled:                true,
		RefreshIntervalSeconds: defaultGroupRateRefreshSec,
		MonitoredGroupIDs:      []string{},
		PublicGroupIDs:         []string{},
	}
}

func normalizeSub2APIGroupRateMonitorSettings(settings Sub2APIGroupRateMonitorSettings) Sub2APIGroupRateMonitorSettings {
	if settings.RefreshIntervalSeconds < minGroupRateRefreshSec {
		settings.RefreshIntervalSeconds = minGroupRateRefreshSec
	}
	if settings.RefreshIntervalSeconds > maxGroupRateRefreshSec {
		settings.RefreshIntervalSeconds = maxGroupRateRefreshSec
	}
	settings.MonitoredGroupIDs = normalizeIDList(settings.MonitoredGroupIDs)
	settings.PublicGroupIDs = normalizeIDList(settings.PublicGroupIDs)
	return settings
}

func (app *App) listSub2APIGroupRateGroups(settings Sub2APIGroupRateMonitorSettings) ([]Sub2APIGroupRateGroupResponse, error) {
	var snapshots []Sub2APIGroupRateSnapshot
	if err := app.db.Order("group_name ASC, group_id ASC").Find(&snapshots).Error; err != nil {
		return nil, err
	}
	monitored := stringSet(settings.MonitoredGroupIDs)
	public := stringSet(settings.PublicGroupIDs)
	monitorAll := len(monitored) == 0
	groups := make([]Sub2APIGroupRateGroupResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		groups = append(groups, Sub2APIGroupRateGroupResponse{
			GroupID:        snapshot.GroupID,
			GroupName:      snapshot.GroupName,
			RateMultiplier: decimalToFloat(snapshot.RateMultiplier),
			Monitored:      monitorAll || monitored[snapshot.GroupID],
			PublicVisible:  public[snapshot.GroupID],
			LastSeenAt:     formatJSONTime(snapshot.LastSeenAt),
		})
	}
	return groups, nil
}

func (app *App) buildSub2APIGroupRateSeries(settings Sub2APIGroupRateMonitorSettings, publicOnly bool, days int) ([]Sub2APIGroupRateSeries, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	windowStart := time.Now().AddDate(0, 0, -days)
	var snapshots []Sub2APIGroupRateSnapshot
	if err := app.db.Order("group_name ASC, group_id ASC").Find(&snapshots).Error; err != nil {
		return nil, err
	}

	monitored := stringSet(settings.MonitoredGroupIDs)
	public := stringSet(settings.PublicGroupIDs)
	monitorAll := len(monitored) == 0
	seriesByID := map[string]*Sub2APIGroupRateSeries{}
	groupIDs := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if publicOnly {
			if !public[snapshot.GroupID] {
				continue
			}
		} else if !monitorAll && !monitored[snapshot.GroupID] {
			continue
		}
		groupIDs = append(groupIDs, snapshot.GroupID)
		seriesByID[snapshot.GroupID] = &Sub2APIGroupRateSeries{
			GroupID:       snapshot.GroupID,
			GroupName:     snapshot.GroupName,
			PublicVisible: public[snapshot.GroupID],
			Points: []Sub2APIGroupRatePoint{{
				Time: formatJSONTime(snapshot.LastSeenAt),
				Rate: decimalToFloat(snapshot.RateMultiplier),
			}},
		}
	}
	if len(groupIDs) == 0 {
		return []Sub2APIGroupRateSeries{}, nil
	}

	var logs []Sub2APIGroupRateLog
	query := app.db.Where("group_id IN ? AND created_at >= ?", groupIDs, windowStart).
		Order("group_id ASC, created_at ASC, id ASC")
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	for _, entry := range logs {
		series := seriesByID[entry.GroupID]
		if series == nil {
			continue
		}
		if len(series.Points) == 1 {
			series.Points = []Sub2APIGroupRatePoint{{
				Time: windowStart.Format("2006-01-02 15:04:05"),
				Rate: decimalToFloat(entry.OldRate),
			}}
		}
		appendGroupRatePoint(series, Sub2APIGroupRatePoint{
			Time: formatJSONTime(entry.CreatedAt),
			Rate: decimalToFloat(entry.NewRate),
		})
	}

	series := make([]Sub2APIGroupRateSeries, 0, len(seriesByID))
	for _, item := range seriesByID {
		series = append(series, *item)
	}
	sort.Slice(series, func(i, j int) bool {
		return series[i].GroupName < series[j].GroupName
	})
	return series, nil
}

func appendGroupRatePoint(series *Sub2APIGroupRateSeries, point Sub2APIGroupRatePoint) {
	if len(series.Points) > 0 && series.Points[len(series.Points)-1].Time == point.Time {
		series.Points[len(series.Points)-1] = point
		return
	}
	series.Points = append(series.Points, point)
}

func parseSub2APIGroupRates(raw json.RawMessage) ([]sub2APIGroupRate, error) {
	items, err := sub2APIGroupArray(raw)
	if err != nil {
		return nil, err
	}
	groups := make([]sub2APIGroupRate, 0, len(items))
	for _, item := range items {
		group, err := parseSub2APIGroupRate(item)
		if err != nil {
			continue
		}
		groups = append(groups, group)
	}
	if len(groups) == 0 && len(items) > 0 {
		return nil, errors.New("Sub2API groups response does not contain recognizable rate fields")
	}
	return groups, nil
}

func sub2APIGroupArray(raw json.RawMessage) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return items, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	for _, key := range []string{"groups", "items", "content", "list", "data"} {
		if value, ok := obj[key]; ok {
			if err := json.Unmarshal(value, &items); err == nil {
				return items, nil
			}
		}
	}
	return nil, errors.New("Sub2API groups response is not an array")
}

func parseSub2APIGroupRate(raw json.RawMessage) (sub2APIGroupRate, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return sub2APIGroupRate{}, err
	}
	id := firstStringish(obj, "id", "group_id", "groupId", "key")
	name := firstStringish(obj, "name", "group_name", "groupName", "title")
	rate, ok := firstDecimal(obj, "rate_multiplier", "rateMultiplier", "multiplier", "rate", "quota_multiplier", "quotaMultiplier")
	if id == "" || !ok {
		return sub2APIGroupRate{}, errors.New("missing group id or rate")
	}
	if name == "" {
		name = id
	}
	return sub2APIGroupRate{
		GroupID:        id,
		GroupName:      name,
		RateMultiplier: rate,
		RawJSON:        string(raw),
	}, nil
}

func firstStringish(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			return strings.TrimSpace(v)
		case float64:
			if v == float64(int64(v)) {
				return strconv.FormatInt(int64(v), 10)
			}
			return strconv.FormatFloat(v, 'f', -1, 64)
		case json.Number:
			return v.String()
		default:
			text := strings.TrimSpace(fmt.Sprint(v))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func firstDecimal(obj map[string]any, keys ...string) (decimal.Decimal, bool) {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			parsed, err := decimal.NewFromString(strings.TrimSpace(v))
			if err == nil {
				return parsed, true
			}
		case float64:
			return decimal.NewFromFloat(v), true
		case json.Number:
			parsed, err := decimal.NewFromString(v.String())
			if err == nil {
				return parsed, true
			}
		default:
			parsed, err := decimal.NewFromString(strings.TrimSpace(fmt.Sprint(v)))
			if err == nil {
				return parsed, true
			}
		}
	}
	return decimal.Zero, false
}

func normalizeIDList(values []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func decimalToFloat(value decimal.Decimal) float64 {
	result, _ := value.Float64()
	return result
}

func formatJSONTime(value JSONTime) string {
	if value.Time.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

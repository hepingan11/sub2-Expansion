package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (app *App) generateSub2APIRedeemCode(ctx context.Context, userID string, today LocalDate, amount Amount) (string, error) {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		return "", err
	}
	if cfg.BaseURL == "" {
		return "", businessConflict("Sub2API 未配置：请设置 SUB2API_BASE_URL")
	}
	authName, authValue, err := app.sub2APIAuthHeader(ctx, cfg)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"count": 1,
		"type":  "balance",
		"value": amount.InexactFloat64(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := cfg.BaseURL + "/api/v1/admin/redeem-codes/generate"
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Idempotency-Key", sub2APIIdempotencyKey(userID, today, amount))
	req.Header.Set(authName, authValue)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("生成 Sub2API 兑换码失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var envelope sub2APIResponse[[]sub2APIRedeemCode]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("解析 Sub2API 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("生成 Sub2API 兑换码失败: %s", message)
	}
	if len(envelope.Data) == 0 || strings.TrimSpace(envelope.Data[0].Code) == "" {
		return "", errors.New("生成 Sub2API 兑换码失败: 响应中没有兑换码")
	}
	return strings.TrimSpace(envelope.Data[0].Code), nil
}

func (app *App) sub2APIAuthHeader(ctx context.Context, cfg Sub2APIConfig) (string, string, error) {
	switch normalizeSub2APIAuthMode(cfg.AuthMode) {
	case "admin_api_key":
		if cfg.AdminAPIKey == "" {
			return "", "", businessConflict("Sub2API 未配置：当前认证方式需要 Admin API Key")
		}
		return "x-api-key", cfg.AdminAPIKey, nil
	default:
		token, err := app.sub2APILogin(ctx, cfg)
		if err != nil {
			return "", "", err
		}
		return "Authorization", "Bearer " + token, nil
	}
}

func (app *App) startSub2APITokenRefresher(ctx context.Context) {
	if !app.cfg.Sub2APIRefreshToken {
		return
	}
	interval := app.cfg.Sub2APIRefreshEvery
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	go func() {
		app.refreshSub2APITokenOnce(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				app.refreshSub2APITokenOnce(ctx)
			}
		}
	}()
}

func (app *App) refreshSub2APITokenOnce(ctx context.Context) {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		log.Printf("Sub2API token refresh skipped: %v", err)
		return
	}
	if normalizeSub2APIAuthMode(cfg.AuthMode) != "password" {
		return
	}
	if cfg.BaseURL == "" || cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return
	}
	if _, err := app.sub2APILogin(ctx, cfg); err != nil {
		log.Printf("Sub2API token refresh failed: %v", err)
	}
}

func (app *App) sub2APILogin(ctx context.Context, cfg Sub2APIConfig) (string, error) {
	if cfg.BaseURL == "" {
		return "", businessConflict("Sub2API 未配置：请设置 SUB2API_BASE_URL")
	}
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return "", businessConflict("Sub2API 未配置：请设置 SUB2API_ADMIN_API_KEY，或 SUB2API_ADMIN_EMAIL/SUB2API_ADMIN_PASSWORD")
	}

	app.sub2APITokenMu.Lock()
	defer app.sub2APITokenMu.Unlock()
	if app.sub2APIToken != "" && time.Now().Before(app.sub2APITokenExpiry.Add(-sub2APITokenRefreshBefore)) {
		return app.sub2APIToken, nil
	}
	if token, expiresAt, ok := app.loadStoredSub2APIToken(); ok && time.Now().Before(expiresAt.Add(-sub2APITokenRefreshBefore)) {
		app.sub2APIToken = token
		app.sub2APITokenExpiry = expiresAt
		return token, nil
	}

	payload := map[string]string{
		"email":    cfg.AdminEmail,
		"password": cfg.AdminPassword,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, cfg.BaseURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("登录 Sub2API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var envelope sub2APIResponse[sub2APILoginData]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("解析 Sub2API 登录响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("登录 Sub2API 失败: %s", message)
	}
	if strings.TrimSpace(envelope.Data.AccessToken) == "" {
		return "", errors.New("登录 Sub2API 失败: 响应中没有 access_token")
	}

	expiresIn := envelope.Data.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	app.sub2APIToken = strings.TrimSpace(envelope.Data.AccessToken)
	app.sub2APITokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	if err := app.saveSub2APIToken(app.sub2APIToken, app.sub2APITokenExpiry); err != nil {
		log.Printf("save Sub2API token failed: %v", err)
	}
	return app.sub2APIToken, nil
}

func (app *App) loadStoredSub2APIToken() (string, time.Time, bool) {
	token, found, err := app.getSetting(sub2APIAccessTokenKey)
	if err != nil {
		log.Printf("load Sub2API token failed: %v", err)
		return "", time.Time{}, false
	}
	if !found || strings.TrimSpace(token) == "" {
		return "", time.Time{}, false
	}
	rawExpiresAt, found, err := app.getSetting(sub2APITokenExpiresAtKey)
	if err != nil {
		log.Printf("load Sub2API token expiry failed: %v", err)
		return "", time.Time{}, false
	}
	if !found {
		return "", time.Time{}, false
	}
	expiresAt, err := parseSub2APITokenExpiry(rawExpiresAt)
	if err != nil {
		log.Printf("parse Sub2API token expiry failed: %v", err)
		return "", time.Time{}, false
	}
	return strings.TrimSpace(token), expiresAt, true
}

func (app *App) saveSub2APIToken(token string, expiresAt time.Time) error {
	if strings.TrimSpace(token) == "" || expiresAt.IsZero() {
		return nil
	}
	if err := app.saveSetting(sub2APIAccessTokenKey, strings.TrimSpace(token)); err != nil {
		return err
	}
	return app.saveSetting(sub2APITokenExpiresAtKey, strconv.FormatInt(expiresAt.Unix(), 10))
}

func parseSub2APITokenExpiry(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty token expiry")
	}
	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unixSeconds, 0), nil
	}
	return time.Parse(time.RFC3339, value)
}

func (app *App) clearSub2APIToken() {
	app.sub2APITokenMu.Lock()
	defer app.sub2APITokenMu.Unlock()
	app.sub2APIToken = ""
	app.sub2APITokenExpiry = time.Time{}
}

func sub2APIIdempotencyKey(userID string, today LocalDate, amount Amount) string {
	sum := sha256.Sum256([]byte(userID))
	return fmt.Sprintf("checkin-%s-%x-%s", today.Format("2006-01-02"), sum[:8], amount.StringFixed(2))
}

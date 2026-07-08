package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type UserLoginRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstile_token"`
}

type UserRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UserLogin2FARequest struct {
	TempToken string `json:"temp_token"`
	TotpCode  string `json:"totp_code"`
}

type Sub2APIUserLoginData struct {
	AccessToken     string          `json:"access_token,omitempty"`
	RefreshToken    string          `json:"refresh_token,omitempty"`
	ExpiresIn       int             `json:"expires_in,omitempty"`
	TokenType       string          `json:"token_type,omitempty"`
	User            json.RawMessage `json:"user,omitempty"`
	Requires2FA     bool            `json:"requires_2fa,omitempty"`
	TempToken       string          `json:"temp_token,omitempty"`
	UserEmailMasked string          `json:"user_email_masked,omitempty"`
}

type Sub2APIUserRefreshData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type sub2APIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type upstreamAPIError struct {
	statusCode int
	message    string
}

func (e upstreamAPIError) Error() string {
	return e.message
}

func (app *App) userLogin(c *gin.Context) {
	var req UserLoginRequest
	if !bindJSON(c, &req) {
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || strings.TrimSpace(req.Password) == "" {
		badRequest(c, "email/password must not be blank")
		return
	}

	result, err := app.sub2APIUserLogin(c.Request.Context(), req)
	if err != nil {
		respondSub2APIError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (app *App) userLogin2FA(c *gin.Context) {
	var req UserLogin2FARequest
	if !bindJSON(c, &req) {
		return
	}
	req.TempToken = strings.TrimSpace(req.TempToken)
	req.TotpCode = strings.TrimSpace(req.TotpCode)
	if req.TempToken == "" || req.TotpCode == "" {
		badRequest(c, "temp_token/totp_code must not be blank")
		return
	}

	result, err := app.sub2APIUserLogin2FA(c.Request.Context(), req)
	if err != nil {
		respondSub2APIError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (app *App) refreshUserToken(c *gin.Context) {
	var req UserRefreshRequest
	if !bindJSON(c, &req) {
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		badRequest(c, "refresh_token must not be blank")
		return
	}

	result, err := app.sub2APIUserRefresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		respondSub2APIError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (app *App) userAuth(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Next()
		return
	}
	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, APIError{Message: "Missing user token"})
		return
	}

	user, err := app.sub2APIUserMe(c.Request.Context(), token)
	if err != nil {
		status := http.StatusUnauthorized
		var upstreamErr upstreamAPIError
		if errors.As(err, &upstreamErr) {
			if upstreamErr.statusCode >= 500 {
				status = http.StatusBadGateway
			} else if upstreamErr.statusCode >= 400 {
				status = upstreamErr.statusCode
			}
		}
		c.AbortWithStatusJSON(status, APIError{Message: err.Error()})
		return
	}
	c.Set("sub2apiUser", user)
	c.Next()
}

func (app *App) getCurrentSub2APIUser(c *gin.Context) {
	value, ok := c.Get("sub2apiUser")
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Missing user token"})
		return
	}
	user, ok := value.(json.RawMessage)
	if !ok || len(user) == 0 {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}

	snapshot, ok := sub2APIUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, APIError{Message: "Invalid user token"})
		return
	}

	var profile map[string]any
	if err := json.Unmarshal(user, &profile); err != nil {
		serverError(c, err)
		return
	}

	bindings, err := app.listSocialBindingsForUser(snapshot.ID)
	if err != nil {
		serverError(c, err)
		return
	}
	profile["socialBindings"] = bindings
	c.JSON(http.StatusOK, profile)
}

func (app *App) sub2APIUserLogin(ctx context.Context, req UserLoginRequest) (Sub2APIUserLoginData, error) {
	payload := map[string]string{
		"email":    req.Email,
		"password": req.Password,
	}
	if strings.TrimSpace(req.TurnstileToken) != "" {
		payload["turnstile_token"] = strings.TrimSpace(req.TurnstileToken)
	}

	var result Sub2APIUserLoginData
	err := app.sub2APIJSON(ctx, http.MethodPost, "/api/v1/auth/login", "", payload, &result)
	return result, err
}

func (app *App) sub2APIUserLogin2FA(ctx context.Context, req UserLogin2FARequest) (Sub2APIUserLoginData, error) {
	var result Sub2APIUserLoginData
	err := app.sub2APIJSON(ctx, http.MethodPost, "/api/v1/auth/login/2fa", "", map[string]string{
		"temp_token": req.TempToken,
		"totp_code":  req.TotpCode,
	}, &result)
	return result, err
}

func (app *App) sub2APIUserRefresh(ctx context.Context, refreshToken string) (Sub2APIUserRefreshData, error) {
	var result Sub2APIUserRefreshData
	err := app.sub2APIJSON(ctx, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": refreshToken,
	}, &result)
	return result, err
}

func (app *App) sub2APIUserMe(ctx context.Context, accessToken string) (json.RawMessage, error) {
	var result json.RawMessage
	err := app.sub2APIJSON(ctx, http.MethodGet, "/api/v1/auth/me", accessToken, nil, &result)
	return result, err
}

func (app *App) sub2APIJSON(ctx context.Context, method, path, bearer string, payload any, out any) error {
	cfg, err := app.effectiveSub2APIConfig()
	if err != nil {
		return err
	}
	if cfg.BaseURL == "" {
		return businessConflict("Sub2API is not configured: set SUB2API_BASE_URL")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, method, cfg.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	if strings.TrimSpace(bearer) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearer))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Sub2API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
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
		status := resp.StatusCode
		if status < 400 {
			status = http.StatusBadRequest
		}
		return upstreamAPIError{statusCode: status, message: message}
	}

	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func bearerToken(authHeader string) string {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
}

func respondSub2APIError(c *gin.Context, err error) {
	if isBusinessConflict(err) {
		conflict(c, err.Error())
		return
	}
	var upstreamErr upstreamAPIError
	if errors.As(err, &upstreamErr) {
		status := upstreamErr.statusCode
		if status <= 0 {
			status = http.StatusBadGateway
		}
		c.JSON(status, APIError{Message: upstreamErr.message})
		return
	}
	serverError(c, err)
}

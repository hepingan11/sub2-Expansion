package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (app *App) login(c *gin.Context) {
	var req AdminLoginRequest
	if !bindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		badRequest(c, "username/password must not be blank")
		return
	}
	if req.Username != app.cfg.AdminUsername || !app.matchesPassword(req.Password, app.cfg.AdminPassword) {
		badRequest(c, "用户名或密码错误")
		return
	}
	c.JSON(http.StatusOK, AdminLoginResponse{Token: app.issueToken(req.Username), ExpiresInHours: app.cfg.AuthTokenTTLHours})
}

func (app *App) adminAuth(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Next()
		return
	}
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, APIError{Message: "Missing admin token"})
		return
	}
	if !app.verifyToken(strings.TrimPrefix(auth, "Bearer ")) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, APIError{Message: "Invalid admin token"})
		return
	}
	c.Next()
}

func (app *App) issueToken(username string) string {
	expiresAt := time.Now().Add(time.Duration(app.cfg.AuthTokenTTLHours) * time.Hour).Unix()
	payload := base64.RawURLEncoding.EncodeToString([]byte(username)) + "." + strconv.FormatInt(expiresAt, 10)
	return payload + "." + app.sign(payload)
}

func (app *App) verifyToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(app.sign(payload)), []byte(parts[2])) {
		return false
	}
	expiresAt, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && expiresAt > time.Now().Unix()
}

func (app *App) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(app.cfg.AuthSecret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (app *App) matchesPassword(rawPassword, configuredPassword string) bool {
	if strings.TrimSpace(configuredPassword) == "" {
		return false
	}
	if strings.HasPrefix(configuredPassword, "$2a$") || strings.HasPrefix(configuredPassword, "$2b$") || strings.HasPrefix(configuredPassword, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(configuredPassword), []byte(rawPassword)) == nil
	}
	return rawPassword == configuredPassword
}

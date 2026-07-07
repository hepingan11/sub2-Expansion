package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func loadConfig() Config {
	return Config{
		Port:                 env("SERVER_PORT", env("PORT", "8080")),
		DBURL:                env("DB_URL", "postgres://postgres:postgres@localhost:5432/redeem_code_system?sslmode=disable&TimeZone=Asia/Shanghai"),
		DBUsername:           env("DB_USERNAME", "postgres"),
		DBPassword:           env("DB_PASSWORD", "postgres"),
		AdminUsername:        env("ADMIN_USERNAME", "admin"),
		AdminPassword:        env("ADMIN_PASSWORD", "admin123"),
		AuthSecret:           env("AUTH_SECRET", "change-this-secret-to-a-long-random-string"),
		AuthTokenTTLHours:    envInt64("AUTH_TOKEN_TTL_HOURS", 12),
		CorsAllowedOrigins:   splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://8.137.103.102:5173")),
		CheckInDailyMaxUsers: envInt("CHECK_IN_DAILY_MAX_USERS", 20),
		Sub2APIBaseURL:       strings.TrimRight(env("SUB2API_BASE_URL", ""), "/"),
		Sub2APIAdminAPIKey:   env("SUB2API_ADMIN_API_KEY", ""),
		Sub2APIAdminEmail:    env("SUB2API_ADMIN_EMAIL", env("SUB2API_ADMIN_USERNAME", "")),
		Sub2APIAdminPassword: env("SUB2API_ADMIN_PASSWORD", ""),
		Sub2APITimeout:       time.Duration(envInt("SUB2API_TIMEOUT_SECONDS", 15)) * time.Second,
		Sub2APIRefreshToken:  envBool("SUB2API_TOKEN_REFRESH_ENABLED", true),
		Sub2APIRefreshEvery:  time.Duration(envInt("SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS", 300)) * time.Second,
	}
}

func loadDotEnv() {
	loaded := map[string]bool{}
	for _, path := range dotenvPaths() {
		cleanPath := filepath.Clean(path)
		if loaded[cleanPath] {
			continue
		}
		loaded[cleanPath] = true
		if err := loadDotEnvFile(cleanPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("load %s: %v", cleanPath, err)
		}
	}
}

func dotenvPaths() []string {
	paths := []string{".env", filepath.Join("backend-go", ".env")}
	if exePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), ".env"))
	}
	return paths
}

func loadDotEnvFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for index, line := range strings.Split(string(content), "\n") {
		key, value, ok, err := parseDotEnvLine(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("line %d: %w", index+1, err)
			}
		}
	}
	return nil
}

func parseDotEnvLine(line string) (key string, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	line = strings.TrimPrefix(line, "export ")
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false, fmt.Errorf("expected KEY=VALUE")
	}
	key = strings.TrimSpace(parts[0])
	if !isDotEnvKey(key) {
		return "", "", false, fmt.Errorf("invalid key %q", key)
	}
	value = strings.TrimSpace(parts[1])
	if len(value) >= 2 {
		quote := value[0]
		if quote == '"' || quote == '\'' {
			if value[len(value)-1] != quote {
				return "", "", false, fmt.Errorf("unterminated quoted value for %s", key)
			}
			value = value[1 : len(value)-1]
			if quote == '"' {
				if unquoted, unquoteErr := strconv.Unquote(`"` + value + `"`); unquoteErr == nil {
					value = unquoted
				}
			}
			return key, value, true, nil
		}
	}
	value = strings.TrimSpace(stripDotEnvComment(value))
	return key, value, true, nil
}

func isDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for index, char := range key {
		valid := char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || index > 0 && char >= '0' && char <= '9'
		if !valid {
			return false
		}
	}
	return true
}

func stripDotEnvComment(value string) string {
	for index, char := range value {
		if char == '#' && (index == 0 || value[index-1] == ' ' || value[index-1] == '\t') {
			return value[:index]
		}
	}
	return value
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, ""))
	if err != nil {
		return fallback
	}
	return value
}

func envInt64(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(env(key, ""), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(env(key, "")))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

package app

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

type Config struct {
	Port                  string
	DBURL                 string
	DBUsername            string
	DBPassword            string
	AdminUsername         string
	AdminPassword         string
	AuthSecret            string
	AuthTokenTTLHours     int64
	CorsAllowedOrigins    []string
	FrontendPublicURL     string
	CheckInDailyMaxUsers  int
	Sub2APIBaseURL        string
	Sub2APIAdminAPIKey    string
	Sub2APIAdminEmail     string
	Sub2APIAdminPassword  string
	Sub2APITimeout        time.Duration
	Sub2APIRefreshToken   bool
	Sub2APIRefreshEvery   time.Duration
	AppVersion            string
	GitHubRepository      string
	SystemUpdateCommand   string
	TelegramBotEnabled    bool
	TelegramBotToken      string
	TelegramBotAPIBaseURL string
	TelegramBotPollEvery  time.Duration
}

type App struct {
	db                        *gorm.DB
	cfg                       Config
	sub2APITokenMu            sync.Mutex
	sub2APIToken              string
	sub2APITokenExpiry        time.Time
	sub2APIGroupRateMonitorMu sync.Mutex
	telegramBotMu             sync.Mutex
	telegramBotCancel         context.CancelFunc
}

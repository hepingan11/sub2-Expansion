package app

import (
	"regexp"
	"time"
)

const (
	statusAvailable = "AVAILABLE"
	statusAssigned  = "ASSIGNED"
	statusUsed      = "USED"
	statusVoided    = "VOIDED"

	dailyMaxUsersKey = "check_in.daily_max_users"
	prizeTiersKey    = "check_in.prize_tiers"

	sub2APIBaseURLKey        = "sub2api.base_url"
	sub2APIAdminAPIKeyKey    = "sub2api.admin_api_key"
	sub2APIAccessTokenKey    = "sub2api.jwt"
	sub2APIAdminEmailKey     = "sub2api.admin_email"
	sub2APIAdminPasswordKey  = "sub2api.admin_password"
	sub2APIAuthModeKey       = "sub2api.auth_mode"
	sub2APITimeoutKey        = "sub2api.timeout_seconds"
	sub2APITokenExpiresAtKey = "sub2api.jwt_expires_at"
)

const sub2APITokenRefreshBefore = 10 * time.Minute

var (
	validStatuses     = map[string]bool{statusAvailable: true, statusAssigned: true, statusUsed: true, statusVoided: true}
	defaultPrizeTiers = []PrizeTier{{Amount: MustAmount("1.00"), Probability: MustAmount("70.00")}, {Amount: MustAmount("3.00"), Probability: MustAmount("20.00")}, {Amount: MustAmount("5.00"), Probability: MustAmount("8.00")}, {Amount: MustAmount("10.00"), Probability: MustAmount("2.00")}}
	codeSplitPattern  = regexp.MustCompile(`[\,;\s]+`)
	codeAlphabet      = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
)

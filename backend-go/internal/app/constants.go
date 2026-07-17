package app

import (
	"regexp"
	"time"
)

const (
	statusAvailable     = "AVAILABLE"
	statusAssigned      = "ASSIGNED"
	statusUsed          = "USED"
	statusVoided        = "VOIDED"
	claimPending        = "PENDING"
	claimClaimed        = "CLAIMED"
	claimFailed         = "FAILED"
	inviteRewardPending = "PENDING"
	inviteRewarded      = "REWARDED"
	inviteRewardFailed  = "FAILED"
	checkInMethodDirect = "direct"
	checkInMethodSocial = "social"

	dailyMaxUsersKey       = "check_in.daily_max_users"
	dailyLimitModeKey      = "check_in.daily_limit_mode"
	directDailyMaxUsersKey = "check_in.direct_daily_max_users"
	socialDailyMaxUsersKey = "check_in.social_daily_max_users"
	prizeTiersKey          = "check_in.prize_tiers"
	socialPrizeTiersKey    = "check_in.social_prize_tiers"
	checkInGroupLinkKey    = "check_in.group_link"
	invitationAfterTimeKey = "invitation.after_time"
	invitationAmountKey    = "invitation.amount"
	frontendPublicURLKey   = "app.frontend_public_url"
	platformSettingsPrefix = "platform."

	adminUsernameKey     = "admin.username"
	adminPasswordHashKey = "admin.password_hash"

	sub2APIBaseURLKey        = "sub2api.base_url"
	sub2APIAdminAPIKeyKey    = "sub2api.admin_api_key"
	sub2APIAccessTokenKey    = "sub2api.jwt"
	sub2APIAdminEmailKey     = "sub2api.admin_email"
	sub2APIAdminPasswordKey  = "sub2api.admin_password"
	sub2APIAuthModeKey       = "sub2api.auth_mode"
	sub2APITimeoutKey        = "sub2api.timeout_seconds"
	sub2APITokenExpiresAtKey = "sub2api.jwt_expires_at"

	sub2APIGroupRateMonitorKey = "sub2api.group_rate_monitor"

	telegramEnabledKey             = "telegram.enabled"
	telegramBotTokenKey            = "telegram.bot_token"
	telegramBotAPIBaseURLKey       = "telegram.api_base_url"
	telegramBotPollIntervalSeconds = "telegram.poll_interval_seconds"
	telegramMembershipCheckKey     = "telegram.membership_check_enabled"
	telegramRequiredGroupChatIDKey = "telegram.required_group_chat_id"
	telegramGroupJoinURLKey        = "telegram.group_join_url"
	telegramBindingTokenTTLKey     = "telegram.binding_token_ttl_minutes"
)

const sub2APITokenRefreshBefore = 10 * time.Minute

var (
	validStatuses     = map[string]bool{statusAvailable: true, statusAssigned: true, statusUsed: true, statusVoided: true}
	defaultPrizeTiers = []PrizeTier{{Amount: MustAmount("1.00"), Probability: MustAmount("70.00")}, {Amount: MustAmount("3.00"), Probability: MustAmount("20.00")}, {Amount: MustAmount("5.00"), Probability: MustAmount("8.00")}, {Amount: MustAmount("10.00"), Probability: MustAmount("2.00")}}
	codeSplitPattern  = regexp.MustCompile(`[\,;\s]+`)
	codeAlphabet      = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
)

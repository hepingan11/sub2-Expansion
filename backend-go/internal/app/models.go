package app

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type RedeemCode struct {
	ID        uint64     `gorm:"primaryKey;column:id" json:"id"`
	Code      string     `gorm:"column:code;size:128;not null;uniqueIndex:uk_redeem_codes_code" json:"code"`
	UserID    *string    `gorm:"column:user_id;size:64;uniqueIndex:uk_redeem_codes_user_date;index:idx_redeem_codes_user_date" json:"userId"`
	SignDate  *LocalDate `gorm:"column:sign_date;type:date;uniqueIndex:uk_redeem_codes_user_date;index:idx_redeem_codes_user_date" json:"signDate"`
	Amount    Amount     `gorm:"column:amount;type:decimal(10,2);not null;index:idx_redeem_codes_status_amount" json:"amount"`
	Status    string     `gorm:"column:status;type:varchar(20);not null;index:idx_redeem_codes_status;index:idx_redeem_codes_status_amount" json:"status"`
	CreatedAt JSONTime   `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt JSONTime   `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (RedeemCode) TableName() string {
	return "redeem_codes"
}

func (r *RedeemCode) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if r.CreatedAt.Time.IsZero() {
		r.CreatedAt = now
	}
	if r.UpdatedAt.Time.IsZero() {
		r.UpdatedAt = now
	}
	return nil
}

func (r *RedeemCode) BeforeUpdate(*gorm.DB) error {
	r.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type CheckInRecord struct {
	ID           uint64    `gorm:"primaryKey;column:id"`
	UserID       string    `gorm:"column:user_id;size:64;not null;uniqueIndex:uk_check_in_records_user_date"`
	SignDate     LocalDate `gorm:"column:sign_date;type:date;not null;uniqueIndex:uk_check_in_records_user_date"`
	RedeemCodeID uint64    `gorm:"column:redeem_code_id;not null;index:idx_check_in_records_code_id"`
	CreatedAt    JSONTime  `gorm:"column:created_at;autoCreateTime"`
}

func (CheckInRecord) TableName() string {
	return "check_in_records"
}

func (r *CheckInRecord) BeforeCreate(*gorm.DB) error {
	if r.CreatedAt.Time.IsZero() {
		r.CreatedAt = JSONTime{Time: time.Now()}
	}
	return nil
}

type DailyCheckInLimit struct {
	SignDate     LocalDate `gorm:"primaryKey;column:sign_date;type:date"`
	CheckedCount int       `gorm:"column:checked_count;not null"`
	CreatedAt    JSONTime  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    JSONTime  `gorm:"column:updated_at;autoUpdateTime"`
}

func (DailyCheckInLimit) TableName() string {
	return "daily_checkin_limits"
}

func (l *DailyCheckInLimit) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if l.CreatedAt.Time.IsZero() {
		l.CreatedAt = now
	}
	if l.UpdatedAt.Time.IsZero() {
		l.UpdatedAt = now
	}
	return nil
}

func (l *DailyCheckInLimit) BeforeUpdate(*gorm.DB) error {
	l.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type SystemSetting struct {
	SettingKey   string   `gorm:"primaryKey;column:setting_key;size:100"`
	SettingValue string   `gorm:"column:setting_value;type:text;not null"`
	CreatedAt    JSONTime `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    JSONTime `gorm:"column:updated_at;autoUpdateTime"`
}

func (SystemSetting) TableName() string {
	return "system_settings"
}

func (s *SystemSetting) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if s.CreatedAt.Time.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.Time.IsZero() {
		s.UpdatedAt = now
	}
	return nil
}

func (s *SystemSetting) BeforeUpdate(*gorm.DB) error {
	s.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type FavoriteSite struct {
	ID          uint64   `gorm:"primaryKey;column:id" json:"id"`
	Icon        string   `gorm:"column:icon;size:500;not null" json:"icon"`
	URL         string   `gorm:"column:url;size:500;not null;uniqueIndex:uk_favorite_sites_url" json:"url"`
	Name        string   `gorm:"column:name;size:100;not null" json:"name"`
	Description string   `gorm:"column:description;size:500;not null" json:"description"`
	SortOrder   int      `gorm:"column:sort_order;not null;index:idx_favorite_sites_group_sort" json:"sort"`
	Group       string   `gorm:"column:group_name;size:100;not null;index:idx_favorite_sites_group_sort" json:"group"`
	CreatedAt   JSONTime `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt   JSONTime `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (FavoriteSite) TableName() string {
	return "favorite_sites"
}

func (s *FavoriteSite) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if s.CreatedAt.Time.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.Time.IsZero() {
		s.UpdatedAt = now
	}
	return nil
}

func (s *FavoriteSite) BeforeUpdate(*gorm.DB) error {
	s.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type RechargeActivity struct {
	ID          uint64    `gorm:"primaryKey;column:id" json:"id"`
	Name        string    `gorm:"column:name;size:120;not null" json:"name"`
	Description string    `gorm:"column:description;type:text;not null" json:"description"`
	Enabled     bool      `gorm:"column:enabled;not null;index:idx_recharge_activities_enabled" json:"enabled"`
	StartAt     *JSONTime `gorm:"column:start_at" json:"startAt"`
	EndAt       *JSONTime `gorm:"column:end_at" json:"endAt"`
	CreatedAt   JSONTime  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt   JSONTime  `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (RechargeActivity) TableName() string {
	return "recharge_activities"
}

func (a *RechargeActivity) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if a.CreatedAt.Time.IsZero() {
		a.CreatedAt = now
	}
	if a.UpdatedAt.Time.IsZero() {
		a.UpdatedAt = now
	}
	return nil
}

func (a *RechargeActivity) BeforeUpdate(*gorm.DB) error {
	a.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type RechargeRewardTier struct {
	ID              uint64   `gorm:"primaryKey;column:id" json:"id"`
	ActivityID      uint64   `gorm:"column:activity_id;not null;index:idx_recharge_reward_tiers_activity" json:"activityId"`
	ThresholdAmount Amount   `gorm:"column:threshold_amount;type:decimal(10,2);not null" json:"thresholdAmount"`
	RewardAmount    Amount   `gorm:"column:reward_amount;type:decimal(10,2);not null" json:"rewardAmount"`
	SortOrder       int      `gorm:"column:sort_order;not null;index:idx_recharge_reward_tiers_activity" json:"sort"`
	CreatedAt       JSONTime `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       JSONTime `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (RechargeRewardTier) TableName() string {
	return "recharge_reward_tiers"
}

func (t *RechargeRewardTier) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if t.CreatedAt.Time.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.Time.IsZero() {
		t.UpdatedAt = now
	}
	return nil
}

func (t *RechargeRewardTier) BeforeUpdate(*gorm.DB) error {
	t.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type RechargeRewardClaim struct {
	ID              uint64   `gorm:"primaryKey;column:id" json:"id"`
	ActivityID      uint64   `gorm:"column:activity_id;not null;uniqueIndex:uk_recharge_reward_claim_user_tier" json:"activityId"`
	TierID          uint64   `gorm:"column:tier_id;not null;uniqueIndex:uk_recharge_reward_claim_user_tier" json:"tierId"`
	UserID          int64    `gorm:"column:user_id;not null;uniqueIndex:uk_recharge_reward_claim_user_tier;index:idx_recharge_reward_claims_user" json:"userId"`
	ThresholdAmount Amount   `gorm:"column:threshold_amount;type:decimal(10,2);not null" json:"thresholdAmount"`
	RewardAmount    Amount   `gorm:"column:reward_amount;type:decimal(10,2);not null" json:"rewardAmount"`
	Status          string   `gorm:"column:status;size:20;not null;index:idx_recharge_reward_claims_status" json:"status"`
	RedeemCode      string   `gorm:"column:redeem_code;size:128;not null" json:"redeemCode"`
	ErrorMessage    string   `gorm:"column:error_message;type:text;not null" json:"errorMessage"`
	CreatedAt       JSONTime `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       JSONTime `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (RechargeRewardClaim) TableName() string {
	return "recharge_reward_claims"
}

func (c *RechargeRewardClaim) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if c.CreatedAt.Time.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.Time.IsZero() {
		c.UpdatedAt = now
	}
	return nil
}

func (c *RechargeRewardClaim) BeforeUpdate(*gorm.DB) error {
	c.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type SocialAccountBinding struct {
	ID             uint64   `gorm:"primaryKey;column:id" json:"id"`
	UserID         int64    `gorm:"column:user_id;not null;uniqueIndex:uk_social_bindings_user_platform;index:idx_social_bindings_user" json:"userId"`
	Platform       string   `gorm:"column:platform;size:40;not null;uniqueIndex:uk_social_bindings_user_platform;uniqueIndex:uk_social_bindings_platform_external" json:"platform"`
	ExternalUserID string   `gorm:"column:external_user_id;size:128;not null;uniqueIndex:uk_social_bindings_platform_external" json:"externalUserId"`
	CreatedAt      JSONTime `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt      JSONTime `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (SocialAccountBinding) TableName() string {
	return "social_account_bindings"
}

func (b *SocialAccountBinding) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if b.CreatedAt.Time.IsZero() {
		b.CreatedAt = now
	}
	if b.UpdatedAt.Time.IsZero() {
		b.UpdatedAt = now
	}
	return nil
}

func (b *SocialAccountBinding) BeforeUpdate(*gorm.DB) error {
	b.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type Sub2APIGroupRateSnapshot struct {
	GroupID        string          `gorm:"primaryKey;column:group_id;size:100" json:"groupId"`
	GroupName      string          `gorm:"column:group_name;size:200;not null" json:"groupName"`
	RateMultiplier decimal.Decimal `gorm:"column:rate_multiplier;type:decimal(18,6);not null" json:"-"`
	RawJSON        string          `gorm:"column:raw_json;type:text;not null" json:"rawJson"`
	LastSeenAt     JSONTime        `gorm:"column:last_seen_at;not null" json:"lastSeenAt"`
	CreatedAt      JSONTime        `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt      JSONTime        `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (Sub2APIGroupRateSnapshot) TableName() string {
	return "sub2api_group_rate_snapshots"
}

func (s *Sub2APIGroupRateSnapshot) BeforeCreate(*gorm.DB) error {
	now := JSONTime{Time: time.Now()}
	if s.CreatedAt.Time.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.Time.IsZero() {
		s.UpdatedAt = now
	}
	if s.LastSeenAt.Time.IsZero() {
		s.LastSeenAt = now
	}
	return nil
}

func (s *Sub2APIGroupRateSnapshot) BeforeUpdate(*gorm.DB) error {
	s.UpdatedAt = JSONTime{Time: time.Now()}
	return nil
}

type Sub2APIGroupRateLog struct {
	ID            uint64          `gorm:"primaryKey;column:id" json:"id"`
	GroupID       string          `gorm:"column:group_id;size:100;not null;index:idx_sub2api_group_rate_logs_group_time" json:"groupId"`
	GroupName     string          `gorm:"column:group_name;size:200;not null" json:"groupName"`
	OldRate       decimal.Decimal `gorm:"column:old_rate;type:decimal(18,6);not null" json:"-"`
	NewRate       decimal.Decimal `gorm:"column:new_rate;type:decimal(18,6);not null" json:"-"`
	Source        string          `gorm:"column:source;size:40;not null" json:"source"`
	PublicVisible bool            `gorm:"column:public_visible;not null;index:idx_sub2api_group_rate_logs_public" json:"publicVisible"`
	CreatedAt     JSONTime        `gorm:"column:created_at;autoCreateTime;index:idx_sub2api_group_rate_logs_group_time" json:"createdAt"`
}

func (Sub2APIGroupRateLog) TableName() string {
	return "sub2api_group_rate_logs"
}

func (l *Sub2APIGroupRateLog) BeforeCreate(*gorm.DB) error {
	if l.CreatedAt.Time.IsZero() {
		l.CreatedAt = JSONTime{Time: time.Now()}
	}
	return nil
}

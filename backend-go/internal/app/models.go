package app

import (
	"time"

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

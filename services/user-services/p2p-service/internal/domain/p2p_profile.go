// internal/domain/p2p_profile.go
package domain

import (
	//"database/sql"
	"encoding/json"
	"time"
)

type CreateProfileRequest struct {
	UserID                  string     `json:"user_id"`
	Username                string     `json:"username,omitempty"`
	PhoneNumber             string     `json:"phone_number,omitempty"`
	Email                   string     `json:"email,omitempty"`
	ProfilePictureURL       string     `json:"profile_picture_url,omitempty"`
	PreferredCurrency       string     `json:"preferred_currency,omitempty"`
	PreferredPaymentMethods []int      `json:"preferred_payment_methods,omitempty"`
	AutoReplyMessage        string     `json:"auto_reply_message,omitempty"`
	HasConsent              bool       `json:"has_consent"`
	ConsentedAt             *time.Time `json:"consented_at,omitempty"`
}

type P2PProfile struct {
	ID       int64  `json:"id"`
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`

	// Contact info (optional)
	PhoneNumber       *string `json:"phone_number,omitempty"`
	Email             *string `json:"email,omitempty"`
	ProfilePictureURL *string `json:"profile_picture_url,omitempty"`

	// Trading stats
	TotalTrades     int     `json:"total_trades"`
	CompletedTrades int     `json:"completed_trades"`
	CancelledTrades int     `json:"cancelled_trades"`
	AvgRating       float64 `json:"avg_rating"`
	TotalReviews    int     `json:"total_reviews"`

	// Status
	IsVerified  bool    `json:"is_verified"`
	IsMerchant  bool    `json:"is_merchant"`
	IsSuspended bool    `json:"is_suspended"`
	SuspensionReason *string `json:"suspension_reason,omitempty"`
	SuspendedUntil   *time.Time `json:"suspended_until,omitempty"`

	// Preferences
	PreferredCurrency        *string          `json:"preferred_currency,omitempty"`
	PreferredPaymentMethods  json.RawMessage  `json:"preferred_payment_methods,omitempty"` // []int
	AutoReplyMessage         *string          `json:"auto_reply_message,omitempty"`

	HasConsent  bool       `json:"has_consent"`
	ConsentedAt *time.Time `json:"consented_at,omitempty"`

	// Metadata
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	LastActiveAt   *time.Time      `json:"last_active_at,omitempty"`
	JoinedAt       time.Time       `json:"joined_at"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ProfileStats represents trading statistics
type ProfileStats struct {
	TotalTrades       int     `json:"total_trades"`
	CompletedTrades   int     `json:"completed_trades"`
	CancelledTrades   int     `json:"cancelled_trades"`
	CompletionRate    float64 `json:"completion_rate"`
	AvgRating         float64 `json:"avg_rating"`
	TotalReviews      int     `json:"total_reviews"`
	TotalTradeVolume  string  `json:"total_trade_volume"` // In preferred currency
}

// ProfilePreferences represents user preferences
type ProfilePreferences struct {
	PreferredCurrency       string   `json:"preferred_currency"`
	PreferredPaymentMethods []int    `json:"preferred_payment_methods"`
	AutoReplyMessage        string   `json:"auto_reply_message"`
	NotificationSettings    map[string]bool `json:"notification_settings"`
}

// UpdateProfileRequest for updating profiles
type UpdateProfileRequest struct {
	Username                *string  `json:"username,omitempty"`
	PhoneNumber             *string  `json:"phone_number,omitempty"`
	Email                   *string  `json:"email,omitempty"`
	ProfilePictureURL       *string  `json:"profile_picture_url,omitempty"`
	PreferredCurrency       *string  `json:"preferred_currency,omitempty"`
	PreferredPaymentMethods []int    `json:"preferred_payment_methods,omitempty"`
	AutoReplyMessage        *string  `json:"auto_reply_message,omitempty"`
}

// ProfileFilter for filtering profiles
type ProfileFilter struct {
	IsVerified  *bool
	IsMerchant  *bool
	IsSuspended *bool
	MinRating   *float64
	MinTrades   *int
	Search      string // Search by username
	Limit       int
	Offset      int
}

// CompletionRate calculates the trade completion rate
func (p *P2PProfile) CompletionRate() float64 {
	if p.TotalTrades == 0 {
		return 0
	}
	return (float64(p.CompletedTrades) / float64(p.TotalTrades)) * 100
}

// IsActive checks if profile is active (not suspended)
func (p *P2PProfile) IsActive() bool {
	if !p.IsSuspended {
		return true
	}
	if p.SuspendedUntil != nil && time.Now().After(*p.SuspendedUntil) {
		return true
	}
	return false
}

// CanTrade checks if user can trade
func (p *P2PProfile) CanTrade() bool {
	return p.IsActive() && !p.IsSuspended
}
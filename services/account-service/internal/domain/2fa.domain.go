package domain

import (
	"time"
)

// UserTwoFA represents a user’s 2FA setup.
type UserTwoFA struct {
	ID        int64     `json:"id"`
	UserID    string     `json:"user_id"`
	Method    string    `json:"method"`   // "totp", "sms", "email"
	Secret    string    `json:"secret"`   // base32 secret (for TOTP)
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserTwoFABackupCode represents a single backup code.
type UserTwoFABackupCode struct {
	ID       int64     `json:"id"`
	TwoFAID  string     `json:"twofa_id"`
	CodeHash string    `json:"code_hash"`
	IsUsed   bool      `json:"is_used"`
	CreatedAt time.Time `json:"created_at"`
	UsedAt   *time.Time `json:"used_at,omitempty"`
}

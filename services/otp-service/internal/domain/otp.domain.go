package domain

import "time"

type UserOTP struct {
	ID             int64      `json:"id"`
	UserID         *int64     `json:"user_id,omitempty"`
	OTPCode        string     `json:"otp_code"`
	DeliveryChannel string    `json:"delivery_channel"` // email, sms
	OTPPurpose     string     `json:"otp_purpose"`      // login, reset, etc.
	IssuedAt       time.Time  `json:"issued_at"`
	ValidUntil     time.Time  `json:"valid_until"`
	IsVerified     bool       `json:"is_verified"`
	IsActive       bool       `json:"is_active"`
}

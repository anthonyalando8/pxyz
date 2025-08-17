package domain

import (
	"time"
)

// UserPreferences represents customizable settings for a user.
type UserPreferences struct {
	UserID     int64                  `json:"user_id"`
	Preferences map[string]string     `json:"preferences"` // flexible key-value
	UpdatedAt  time.Time              `json:"updated_at"`
}

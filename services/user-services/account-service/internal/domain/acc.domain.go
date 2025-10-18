package domain

import (
	"time"
)

// UserProfile represents extended account info (separate from auth identity).
type UserProfile struct {
	UserID          string                  `json:"user_id"`
	DateOfBirth     *time.Time              `json:"date_of_birth,omitempty"`
	ProfileImageURL string                  `json:"profile_image_url,omitempty"`
	FirstName       string                  `json:"first_name,omitempty"`
	LastName        string                  `json:"last_name,omitempty"`
	Surname         string                  `json:"surname,omitempty"`
	SysUsername     string                  `json:"sys_username,omitempty"` // system generated
	Address         map[string]interface{}  `json:"address,omitempty"`      // stored as JSONB
	Gender          string                  `json:"gender,omitempty"`
	Bio             string                  `json:"bio,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`

	Nationality *string `json:"nationality,omitempty"` // ISO2 code, can be null
	FirstTime   bool    `json:"first_time,omitempty"`  // transient flag, not stored
}


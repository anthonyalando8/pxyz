package domain

import (
	"database/sql"
	"time"
)

type Session struct {
	ID           string      `json:"id"`
	UserID       string      `json:"user_id"`
	AuthToken    string     `json:"auth_token"` // Could be a JWT or other session token
	DeviceID     *string    `json:"device_id,omitempty"`
	IPAddress    *string    `json:"ip_address,omitempty"`
	UserAgent    *string    `json:"user_agent,omitempty"`
	GeoLocation  *string    `json:"geo_location,omitempty"`
	DeviceMeta   *any       `json:"device_metadata,omitempty"` // You might want a concrete type
	IsActive     bool       `json:"is_active"`
	IsSingleUse bool        `json:"is_single_use"`
	IsTemp bool				`json:"is_temp"`
	IsUsed bool 			`json:"is_used"`
	Purpose string			`json:"purpose"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt sql.NullTime `db:"expires_at" json:"expires_at,omitempty"`
}

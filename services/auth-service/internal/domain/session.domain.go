package domain
import "time"

type Session struct {
	ID           string      `json:"id"`
	UserID       string      `json:"user_id"`
	AuthToken    string     `json:"auth_token"`
	DeviceID     *string    `json:"device_id,omitempty"`
	IPAddress    *string    `json:"ip_address,omitempty"`
	UserAgent    *string    `json:"user_agent,omitempty"`
	GeoLocation  *string    `json:"geo_location,omitempty"`
	DeviceMeta   *any       `json:"device_metadata,omitempty"` 
	IsActive     bool       `json:"is_active"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

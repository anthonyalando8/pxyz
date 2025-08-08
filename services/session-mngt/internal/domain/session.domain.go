package domain
import "time"

type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Device     string    `json:"device,omitempty"`
	IP         string    `json:"ip,omitempty"`
	LastActive time.Time `json:"last_active"`
	Token      string 	 `json:"token"`
	CreatedAt  time.Time `json:"created_at"`
}

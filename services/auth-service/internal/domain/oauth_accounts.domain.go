package domain
import "time"
type OAuthAccount struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	Provider        string     `json:"provider"`          // google, facebook, telegram
	ProviderUserID  string     `json:"provider_user_id"`
	AccessToken     *string    `json:"access_token,omitempty"`
	RefreshToken    *string    `json:"refresh_token,omitempty"`
	LinkedAt        time.Time  `json:"linked_at"`
}

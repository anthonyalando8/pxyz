package domain
import "time"
type OAuthAccount struct {
	ID              string      `json:"id"`
	UserID          string      `json:"user_id"`
	Provider        string     `json:"provider"`          // google, facebook, telegram
	ProviderUID  string     `json:"provider_user_id"`
	AccessToken     *string    `json:"access_token,omitempty"`
	RefreshToken    *string    `json:"refresh_token,omitempty"`
	LinkedAt        time.Time  `json:"linked_at"`
}

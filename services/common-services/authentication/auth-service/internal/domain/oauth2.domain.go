package domain

import "time"

// OAuthAccount represents a linked OAuth provider account
type OAuthAccount struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	Provider     string                 `json:"provider"`      // google, apple, telegram, facebook, github, etc.
	ProviderUID  string                 `json:"provider_uid"`  // unique ID from provider
	AccessToken  *string                `json:"access_token,omitempty"`
	RefreshToken *string                `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time             `json:"expires_at,omitempty"`
	Scope        *string                `json:"scope,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"` // Additional provider-specific data
	LinkedAt     time.Time              `json:"linked_at"`
	UpdatedAt    *time.Time             `json:"updated_at,omitempty"`
}

// IsTokenValid checks if the access token is still valid
func (oa *OAuthAccount) IsTokenValid() bool {
	if oa.AccessToken == nil {
		return false
	}
	if oa.ExpiresAt == nil {
		return true // No expiry set, assume valid
	}
	return time.Now().Before(*oa.ExpiresAt)
}

// NeedsRefresh checks if the token needs to be refreshed
func (oa *OAuthAccount) NeedsRefresh() bool {
	if oa.RefreshToken == nil {
		return false
	}
	if oa.ExpiresAt == nil {
		return false
	}
	// Refresh if token expires within 5 minutes
	return time.Now().Add(5 * time.Minute).After(*oa.ExpiresAt)
}

// GetProviderName returns a human-readable provider name
func (oa *OAuthAccount) GetProviderName() string {
	names := map[string]string{
		"google":   "Google",
		"apple":    "Apple",
		"telegram": "Telegram",
		"facebook": "Facebook",
		"github":   "GitHub",
		"twitter":  "Twitter",
		"microsoft": "Microsoft",
	}
	if name, ok := names[oa.Provider]; ok {
		return name
	}
	return oa.Provider
}

// OAuthAccountSummary is a simplified view for API responses
type OAuthAccountSummary struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	ProviderName string    `json:"provider_name"`
	LinkedAt     time.Time `json:"linked_at"`
	IsActive     bool      `json:"is_active"`
}

// ToSummary converts OAuthAccount to OAuthAccountSummary
func (oa *OAuthAccount) ToSummary() *OAuthAccountSummary {
	return &OAuthAccountSummary{
		ID:           oa.ID,
		Provider:     oa.Provider,
		ProviderName: oa.GetProviderName(),
		LinkedAt:     oa.LinkedAt,
		IsActive:     oa.IsTokenValid(),
	}
}

// LinkOAuthAccountRequest represents a request to link an OAuth account
type LinkOAuthAccountRequest struct {
	Provider     string  `json:"provider" binding:"required"`
	Code         *string `json:"code,omitempty"`          // OAuth authorization code
	IDToken      *string `json:"id_token,omitempty"`      // ID token (for OIDC providers)
	AccessToken  *string `json:"access_token,omitempty"`  // Direct access token
	RefreshToken *string `json:"refresh_token,omitempty"` // Refresh token
	RedirectURI  *string `json:"redirect_uri,omitempty"`  // For code exchange
}

// UnlinkOAuthAccountRequest represents a request to unlink an OAuth account
type UnlinkOAuthAccountRequest struct {
	Provider string `json:"provider" binding:"required"`
}

// OAuth provider constants
const (
	ProviderGoogle    = "google"
	ProviderApple     = "apple"
	ProviderTelegram  = "telegram"
	ProviderFacebook  = "facebook"
	ProviderGitHub    = "github"
	ProviderTwitter   = "twitter"
	ProviderMicrosoft = "microsoft"
)

// SupportedProviders lists all supported OAuth providers
var SupportedProviders = []string{
	ProviderGoogle,
	ProviderApple,
	ProviderTelegram,
	ProviderFacebook,
	ProviderGitHub,
	ProviderTwitter,
	ProviderMicrosoft,
}

// IsProviderSupported checks if a provider is supported
func IsProviderSupported(provider string) bool {
	for _, p := range SupportedProviders {
		if p == provider {
			return true
		}
	}
	return false
}
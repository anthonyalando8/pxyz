// oauth2.go under services/auth-service/internal/domain
package domain

import "time"

// ================================
// OAUTH2 PROVIDER MODELS
// ================================

// OAuth2Client represents a third-party application
type OAuth2Client struct {
	ID             string    `json:"id"`
	ClientID       string    `json:"client_id"`
	ClientSecret   *string   `json:"-"` // Never expose
	ClientName     string    `json:"client_name"`
	ClientURI      *string   `json:"client_uri,omitempty"`
	LogoURI        *string   `json:"logo_uri,omitempty"`
	OwnerUserID    string    `json:"owner_user_id"`
	RedirectURIs   []string  `json:"redirect_uris"`
	GrantTypes     []string  `json:"grant_types"`
	ResponseTypes  []string  `json:"response_types"`
	Scope          string    `json:"scope"`
	IsConfidential bool      `json:"is_confidential"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// OAuth2ClientPublic is a safe representation without secrets
type OAuth2ClientPublic struct {
	ClientID      string    `json:"client_id"`
	ClientName    string    `json:"client_name"`
	ClientURI     *string   `json:"client_uri,omitempty"`
	LogoURI       *string   `json:"logo_uri,omitempty"`
	Scope         string    `json:"scope"`
	RedirectURIs  []string  `json:"redirect_uris"`
	CreatedAt     time.Time `json:"created_at"`
}

// ToPublic converts OAuth2Client to public representation
func (c *OAuth2Client) ToPublic() *OAuth2ClientPublic {
	return &OAuth2ClientPublic{
		ClientID:     c.ClientID,
		ClientName:   c.ClientName,
		ClientURI:    c.ClientURI,
		LogoURI:      c.LogoURI,
		Scope:        c.Scope,
		RedirectURIs: c.RedirectURIs,
		CreatedAt:    c.CreatedAt,
	}
}

// OAuth2AuthorizationCode represents an authorization code
type OAuth2AuthorizationCode struct {
	ID                    string    `json:"id"`
	Code                  string    `json:"code"`
	ClientID              string    `json:"client_id"`
	UserID                string    `json:"user_id"`
	RedirectURI           string    `json:"redirect_uri"`
	Scope                 string    `json:"scope"`
	CodeChallenge         *string   `json:"code_challenge,omitempty"`
	CodeChallengeMethod   *string   `json:"code_challenge_method,omitempty"`
	ExpiresAt             time.Time `json:"expires_at"`
	Used                  bool      `json:"used"`
	CreatedAt             time.Time `json:"created_at"`
}

// IsExpired checks if the authorization code has expired
func (a *OAuth2AuthorizationCode) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// IsValid checks if the code is still valid
func (a *OAuth2AuthorizationCode) IsValid() bool {
	return !a.Used && !a.IsExpired()
}

// OAuth2AccessToken represents an access token
type OAuth2AccessToken struct {
	ID        string     `json:"id"`
	TokenHash string     `json:"-"` // Never expose actual token
	ClientID  string     `json:"client_id"`
	UserID    *string    `json:"user_id,omitempty"` // NULL for client_credentials
	Scope     string     `json:"scope"`
	ExpiresAt time.Time  `json:"expires_at"`
	Revoked   bool       `json:"revoked"`
	CreatedAt time.Time  `json:"created_at"`
}

// IsExpired checks if the token has expired
func (t *OAuth2AccessToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the token is still valid
func (t *OAuth2AccessToken) IsValid() bool {
	return !t.Revoked && !t.IsExpired()
}

// OAuth2RefreshToken represents a refresh token
type OAuth2RefreshToken struct {
	ID            string    `json:"id"`
	TokenHash     string    `json:"-"` // Never expose actual token
	AccessTokenID string    `json:"access_token_id"`
	ClientID      string    `json:"client_id"`
	UserID        string    `json:"user_id"`
	Scope         string    `json:"scope"`
	ExpiresAt     time.Time `json:"expires_at"`
	Revoked       bool      `json:"revoked"`
	CreatedAt     time.Time `json:"created_at"`
}

// IsExpired checks if the refresh token has expired
func (t *OAuth2RefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the refresh token is still valid
func (t *OAuth2RefreshToken) IsValid() bool {
	return !t.Revoked && !t.IsExpired()
}

// OAuth2UserConsent represents user permission granted to an app
type OAuth2UserConsent struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	ClientID  string     `json:"client_id"`
	Scope     string     `json:"scope"`
	GrantedAt time.Time  `json:"granted_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Revoked   bool       `json:"revoked"`
}

// IsExpired checks if the consent has expired
func (c *OAuth2UserConsent) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
}

// IsValid checks if the consent is still valid
func (c *OAuth2UserConsent) IsValid() bool {
	return !c.Revoked && !c.IsExpired()
}

// OAuth2Scope represents an available permission scope
type OAuth2Scope struct {
	ID          string    `json:"id"`
	Scope       string    `json:"scope"`
	Description string    `json:"description"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
}

// OAuth2AuditLog represents an audit log entry
type OAuth2AuditLog struct {
	ID        string                 `json:"id"`
	EventType string                 `json:"event_type"`
	ClientID  *string                `json:"client_id,omitempty"`
	UserID    *string                `json:"user_id,omitempty"`
	IPAddress *string                `json:"ip_address,omitempty"`
	UserAgent *string                `json:"user_agent,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// ================================
// REQUEST/RESPONSE MODELS
// ================================

// CreateOAuth2ClientRequest represents a request to register a new OAuth2 client
type CreateOAuth2ClientRequest struct {
	ClientName    string   `json:"client_name" validate:"required,min=3,max=255"`
	ClientURI     *string  `json:"client_uri,omitempty" validate:"omitempty,url"`
	LogoURI       *string  `json:"logo_uri,omitempty" validate:"omitempty,url"`
	RedirectURIs  []string `json:"redirect_uris" validate:"required,min=1,dive,url"`
	Scope         string   `json:"scope" validate:"required"`
	OwnerUserID   string   `json:"owner_user_id" validate:"required"`
}

// OAuth2TokenResponse represents a token response
type OAuth2TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

// OAuth2AuthorizationRequest represents an authorization request
type OAuth2AuthorizationRequest struct {
	ResponseType        string  `json:"response_type" validate:"required"`
	ClientID            string  `json:"client_id" validate:"required"`
	RedirectURI         string  `json:"redirect_uri" validate:"required,url"`
	Scope               string  `json:"scope"`
	State               string `json:"state,omitempty"`
	CodeChallenge       *string `json:"code_challenge,omitempty"`       // PKCE
	CodeChallengeMethod *string `json:"code_challenge_method,omitempty"` // PKCE
}

// ================================
// DOMAIN ERRORS
// ================================

type UpdateOAuth2ClientRequest struct {
	ClientName   *string   `json:"client_name,omitempty"`
	ClientURI    *string   `json:"client_uri,omitempty"`
	LogoURI      *string   `json:"logo_uri,omitempty"`
	RedirectURIs []string  `json:"redirect_uris,omitempty"`
	IsActive     *bool     `json:"is_active,omitempty"`
}

// OAuth2TokenIntrospection represents token introspection response (RFC 7662)
type OAuth2TokenIntrospection struct {
	Active    bool    `json:"active"`
	Scope     string  `json:"scope,omitempty"`
	ClientID  string  `json:"client_id,omitempty"`
	UserID    *string `json:"user_id,omitempty"`
	TokenType string  `json:"token_type,omitempty"`
	ExpiresAt int64   `json:"exp,omitempty"`
	IssuedAt  int64   `json:"iat,omitempty"`
}


// new

// ConsentInfo represents information shown on the consent screen
type ConsentInfo struct {
	ClientID           string            `json:"client_id"`
	ClientName         string            `json:"client_name"`
	ClientURI          *string           `json:"client_uri,omitempty"`
	LogoURI            *string           `json:"logo_uri,omitempty"`
	RequestedScopes    []string          `json:"requested_scopes"`
	ScopeDescriptions  map[string]string `json:"scope_descriptions"`
	HasExistingConsent bool              `json:"has_existing_consent"`
	GrantedScopes      []string          `json:"granted_scopes,omitempty"`
}


// OAuth2TokenRequest represents a token exchange request
type OAuth2TokenRequest struct {
	GrantType    string  `json:"grant_type"`
	ClientID     string  `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
	Code         *string `json:"code"`
	RedirectURI  *string `json:"redirect_uri"`
	RefreshToken *string `json:"refresh_token"`
	Scope        *string `json:"scope"`
	CodeVerifier *string `json:"code_verifier"`
}

// Custom errors for OAuth2
var (
	ErrInvalidClient          = NewAppError("invalid_client", "Client authentication failed")
	ErrUnauthorizedClient     = NewAppError("unauthorized_client", "Client is not authorized")
	ErrInvalidGrant           = NewAppError("invalid_grant", "Invalid authorization grant")
	ErrUnsupportedGrantType   = NewAppError("unsupported_grant_type", "Grant type not supported")
	ErrInvalidScope           = NewAppError("invalid_scope", "Invalid scope")
	ErrInvalidRedirectURI     = NewAppError("invalid_redirect_uri", "Invalid redirect URI")
	ErrOauthConsentRequired        = NewAppError("consent_required", "User consent required")
	ErrAccessDenied           = NewAppError("access_denied", "Access denied")
)

// AppError represents an application error
type AppError struct {
	Code    string
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}
// user.go under services/audit-service/internal/domain
package domain

import "time"

// Core user record (maps to `users` table)
type User struct {
	ID              string    `json:"id"`                // Snowflake ID
	AccountStatus   string    `json:"account_status"`    // active | deleted | suspended
	AccountType     string    `json:"account_type"`      // password | social | hybrid
	AccountRestored bool      `json:"account_restored"`  // restored after deletion
	Consent         bool      `json:"consent"`           // GDPR / TOS consent
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserCredential (maps to `user_credentials` table)
type UserCredential struct {
	ID              string     `json:"id"`                  // Snowflake ID
	UserID          string     `json:"user_id"`             // FK → users.id
	Email           *string    `json:"email,omitempty"`     // Optional & case-insensitive
	Phone           *string    `json:"phone,omitempty"`     // Optional
	PasswordHash    *string    `json:"-"`                   // Never expose
	IsEmailVerified bool       `json:"is_email_verified"`
	IsPhoneVerified bool       `json:"is_phone_verified"`
	Valid           bool       `json:"valid"`               // Only one credential set is valid
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// UserWithCredential combines user and their active credential
type UserWithCredential struct {
	User       User           `json:"user"`
	Credential UserCredential `json:"credential"`
}

// UserProfile is a view-friendly representation without sensitive data
type UserProfile struct {
	ID              string    `json:"id"`
	AccountStatus   string    `json:"account_status"`
	AccountType     string    `json:"account_type"`
	Email           *string   `json:"email,omitempty"`
	Phone           *string   `json:"phone,omitempty"`
	IsEmailVerified bool      `json:"is_email_verified"`
	IsPhoneVerified bool      `json:"is_phone_verified"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ToProfile converts UserWithCredential to UserProfile (safe for API responses)
func (uwc *UserWithCredential) ToProfile() *UserProfile {
	return &UserProfile{
		ID:              uwc.User.ID,
		AccountStatus:   uwc.User.AccountStatus,
		AccountType:     uwc.User.AccountType,
		Email:           uwc.Credential.Email,
		Phone:           uwc.Credential.Phone,
		IsEmailVerified: uwc.Credential.IsEmailVerified,
		IsPhoneVerified: uwc.Credential.IsPhoneVerified,
		CreatedAt:       uwc.User.CreatedAt,
		UpdatedAt:       uwc.User.UpdatedAt,
	}
}

// CredentialHistory (maps to `credential_history` table)
type CredentialHistory struct {
	ID              string    `json:"id"`            // Snowflake ID
	UserID          string    `json:"user_id"`       // FK → users.id
	OldEmail        *string   `json:"old_email"`
	OldPhone        *string   `json:"old_phone"`
	OldPasswordHash *string   `json:"-"`             // Keep internal only
	ChangedAt       time.Time `json:"changed_at"`
}

// CreateUserRequest represents the data needed to create a new user
type CreateUserRequest struct {
	Email        *string `json:"email,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	Consent      bool    `json:"consent"`
}

// Validate checks if the create user request is valid
func (r *CreateUserRequest) Validate() error {
	if r.Email == nil && r.Phone == nil {
		return ErrMissingContactInfo
	}
	if !r.Consent {
		return ErrConsentRequired
	}
	return nil
}

// Domain errors
var (
	ErrMissingContactInfo = &DomainError{Code: "MISSING_CONTACT", Message: "email or phone required"}
	ErrMissingPassword    = &DomainError{Code: "MISSING_PASSWORD", Message: "password required for password accounts"}
	ErrConsentRequired    = &DomainError{Code: "CONSENT_REQUIRED", Message: "user consent is required"}
	ErrInvalidCredential  = &DomainError{Code: "INVALID_CREDENTIAL", Message: "invalid email or phone"}
	ErrCredentialTaken    = &DomainError{Code: "CREDENTIAL_TAKEN", Message: "email or phone already in use"}
)

// DomainError represents a domain-specific error
type DomainError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *DomainError) Error() string {
	return e.Message
}
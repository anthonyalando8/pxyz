package handler

// OAuth2Context holds OAuth2 authorization request context
type OAuth2Context struct {
	ClientID            string  `json:"client_id"`
	RedirectURI         string  `json:"redirect_uri"`
	Scope               string  `json:"scope"`
	State               string  `json:"state"`
	CodeChallenge       *string `json:"code_challenge,omitempty"`
	CodeChallengeMethod *string `json:"code_challenge_method,omitempty"`
}

type SubmitIdentifierRequest struct {
	Identifier string `json:"identifier" binding:"required"`

	DeviceID       *string        `json:"device_id"`
	GeoLocation    *string        `json:"geo_location"`
	DeviceMetadata *any           `json:"device_metadata"`
	OAuth2Context  *OAuth2Context `json:"oauth2_context,omitempty"` // NEW: OAuth2 context
}

// VerifyIdentifierRequest for verification step
type VerifyIdentifierRequest struct {
	Code          string         `json:"code" binding:"required"`
	OAuth2Context *OAuth2Context `json:"oauth2_context,omitempty"` // NEW
}

// LoginPasswordRequest for final login step
type LoginPasswordRequest struct {
	Password      string         `json:"password" binding:"required"`
	OAuth2Context *OAuth2Context `json:"oauth2_context,omitempty"` // NEW
}

// SetPasswordRequest for setting password
type SetPasswordRequest struct {
	Password      string         `json:"password" binding:"required"`
	OAuth2Context *OAuth2Context `json:"oauth2_context,omitempty"` // NEW
}

type ForgotPasswordRequest struct {
	Identifier string `json:"identifier"`
	
	DeviceID       *string     `json:"device_id"`
	GeoLocation    *string     `json:"geo_location"`
	DeviceMetadata *any        `json:"device_metadata"`
}

type ChangeEmailRequest struct {
	UserID   string `json:"user_id"`
	NewEmail string `json:"new_email"`
}

type RequestPhoneChange struct {
	NewPhone string `json:"new_phone"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password,omitempty"`
	NewPassword string `json:"new_password"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type UserExistsRequest struct {
	Identifier string `json:"identifier"`
}

type ProfileRequest struct {
	UserID string `json:"user_id"`
}

type UpdateProfileRequest struct {
	UserID         string                 `json:"user_id"`
	DateOfBirth    *string                `json:"date_of_birth,omitempty"` // ISO 860
	ProfileImageURL *string                `json:"profile_image_url,omitempty"`
	FirstName      *string                `json:"first_name,omitempty"`
	LastName       *string                `json:"last_name,omitempty"`
	Surname        *string                `json:"surname,omitempty"`
	SysUsername    *string                `json:"username,omitempty"` // system generated
	Address        *map[string]interface{} `json:"address,omitempty"`      // stored as JSONB
	Bio			*string                `json:"bio,omitempty"`
}
package handler

type RegisterRequest struct {
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`

	DeviceID       *string     `json:"device_id"`
	GeoLocation    *string     `json:"geo_location"`
	DeviceMetadata *any        `json:"device_metadata"`
}

type RegisterInit struct {
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
	AcceptTerms bool   `json:"accept_terms"`

	// Optional fields for device tracking

	DeviceID       *string     `json:"device_id"`
	GeoLocation    *string     `json:"geo_location"`
	DeviceMetadata *any        `json:"device_metadata"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
	
	DeviceID       *string     `json:"device_id"`
	GeoLocation    *string     `json:"geo_location"`
	DeviceMetadata *any        `json:"device_metadata"`
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

type SetPasswordRequest struct {
	Password string `json:"new_password"`
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
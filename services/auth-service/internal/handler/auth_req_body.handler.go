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

type ChangeEmailRequest struct {
	UserID   string `json:"user_id"`
	NewEmail string `json:"new_email"`
	OTP string `json:"otp_code"`
}

type RequestEmailChange struct {
	TOTP string `json:"totp_code"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password,omitempty"`
	NewPassword string `json:"new_password"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type SetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}


type UserExistsRequest struct {
	Identifier string `json:"identifier"`
}

type UpdateNameRequest struct {
	UserID     string `json:"user_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}

type ProfileRequest struct {
	UserID string `json:"user_id"`
}

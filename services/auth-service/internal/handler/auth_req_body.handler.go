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
}

type ChangePasswordRequest struct {
	UserID      string `json:"user_id"`
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

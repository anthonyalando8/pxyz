package autherrors

import "errors"

// Generic
var (
	ErrInvalidRequest     = errors.New("invalid request")
	ErrInternalServer     = errors.New("internal server error")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
)

// Registration / Login
var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrWeakPassword       = errors.New("weak password")
)

// Verification
var (
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrPhoneNotVerified   = errors.New("phone not verified")
	ErrInvalidOTP         = errors.New("invalid otp")
	ErrExpiredOTP         = errors.New("expired otp")
	ErrTooManyOTPRequests = errors.New("too many otp requests")
	ErrOTPBlocked         = errors.New("otp temporarily blocked")
)

// Account state
var (
	ErrAccountDeleted     = errors.New("account deleted")
	ErrAccountSuspended   = errors.New("account suspended")
	ErrAccountBanned      = errors.New("account banned")
	ErrAccountRestricted  = errors.New("account restricted")
)

// Social auth
var (
	ErrSocialAccountOnly  = errors.New("social account only")
	ErrPasswordAlreadySet = errors.New("password already set")
	ErrPasswordNotSet     = errors.New("password not set")
)

// Misc
var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("expired token")
)

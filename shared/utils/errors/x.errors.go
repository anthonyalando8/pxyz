package xerrors

import "errors"
import "github.com/jackc/pgx/v5/pgconn"

type RepoError struct {
	Entity string
	Code   string
	Msg    string
	Ref    string
}

func ParsePGErrorCode(err error) string {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code // e.g. 23505 for unique_violation
	}
	return "unknown"
}

// Generic
var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrInternalServer = errors.New("internal server error")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrNotFound       = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input provided")
)

// Registration / Login
var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrWeakPassword       = errors.New("weak password")
	ErrEmailAlreadyInUse  = errors.New("email already in use")
	ErrPhoneAlreadyInUse  = errors.New("phone already in use")
	ErrInvalidSignupStage = errors.New("invalid signup stage")
	ErrIncompleteProfile  = errors.New("incomplete profile") // special case

	// Registration requirements
	ErrTermsRequired      = errors.New("you must accept terms and conditions to register")
	ErrIdentifierRequired = errors.New("identifier required")
	ErrEmailRequired      = errors.New("email required")
	ErrPasswordRequired   = errors.New("password required")
	ErrUserIDRequired     = errors.New("user ID required")
	ErrIdentifierAndPass  = errors.New("identifier and password required")

	// Input validation
	ErrInvalidEmailFormat = errors.New("invalid email format")
)

// SignupError for incomplete profile
type SignupError struct {
	Stage     string
	NextStage string
}

func (e *SignupError) Error() string {
	return "incomplete profile at stage: " + e.Stage
}

// Verification / OTP / 2FA
var (
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrPhoneNotVerified   = errors.New("phone not verified")
	ErrInvalidOTP         = errors.New("invalid otp")
	ErrExpiredOTP         = errors.New("expired otp")
	ErrTooManyOTPRequests = errors.New("too many otp requests")
	ErrOTPBlocked         = errors.New("otp temporarily blocked")

	// 2FA / TOTP
	ErrInvalidOrExpiredTOTP = errors.New("invalid or expired totp code")
	ErrInvalidTOTP          = errors.New("invalid totp code")
	ErrMissingVerification  = errors.New("missing verification code")
	ErrUnsupported2FAMethod = errors.New("unsupported 2FA method")
	Err2FANotEnabled        = errors.New("2FA not enabled for this method")
	ErrInvalidCodeProvided  = errors.New("invalid code provided")

	ErrUserNoEmail = errors.New("user does not have a valid email")
	ErrUserNoPhone = errors.New("user does not have a valid phone number")
	ErrInvalidChannel = errors.New("invalid channel")
	ErrUserNoComms = errors.New("No valid recipient available info available")
)

// Account state
var (
	ErrAccountDeleted    = errors.New("account deleted")
	ErrAccountSuspended  = errors.New("account suspended")
	ErrAccountBanned     = errors.New("account banned")
	ErrAccountRestricted = errors.New("account restricted")
)

// Social auth
var (
	ErrSocialAccountOnly   = errors.New("social account only")
	ErrPasswordAlreadySet  = errors.New("password already set")
	ErrPasswordNotSet      = errors.New("password not set")
	ErrSocialLoginNotAllow = errors.New("social login not allowed")
)

// Password rules
var (
	ErrInvalidPassword       = errors.New("invalid password")
	ErrInvalidOldPassword    = errors.New("invalid old password")
	ErrPasswordTooShort      = errors.New("password must be at least 8 characters long")
	ErrPasswordTooLong       = errors.New("password must not exceed 100 characters")
	ErrPasswordUppercase     = errors.New("password must include at least one uppercase letter")
	ErrPasswordLowercase     = errors.New("password must include at least one lowercase letter")
	ErrPasswordDigit         = errors.New("password must include at least one digit")
	ErrPasswordSpecialChar   = errors.New("password must include at least one special character")
)

// Session
var (
	ErrSessionUsed = errors.New("session used")
)

// Email change
var (
	ErrNoPendingEmailChange = errors.New("no pending email change")
)

// Token
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

// KYC
var (
	ErrNoKYCSubmission     = errors.New("no KYC submission found for user")
	ErrKYCRejectionNote    = errors.New("rejection note is required when rejecting KYC")
	ErrInvalidKYCDecision  = errors.New("invalid decision: must be 'approved' or 'rejected'")
)

// Integrations
var (
	ErrInvalidApplePEM = errors.New("invalid PEM for Apple private key")
)

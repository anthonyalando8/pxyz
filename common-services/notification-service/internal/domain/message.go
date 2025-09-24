package domain

import "context"

// NotificationType defines category of messages
type NotificationType string

const (
	Info          NotificationType = "INFO"
	Alert         NotificationType = "ALERT"
	Transactional NotificationType = "TRANSACTIONAL"
	Security      NotificationType = "SECURITY"

	OTP           NotificationType = "OTP"
	WELCOME		NotificationType = "WELCOME"
	GENERAL NotificationType = "GENERAL"
	EMAIL_UPDATE_OLD NotificationType = "EMAIL_UPDATE_OLD"
	EMAIL_UPDATE_NEW NotificationType = "EMAIL_UPDATE_NEW"
	PHONE_UPDATE NotificationType = "PHONE_UPDATE"
	PASSWORD_UPDATE NotificationType = "PASSWORD_UPDATE"
	TWOFA_ENABLED NotificationType = "2FA_ENABLED"
	TWOFA_DISABLED NotificationType = "2FA_DISABLED"
	TWOFA_BACKUP_CODE_REGN NotificationType = "2FA_BACKUP_CODE_REGN"
	KYC_SUBMITTED NotificationType = "KYC_SUBMITTED"
	KYC_REVIEWED NotificationType = "KYC_REVIEWED"
	WELCOME_ADMIN NotificationType = "WELCOME_ADMIN"
	PARTNER_USER_ADDED NotificationType = "PARTNER_USER_ADDED"
	WELCOME_NEW_PARTNER_USER NotificationType = "WELCOME_NEW_PARTNER_USER"
	PARTNER_CREATED NotificationType  = "PARTNER_CREATED"
	ACCOUNT_CREDITED NotificationType = "ACCOUNT_CREDITED"
	ACCOUNT_DEBITED NotificationType = "ACCOUNT_DEBITED"
	TRANSACTION_FAILED NotificationType = "TRANSACTION_FAILED"
)

// WSMessage is a minimal view of Message for websocket clients
type WSMessage struct {
    OwnerID   string                 `json:"owner_id"`
    OwnerType string                 `json:"owner_type"`
    Title     string                 `json:"title"`
    Body      string                 `json:"body"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
    Data      any                    `json:"data,omitempty"`
}


// Message represents a generic notification payload
type Message struct {
    OwnerID     string
    OwnerType   string
    Recipient   string                 // kept for backward-compatibility (primary recipient)
    Recipients  map[string]string      // {"email": "...", "phone": "..."}
    Title       string
    Body        string
    Metadata    map[string]interface{}
    Channels    []string               // ["email", "sms", "ws"], if empty = all
    Type        NotificationType
    Data        any                    // used for template rendering
    Ctx         context.Context        // allow cancellation/timeouts
}
// security_audit.go under services/audit-service/internal/domain
package domain

import "time"

// ================================
// SECURITY AUDIT LOG
// ================================

type SecurityAuditLog struct {
	ID              string                 `json:"id"`
	EventType       string                 `json:"event_type"`
	EventCategory   string                 `json:"event_category"`
	Severity        string                 `json:"severity"`
	Status          string                 `json:"status"`
	UserID          *string                `json:"user_id,omitempty"`
	TargetUserID    *string                `json:"target_user_id,omitempty"`
	SessionID       *string                `json:"session_id,omitempty"`
	IPAddress       *string                `json:"ip_address,omitempty"`
	UserAgent       *string                `json:"user_agent,omitempty"`
	RequestID       *string                `json:"request_id,omitempty"`
	ResourceType    *string                `json:"resource_type,omitempty"`
	ResourceID      *string                `json:"resource_id,omitempty"`
	Action          *string                `json:"action,omitempty"`
	Description     *string                `json:"description,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	PreviousValue   map[string]interface{} `json:"previous_value,omitempty"`
	NewValue        map[string]interface{} `json:"new_value,omitempty"`
	ErrorCode       *string                `json:"error_code,omitempty"`
	ErrorMessage    *string                `json:"error_message,omitempty"`
	Country         *string                `json:"country,omitempty"`
	City            *string                `json:"city,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// Event Categories
const (
	EventCategoryAuthentication = "authentication"
	EventCategoryAuthorization  = "authorization"
	EventCategoryAccount        = "account"
	EventCategorySecurity       = "security"
	EventCategoryOAuth2         = "oauth2"
	EventCategoryAdmin          = "admin"
)

// Event Types - Authentication
const (
	EventLoginSuccess        = "login_success"
	EventLoginFailed         = "login_failed"
	EventLogout              = "logout"
	EventSessionExpired      = "session_expired"
	EventPasswordChanged     = "password_changed"
	EventPasswordResetRequested = "password_reset_requested"
	EventPasswordResetCompleted = "password_reset_completed"
	EventEmailVerified       = "email_verified"
	EventPhoneVerified       = "phone_verified"
)

// Event Types - Account
const (
	EventAccountCreated      = "account_created"
	EventAccountDeleted      = "account_deleted"
	EventAccountSuspended    = "account_suspended"
	EventAccountRestored     = "account_restored"
	EventAccountLocked       = "account_locked"
	EventAccountUnlocked     = "account_unlocked"
	EventEmailChanged        = "email_changed"
	EventPhoneChanged        = "phone_changed"
)

// Event Types - Security
const (
	EventSuspiciousActivity     = "suspicious_activity"
	EventMultipleFailedLogins   = "multiple_failed_logins"
	EventUnusualLocation        = "unusual_location"
	EventBruteForceDetected     = "brute_force_detected"
	EventTokenRevoked           = "token_revoked"
	EventMFAEnabled             = "mfa_enabled"
	EventMFADisabled            = "mfa_disabled"
)

// Severity Levels
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// Status Values
const (
	StatusSuccess = "success"
	StatusFailure = "failure"
	StatusPending = "pending"
)

// ================================
// SUSPICIOUS ACTIVITY
// ================================

type SuspiciousActivity struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	ActivityType string                 `json:"activity_type"`
	RiskScore    int                    `json:"risk_score"`
	IPAddress    *string                `json:"ip_address,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Status       string                 `json:"status"`
	ResolvedBy   *string                `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time             `json:"resolved_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Activity Status
const (
	ActivityStatusActive        = "active"
	ActivityStatusResolved      = "resolved"
	ActivityStatusFalsePositive = "false_positive"
)

// ================================
// FAILED LOGIN ATTEMPTS
// ================================

type FailedLoginAttempt struct {
	ID             string     `json:"id"`
	Identifier     string     `json:"identifier"`
	IdentifierType string     `json:"identifier_type"`
	IPAddress      string     `json:"ip_address"`
	UserAgent      *string    `json:"user_agent,omitempty"`
	FailureReason  *string    `json:"failure_reason,omitempty"`
	AttemptedAt    time.Time  `json:"attempted_at"`
}

// Identifier Types
const (
	IdentifierTypeEmail  = "email"
	IdentifierTypePhone  = "phone"
	IdentifierTypeUserID = "user_id"
)

// ================================
// ACCOUNT LOCKOUT
// ================================

type AccountLockout struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id"`
	Reason     string                 `json:"reason"`
	LockedBy   *string                `json:"locked_by,omitempty"`
	LockedAt   time.Time              `json:"locked_at"`
	UnlockAt   *time.Time             `json:"unlock_at,omitempty"`
	UnlockedAt *time.Time             `json:"unlocked_at,omitempty"`
	UnlockedBy *string                `json:"unlocked_by,omitempty"`
	IsActive   bool                   `json:"is_active"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Lockout Reasons
const (
	LockoutReasonFailedAttempts    = "too_many_failed_attempts"
	LockoutReasonSuspiciousActivity = "suspicious_activity"
	LockoutReasonAdminAction       = "admin_action"
	LockoutReasonSecurityBreach    = "security_breach"
)

// ================================
// AUDIT RETENTION POLICY
// ================================

type AuditRetentionPolicy struct {
	ID              string    `json:"id"`
	EventCategory   string    `json:"event_category"`
	RetentionDays   int       `json:"retention_days"`
	ArchiveEnabled  bool      `json:"archive_enabled"`
	ArchiveLocation *string   `json:"archive_location,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ================================
// SECURITY EVENTS SUMMARY
// ================================

type SecurityEventsSummary struct {
	EventDate     time.Time `json:"event_date"`
	EventCategory string    `json:"event_category"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	Severity      string    `json:"severity"`
	EventCount    int64     `json:"event_count"`
	UniqueUsers   int64     `json:"unique_users"`
	UniqueIPs     int64     `json:"unique_ips"`
}

// ================================
// REQUEST MODELS
// ================================

type CreateAuditLogRequest struct {
	EventType     string                 `json:"event_type" validate:"required"`
	EventCategory string                 `json:"event_category" validate:"required"`
	Severity      string                 `json:"severity"`
	Status        string                 `json:"status"`
	UserID        *string                `json:"user_id,omitempty"`
	TargetUserID  *string                `json:"target_user_id,omitempty"`
	SessionID     *string                `json:"session_id,omitempty"`
	IPAddress     *string                `json:"ip_address,omitempty"`
	UserAgent     *string                `json:"user_agent,omitempty"`
	RequestID     *string                `json:"request_id,omitempty"`
	ResourceType  *string                `json:"resource_type,omitempty"`
	ResourceID    *string                `json:"resource_id,omitempty"`
	Action        *string                `json:"action,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ErrorCode     *string                `json:"error_code,omitempty"`
	ErrorMessage  *string                `json:"error_message,omitempty"`
}

type AuditLogQuery struct {
	UserID        *string    `json:"user_id,omitempty"`
	EventCategory *string    `json:"event_category,omitempty"`
	EventType     *string    `json:"event_type,omitempty"`
	Severity      *string    `json:"severity,omitempty"`
	Status        *string    `json:"status,omitempty"`
	StartDate     *time.Time `json:"start_date,omitempty"`
	EndDate       *time.Time `json:"end_date,omitempty"`
	Limit         int        `json:"limit"`
	Offset        int        `json:"offset"`
}

type ReportSuspiciousActivityRequest struct {
	UserID       string                 `json:"user_id" validate:"required"`
	ActivityType string                 `json:"activity_type" validate:"required"`
	RiskScore    int                    `json:"risk_score" validate:"min=0,max=100"`
	IPAddress    *string                `json:"ip_address,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

type LockAccountRequest struct {
	UserID   string                 `json:"user_id" validate:"required"`
	Reason   string                 `json:"reason" validate:"required"`
	LockedBy *string                `json:"locked_by,omitempty"`
	UnlockAt *time.Time             `json:"unlock_at,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ================================
// HELPER METHODS
// ================================

func (s *SecurityAuditLog) IsCritical() bool {
	return s.Severity == SeverityCritical
}

func (s *SecurityAuditLog) IsFailed() bool {
	return s.Status == StatusFailure
}

func (a *SuspiciousActivity) IsHighRisk() bool {
	return a.RiskScore >= 70
}

func (a *SuspiciousActivity) IsActive() bool {
	return a.Status == ActivityStatusActive
}

func (l *AccountLockout) IsExpired() bool {
	if l.UnlockAt == nil {
		return false
	}
	return time.Now().After(*l.UnlockAt)
}

func (l *AccountLockout) IsPermanent() bool {
	return l.UnlockAt == nil && l.IsActive
}
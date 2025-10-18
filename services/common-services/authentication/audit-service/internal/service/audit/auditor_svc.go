// security_audit_service.go under services/audit-service/internal/service
package service

import (
	"audit-service/internal/domain"
	"audit-service/internal/repository"
	"context"
	"fmt"
	"net"
	"strings"
	"time"
	"github.com/oschwald/geoip2-golang"
)

// SecurityAuditService handles security audit operations
type SecurityAuditService struct {
	repo            *repository.UserRepository
	geoIPService    GeoIPService // Optional: for location tracking
	maxFailedLogins int
	lockoutDuration time.Duration
}

// GeoIPService interface for IP geolocation (optional)
type GeoIPService interface {
	GetLocation(ip string) (country, city string, err error)
	Close() error
}

type maxmindGeoIP struct {
	db *geoip2.Reader
}

// NewGeoIPService creates a new GeoIP service
func NewGeoIPService(dbPath string) (GeoIPService, error) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &maxmindGeoIP{db: db}, nil
}

// GetLocation returns country and city for given IP
func (g *maxmindGeoIP) GetLocation(ip string) (string, string, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", "", ErrInvalidIP
	}

	record, err := g.db.City(parsedIP)
	if err != nil {
		return "", "", err
	}

	country := record.Country.Names["en"]
	city := ""
	if len(record.City.Names) > 0 {
		city = record.City.Names["en"]
	}

	return country, city, nil
}

func (g *maxmindGeoIP) Close() error {
	return g.db.Close()
}

// ErrInvalidIP is returned when the IP cannot be parsed
var ErrInvalidIP = fmt.Errorf("invalid IP address")

func NewSecurityAuditService(repo *repository.UserRepository, geoIP GeoIPService) *SecurityAuditService {
	return &SecurityAuditService{
		repo:            repo,
		geoIPService:    geoIP,
		maxFailedLogins: 5,
		lockoutDuration: 30 * time.Minute,
	}
}

// ================================
// AUDIT LOGGING
// ================================

// LogAuthenticationEvent logs authentication-related events
func (s *SecurityAuditService) LogAuthenticationEvent(ctx context.Context, eventType string, userID *string, status string, req *AuditContext) error {
	return s.logEvent(ctx, &domain.SecurityAuditLog{
		EventType:     eventType,
		EventCategory: domain.EventCategoryAuthentication,
		Severity:      s.determineSeverity(eventType, status),
		Status:        status,
		UserID:        userID,
		SessionID:     req.SessionID,
		IPAddress:     req.IPAddress,
		UserAgent:     req.UserAgent,
		RequestID:     req.RequestID,
		Description:   req.Description,
		Metadata:      req.Metadata,
		ErrorCode:     req.ErrorCode,
		ErrorMessage:  req.ErrorMessage,
	})
}

// LogAccountEvent logs account management events
func (s *SecurityAuditService) LogAccountEvent(ctx context.Context, eventType string, userID string, req *AuditContext) error {
	return s.logEvent(ctx, &domain.SecurityAuditLog{
		EventType:     eventType,
		EventCategory: domain.EventCategoryAccount,
		Severity:      domain.SeverityInfo,
		Status:        domain.StatusSuccess,
		UserID:        &userID,
		TargetUserID:  req.TargetUserID,
		SessionID:     req.SessionID,
		IPAddress:     req.IPAddress,
		UserAgent:     req.UserAgent,
		RequestID:     req.RequestID,
		Action:        req.Action,
		Description:   req.Description,
		PreviousValue: req.PreviousValue,
		NewValue:      req.NewValue,
		Metadata:      req.Metadata,
	})
}

// LogSecurityEvent logs security-related events
func (s *SecurityAuditService) LogSecurityEvent(ctx context.Context, eventType string, userID *string, severity string, req *AuditContext) error {
	return s.logEvent(ctx, &domain.SecurityAuditLog{
		EventType:     eventType,
		EventCategory: domain.EventCategorySecurity,
		Severity:      severity,
		Status:        domain.StatusSuccess,
		UserID:        userID,
		SessionID:     req.SessionID,
		IPAddress:     req.IPAddress,
		UserAgent:     req.UserAgent,
		RequestID:     req.RequestID,
		Description:   req.Description,
		Metadata:      req.Metadata,
	})
}

// LogAdminAction logs administrative actions
func (s *SecurityAuditService) LogAdminAction(ctx context.Context, adminUserID string, targetUserID string, action string, req *AuditContext) error {
	return s.logEvent(ctx, &domain.SecurityAuditLog{
		EventType:     "admin_action",
		EventCategory: domain.EventCategoryAdmin,
		Severity:      domain.SeverityWarning,
		Status:        domain.StatusSuccess,
		UserID:        &adminUserID,
		TargetUserID:  &targetUserID,
		SessionID:     req.SessionID,
		IPAddress:     req.IPAddress,
		UserAgent:     req.UserAgent,
		RequestID:     req.RequestID,
		Action:        &action,
		Description:   req.Description,
		PreviousValue: req.PreviousValue,
		NewValue:      req.NewValue,
		Metadata:      req.Metadata,
	})
}

func (s *SecurityAuditService) logEvent(ctx context.Context, log *domain.SecurityAuditLog) error {
	// Add geolocation if available
	if s.geoIPService != nil && log.IPAddress != nil {
		country, city, err := s.geoIPService.GetLocation(*log.IPAddress)
		if err == nil {
			log.Country = &country
			log.City = &city
		}
	}

	return s.repo.CreateSecurityAuditLog(ctx, log)
}

// ================================
// FAILED LOGIN MANAGEMENT
// ================================

// RecordFailedLogin records a failed login attempt and checks for lockout
func (s *SecurityAuditService) RecordFailedLogin(ctx context.Context, identifier, identifierType, ipAddress string, reason string, req *AuditContext) error {
	// Record failed attempt
	attempt := &domain.FailedLoginAttempt{
		Identifier:     identifier,
		IdentifierType: identifierType,
		IPAddress:      ipAddress,
		UserAgent:      req.UserAgent,
		FailureReason:  &reason,
	}

	if err := s.repo.RecordFailedLoginAttempt(ctx, attempt); err != nil {
		return err
	}

	// Log the failed login
	s.LogAuthenticationEvent(ctx, domain.EventLoginFailed, nil, domain.StatusFailure, &AuditContext{
		IPAddress:    &ipAddress,
		UserAgent:    req.UserAgent,
		RequestID:    req.RequestID,
		Description:  stringPtr(fmt.Sprintf("Failed login attempt for %s", identifier)),
		ErrorMessage: &reason,
		Metadata: map[string]interface{}{
			"identifier":      identifier,
			"identifier_type": identifierType,
		},
	})

	// Check if we need to lock the account
	return s.checkAndLockAccount(ctx, identifier, identifierType, ipAddress)
}

// checkAndLockAccount checks if account should be locked due to failed attempts
func (s *SecurityAuditService) checkAndLockAccount(ctx context.Context, identifier, identifierType, ipAddress string) error {
	// Count recent failed attempts
	count, err := s.repo.CountRecentFailedLogins(ctx, identifier, 15)
	if err != nil {
		return err
	}

	if count >= s.maxFailedLogins {
		// Get user by identifier to lock account
		// uwc, err := s.repo.GetUserByIdentifier(ctx, identifier)
		// if err != nil {
		// 	return err
		// }

		// // Create lockout
		// unlockAt := time.Now().Add(s.lockoutDuration)
		// lockout := &domain.AccountLockout{
		// 	UserID:   uwc.User.ID,
		// 	Reason:   domain.LockoutReasonFailedAttempts,
		// 	UnlockAt: &unlockAt,
		// 	Metadata: map[string]interface{}{
		// 		"failed_attempts": count,
		// 		"ip_address":      ipAddress,
		// 	},
		// }

		// if err := s.repo.CreateAccountLockout(ctx, lockout); err != nil {
		// 	return err
		// }

		// // Log security event
		// s.LogSecurityEvent(ctx, domain.EventAccountLocked, &uwc.User.ID, domain.SeverityCritical, &AuditContext{
		// 	IPAddress:   &ipAddress,
		// 	Description: stringPtr(fmt.Sprintf("Account locked due to %d failed login attempts", count)),
		// 	Metadata: map[string]interface{}{
		// 		"reason":          domain.LockoutReasonFailedAttempts,
		// 		"unlock_at":       unlockAt,
		// 		"failed_attempts": count,
		// 	},
		// })
	}

	return nil
}

// CheckAccountLockStatus checks if an account is locked
func (s *SecurityAuditService) CheckAccountLockStatus(ctx context.Context, userID string) (*domain.AccountLockout, error) {
	isLocked, err := s.repo.IsAccountLocked(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !isLocked {
		return nil, nil
	}

	return s.repo.GetActiveLockout(ctx, userID)
}

// UnlockAccount unlocks a user account
func (s *SecurityAuditService) UnlockAccount(ctx context.Context, userID, unlockedBy string, req *AuditContext) error {
	if err := s.repo.UnlockAccount(ctx, userID, unlockedBy); err != nil {
		return err
	}

	// Log the unlock event
	return s.LogSecurityEvent(ctx, domain.EventAccountUnlocked, &userID, domain.SeverityWarning, &AuditContext{
		IPAddress:   req.IPAddress,
		Description: stringPtr("Account manually unlocked"),
		Metadata: map[string]interface{}{
			"unlocked_by": unlockedBy,
		},
	})
}

// ================================
// SUSPICIOUS ACTIVITY DETECTION
// ================================

// DetectSuspiciousActivity analyzes login patterns and detects anomalies
func (s *SecurityAuditService) DetectSuspiciousActivity(ctx context.Context, userID, ipAddress string, req *AuditContext) error {
	var suspiciousActivities []string
	riskScore := 0

	// Check for unusual location
	if s.geoIPService != nil {
		country, city, _ := s.geoIPService.GetLocation(ipAddress)
		
		// Get user's recent login locations from audit logs
		recentLogs, _ := s.repo.GetUserAuditHistory(ctx, userID, 20)
		if s.isUnusualLocation(country, city, recentLogs) {
			suspiciousActivities = append(suspiciousActivities, "unusual_location")
			riskScore += 30
		}
	}

	// Check for rapid login attempts from different IPs
	recentLogins, _ := s.repo.GetUserAuditHistory(ctx, userID, 10)
	if s.hasRapidIPChanges(recentLogins) {
		suspiciousActivities = append(suspiciousActivities, "rapid_ip_changes")
		riskScore += 20
	}

	// Check failed login attempts
	failedCount, _ := s.repo.CountRecentFailedLogins(ctx, userID, 60)
	if failedCount > 3 {
		suspiciousActivities = append(suspiciousActivities, "multiple_failed_attempts")
		riskScore += failedCount * 5
	}

	// If suspicious activity detected, record it
	if len(suspiciousActivities) > 0 {
		activity := &domain.SuspiciousActivity{
			UserID:       userID,
			ActivityType: strings.Join(suspiciousActivities, ","),
			RiskScore:    min(riskScore, 100),
			IPAddress:    &ipAddress,
			Details: map[string]interface{}{
				"activities":      suspiciousActivities,
				"failed_attempts": failedCount,
			},
		}

		if err := s.repo.CreateSuspiciousActivity(ctx, activity); err != nil {
			return err
		}

		// Log critical security event if high risk
		severity := domain.SeverityWarning
		if riskScore >= 70 {
			severity = domain.SeverityCritical
		}

		s.LogSecurityEvent(ctx, domain.EventSuspiciousActivity, &userID, severity, &AuditContext{
			IPAddress:   &ipAddress,
			Description: stringPtr(fmt.Sprintf("Suspicious activity detected: %s", strings.Join(suspiciousActivities, ", "))),
			Metadata: map[string]interface{}{
				"risk_score": riskScore,
				"activities": suspiciousActivities,
			},
		})
	}

	return nil
}

// ReportSuspiciousActivity manually reports suspicious activity
func (s *SecurityAuditService) ReportSuspiciousActivity(ctx context.Context, req *domain.ReportSuspiciousActivityRequest, reportedBy string) error {
	activity := &domain.SuspiciousActivity{
		UserID:       req.UserID,
		ActivityType: req.ActivityType,
		RiskScore:    req.RiskScore,
		IPAddress:    req.IPAddress,
		Details:      req.Details,
	}

	if err := s.repo.CreateSuspiciousActivity(ctx, activity); err != nil {
		return err
	}

	// Log the report
	return s.LogSecurityEvent(ctx, domain.EventSuspiciousActivity, &req.UserID, domain.SeverityWarning, &AuditContext{
		IPAddress:   req.IPAddress,
		Description: stringPtr(fmt.Sprintf("Suspicious activity reported by %s", reportedBy)),
		Metadata: map[string]interface{}{
			"activity_type": req.ActivityType,
			"risk_score":    req.RiskScore,
			"reported_by":   reportedBy,
		},
	})
}

// ResolveSuspiciousActivity resolves a suspicious activity
func (s *SecurityAuditService) ResolveSuspiciousActivity(ctx context.Context, activityID, resolvedBy, status string) error {
	return s.repo.ResolveSuspiciousActivity(ctx, activityID, resolvedBy, status)
}

// GetActiveSuspiciousActivities retrieves active suspicious activities for a user
func (s *SecurityAuditService) GetActiveSuspiciousActivities(ctx context.Context, userID string) ([]*domain.SuspiciousActivity, error) {
	return s.repo.GetActiveSuspiciousActivities(ctx, userID)
}

// GetHighRiskUsers retrieves users with high-risk activities
func (s *SecurityAuditService) GetHighRiskUsers(ctx context.Context, minRiskScore int, limit int) ([]*domain.SuspiciousActivity, error) {
	return s.repo.GetHighRiskActivities(ctx, minRiskScore, limit)
}

// ================================
// AUDIT QUERY & REPORTING
// ================================

// GetUserAuditHistory retrieves audit history for a user
func (s *SecurityAuditService) GetUserAuditHistory(ctx context.Context, userID string, limit int) ([]*domain.SecurityAuditLog, error) {
	return s.repo.GetUserAuditHistory(ctx, userID, limit)
}

// QueryAuditLogs queries audit logs with filters
func (s *SecurityAuditService) QueryAuditLogs(ctx context.Context, query *domain.AuditLogQuery) ([]*domain.SecurityAuditLog, error) {
	// Set default limit if not provided
	if query.Limit <= 0 {
		query.Limit = 100
	}
	if query.Limit > 1000 {
		query.Limit = 1000 // Max limit
	}

	return s.repo.GetSecurityAuditLogs(ctx, query)
}

// GetCriticalEvents retrieves recent critical security events
func (s *SecurityAuditService) GetCriticalEvents(ctx context.Context, hours int, limit int) ([]*domain.SecurityAuditLog, error) {
	return s.repo.GetCriticalSecurityEvents(ctx, hours, limit)
}

// GetSecuritySummary generates a security summary report
func (s *SecurityAuditService) GetSecuritySummary(ctx context.Context, startDate, endDate time.Time) (*SecuritySummaryReport, error) {
	summary, err := s.repo.GetSecurityEventsSummary(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	report := &SecuritySummaryReport{
		StartDate: startDate,
		EndDate:   endDate,
		Events:    summary,
	}

	// Calculate totals
	for _, event := range summary {
		report.TotalEvents += event.EventCount
		if event.Status == domain.StatusFailure {
			report.TotalFailures += event.EventCount
		}
		if event.Severity == domain.SeverityCritical {
			report.CriticalEvents += event.EventCount
		}
	}

	// Get high-risk users
	report.HighRiskUsers, _ = s.repo.GetHighRiskActivities(ctx, 70, 10)

	return report, nil
}

// GetUserRiskScore retrieves the risk score for a user
func (s *SecurityAuditService) GetUserRiskScore(ctx context.Context, userID string) (int, error) {
	return s.repo.GetUserRiskScore(ctx, userID)
}

// ================================
// RATE LIMITING
// ================================

// CheckRateLimit checks if a request should be rate-limited
func (s *SecurityAuditService) CheckRateLimit(ctx context.Context, identifier, ipAddress string) (bool, error) {
	// Check failed attempts by identifier
	identifierCount, err := s.repo.CountRecentFailedLogins(ctx, identifier, 15)
	if err != nil {
		return false, err
	}

	if identifierCount >= s.maxFailedLogins {
		return true, nil
	}

	// Check failed attempts by IP
	ipCount, err := s.repo.CountRecentFailedLoginsByIP(ctx, ipAddress, 15)
	if err != nil {
		return false, err
	}

	if ipCount >= s.maxFailedLogins*2 {
		return true, nil
	}

	return false, nil
}

// ================================
// CLEANUP & MAINTENANCE
// ================================

// RunMaintenance runs periodic maintenance tasks
func (s *SecurityAuditService) RunMaintenance(ctx context.Context) (*MaintenanceReport, error) {
	report := &MaintenanceReport{
		StartTime: time.Now(),
	}

	// Auto-unlock expired lockouts
	unlockedCount, err := s.repo.AutoUnlockExpiredLockouts(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("auto-unlock error: %v", err))
	} else {
		report.UnlockedAccounts = unlockedCount
	}

	// Cleanup old failed login attempts (older than 30 days)
	cleanedLogins, err := s.repo.CleanupOldFailedLoginAttempts(ctx, 30)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("cleanup failed logins error: %v", err))
	} else {
		report.CleanedFailedLogins = cleanedLogins
	}

	// Cleanup old audit logs based on retention policy
	cleanedAudits, err := s.repo.CleanupOldAuditLogs(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("cleanup audit logs error: %v", err))
	} else {
		report.CleanedAuditLogs = cleanedAudits
	}

	// Refresh materialized view
	if err := s.repo.RefreshSecurityEventsSummary(ctx); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("refresh summary error: %v", err))
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	return report, nil
}

func (s *SecurityAuditService) RefreshSecurityEventsSummary(ctx context.Context) (error) {
	return s.repo.RefreshSecurityEventsSummary(ctx)
}
// ================================
// HELPER FUNCTIONS
// ================================

func (s *SecurityAuditService) determineSeverity(eventType, status string) string {
	if status == domain.StatusFailure {
		switch eventType {
		case domain.EventLoginFailed:
			return domain.SeverityWarning
		default:
			return domain.SeverityError
		}
	}

	switch eventType {
	case domain.EventPasswordChanged, domain.EventEmailChanged:
		return domain.SeverityWarning
	case domain.EventAccountDeleted, domain.EventAccountSuspended:
		return domain.SeverityCritical
	default:
		return domain.SeverityInfo
	}
}

func (s *SecurityAuditService) isUnusualLocation(country, city string, recentLogs []*domain.SecurityAuditLog) bool {
	if len(recentLogs) == 0 {
		return false
	}

	// Check if this location was seen in recent logs
	for _, log := range recentLogs {
		if log.Country != nil && *log.Country == country {
			return false
		}
	}

	return true
}

func (s *SecurityAuditService) hasRapidIPChanges(logs []*domain.SecurityAuditLog) bool {
	if len(logs) < 3 {
		return false
	}

	ipMap := make(map[string]bool)
	for i := 0; i < min(5, len(logs)); i++ {
		if logs[i].IPAddress != nil {
			ipMap[*logs[i].IPAddress] = true
		}
	}

	return len(ipMap) >= 3
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringPtr(s string) *string {
	return &s
}

// ================================
// CONTEXT & REPORT TYPES
// ================================

type AuditContext struct {
	SessionID     *string
	IPAddress     *string
	UserAgent     *string
	RequestID     *string
	TargetUserID  *string
	Action        *string
	Description   *string
	Metadata      map[string]interface{}
	PreviousValue map[string]interface{}
	NewValue      map[string]interface{}
	ErrorCode     *string
	ErrorMessage  *string
}

type SecuritySummaryReport struct {
	StartDate      time.Time                        `json:"start_date"`
	EndDate        time.Time                        `json:"end_date"`
	TotalEvents    int64                            `json:"total_events"`
	TotalFailures  int64                            `json:"total_failures"`
	CriticalEvents int64                            `json:"critical_events"`
	Events         []*domain.SecurityEventsSummary  `json:"events"`
	HighRiskUsers  []*domain.SuspiciousActivity     `json:"high_risk_users"`
}

type MaintenanceReport struct {
	StartTime            time.Time     `json:"start_time"`
	EndTime              time.Time     `json:"end_time"`
	Duration             time.Duration `json:"duration"`
	UnlockedAccounts     int64         `json:"unlocked_accounts"`
	CleanedFailedLogins  int64         `json:"cleaned_failed_logins"`
	CleanedAuditLogs     int64         `json:"cleaned_audit_logs"`
	Errors               []string      `json:"errors,omitempty"`
}

// ================================
// IP VALIDATION HELPER
// ================================

func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}
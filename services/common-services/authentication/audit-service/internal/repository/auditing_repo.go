// security_audit_repository.go under services/audit-service/internal/repository
package repository

import (
	"audit-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	//"strings"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jackc/pgx/v5"
)

// ================================
// SECURITY AUDIT LOG OPERATIONS
// ================================
type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// CreateSecurityAuditLog creates a new security audit log entry
func (r *UserRepository) CreateSecurityAuditLog(ctx context.Context, log *domain.SecurityAuditLog) error {
	query := `
		INSERT INTO security_audit_log (
			event_type, event_category, severity, status,
			user_id, target_user_id, session_id, ip_address, user_agent, request_id,
			resource_type, resource_id, action, description,
			metadata, previous_value, new_value,
			error_code, error_message, country, city
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING id, created_at
	`

	var userID, targetUserID *int64
	if log.UserID != nil {
		uid, _ := strconv.ParseInt(*log.UserID, 10, 64)
		userID = &uid
	}
	if log.TargetUserID != nil {
		tuid, _ := strconv.ParseInt(*log.TargetUserID, 10, 64)
		targetUserID = &tuid
	}

	metadataJSON, _ := json.Marshal(log.Metadata)
	prevValueJSON, _ := json.Marshal(log.PreviousValue)
	newValueJSON, _ := json.Marshal(log.NewValue)

	var id int64
	err := r.db.QueryRow(ctx, query,
		log.EventType,
		log.EventCategory,
		coalesce(log.Severity, domain.SeverityInfo),
		coalesce(log.Status, domain.StatusSuccess),
		userID,
		targetUserID,
		log.SessionID,
		log.IPAddress,
		log.UserAgent,
		log.RequestID,
		log.ResourceType,
		log.ResourceID,
		log.Action,
		log.Description,
		metadataJSON,
		prevValueJSON,
		newValueJSON,
		log.ErrorCode,
		log.ErrorMessage,
		log.Country,
		log.City,
	).Scan(&id, &log.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create security audit log: %w", err)
	}

	log.ID = strconv.FormatInt(id, 10)
	return nil
}

// GetSecurityAuditLogs retrieves audit logs with filtering
func (r *UserRepository) GetSecurityAuditLogs(ctx context.Context, query *domain.AuditLogQuery) ([]*domain.SecurityAuditLog, error) {
	sqlQuery := `
		SELECT 
			id, event_type, event_category, severity, status,
			user_id, target_user_id, session_id, ip_address, user_agent, request_id,
			resource_type, resource_id, action, description,
			metadata, previous_value, new_value,
			error_code, error_message, country, city, created_at
		FROM security_audit_log
		WHERE 1=1
	`

	args := make([]interface{}, 0)
	argIdx := 1

	// Build dynamic WHERE clause
	if query.UserID != nil {
		uid, _ := strconv.ParseInt(*query.UserID, 10, 64)
		sqlQuery += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, uid)
		argIdx++
	}

	if query.EventCategory != nil {
		sqlQuery += fmt.Sprintf(" AND event_category = $%d", argIdx)
		args = append(args, *query.EventCategory)
		argIdx++
	}

	if query.EventType != nil {
		sqlQuery += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, *query.EventType)
		argIdx++
	}

	if query.Severity != nil {
		sqlQuery += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, *query.Severity)
		argIdx++
	}

	if query.Status != nil {
		sqlQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *query.Status)
		argIdx++
	}

	if query.StartDate != nil {
		sqlQuery += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *query.StartDate)
		argIdx++
	}

	if query.EndDate != nil {
		sqlQuery += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *query.EndDate)
		argIdx++
	}

	sqlQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, query.Limit, query.Offset)

	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query security audit logs: %w", err)
	}
	defer rows.Close()

	return r.scanSecurityAuditLogs(rows)
}

// GetUserAuditHistory retrieves audit history for a specific user
func (r *UserRepository) GetUserAuditHistory(ctx context.Context, userID string, limit int) ([]*domain.SecurityAuditLog, error) {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		SELECT 
			id, event_type, event_category, severity, status,
			user_id, target_user_id, session_id, ip_address, user_agent, request_id,
			resource_type, resource_id, action, description,
			metadata, previous_value, new_value,
			error_code, error_message, country, city, created_at
		FROM security_audit_log
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, uid, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query user audit history: %w", err)
	}
	defer rows.Close()

	return r.scanSecurityAuditLogs(rows)
}

// GetCriticalSecurityEvents retrieves recent critical security events
func (r *UserRepository) GetCriticalSecurityEvents(ctx context.Context, hours int, limit int) ([]*domain.SecurityAuditLog, error) {
	query := `
		SELECT 
			id, event_type, event_category, severity, status,
			user_id, target_user_id, session_id, ip_address, user_agent, request_id,
			resource_type, resource_id, action, description,
			metadata, previous_value, new_value,
			error_code, error_message, country, city, created_at
		FROM security_audit_log
		WHERE severity = $1
		  AND created_at > NOW() - ($2 || ' hours')::INTERVAL
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, domain.SeverityCritical, hours, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query critical events: %w", err)
	}
	defer rows.Close()

	return r.scanSecurityAuditLogs(rows)
}

func (r *UserRepository) scanSecurityAuditLogs(rows pgx.Rows) ([]*domain.SecurityAuditLog, error) {
	var logs []*domain.SecurityAuditLog

	for rows.Next() {
		var log domain.SecurityAuditLog
		var id int64
		var userID, targetUserID *int64
		var metadataJSON, prevValueJSON, newValueJSON []byte

		err := rows.Scan(
			&id,
			&log.EventType,
			&log.EventCategory,
			&log.Severity,
			&log.Status,
			&userID,
			&targetUserID,
			&log.SessionID,
			&log.IPAddress,
			&log.UserAgent,
			&log.RequestID,
			&log.ResourceType,
			&log.ResourceID,
			&log.Action,
			&log.Description,
			&metadataJSON,
			&prevValueJSON,
			&newValueJSON,
			&log.ErrorCode,
			&log.ErrorMessage,
			&log.Country,
			&log.City,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan security audit log: %w", err)
		}

		log.ID = strconv.FormatInt(id, 10)
		if userID != nil {
			uid := strconv.FormatInt(*userID, 10)
			log.UserID = &uid
		}
		if targetUserID != nil {
			tuid := strconv.FormatInt(*targetUserID, 10)
			log.TargetUserID = &tuid
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &log.Metadata)
		}
		if len(prevValueJSON) > 0 {
			json.Unmarshal(prevValueJSON, &log.PreviousValue)
		}
		if len(newValueJSON) > 0 {
			json.Unmarshal(newValueJSON, &log.NewValue)
		}

		logs = append(logs, &log)
	}

	return logs, rows.Err()
}

// ================================
// FAILED LOGIN ATTEMPTS
// ================================

// RecordFailedLoginAttempt records a failed login attempt
func (r *UserRepository) RecordFailedLoginAttempt(ctx context.Context, attempt *domain.FailedLoginAttempt) error {
	query := `
		INSERT INTO failed_login_attempts (
			identifier, identifier_type, ip_address, user_agent, failure_reason
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, attempted_at
	`

	var id int64
	err := r.db.QueryRow(ctx, query,
		attempt.Identifier,
		attempt.IdentifierType,
		attempt.IPAddress,
		attempt.UserAgent,
		attempt.FailureReason,
	).Scan(&id, &attempt.AttemptedAt)

	if err != nil {
		return fmt.Errorf("failed to record failed login attempt: %w", err)
	}

	attempt.ID = strconv.FormatInt(id, 10)
	return nil
}

// CountRecentFailedLogins counts failed login attempts within a time window
func (r *UserRepository) CountRecentFailedLogins(ctx context.Context, identifier string, minutes int) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM failed_login_attempts 
		WHERE identifier = $1 
		  AND attempted_at > NOW() - ($2 || ' minutes')::INTERVAL
	`

	var count int
	err := r.db.QueryRow(ctx, query, identifier, minutes).Scan(&count)
	return count, err
}

// CountRecentFailedLoginsByIP counts failed attempts from an IP address
func (r *UserRepository) CountRecentFailedLoginsByIP(ctx context.Context, ipAddress string, minutes int) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM failed_login_attempts 
		WHERE ip_address = $1 
		  AND attempted_at > NOW() - ($2 || ' minutes')::INTERVAL
	`

	var count int
	err := r.db.QueryRow(ctx, query, ipAddress, minutes).Scan(&count)
	return count, err
}

// GetFailedLoginAttempts retrieves recent failed login attempts
func (r *UserRepository) GetFailedLoginAttempts(ctx context.Context, identifier string, limit int) ([]*domain.FailedLoginAttempt, error) {
	query := `
		SELECT id, identifier, identifier_type, ip_address, user_agent, failure_reason, attempted_at
		FROM failed_login_attempts
		WHERE identifier = $1
		ORDER BY attempted_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, identifier, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed login attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.FailedLoginAttempt
	for rows.Next() {
		var attempt domain.FailedLoginAttempt
		var id int64

		err := rows.Scan(
			&id,
			&attempt.Identifier,
			&attempt.IdentifierType,
			&attempt.IPAddress,
			&attempt.UserAgent,
			&attempt.FailureReason,
			&attempt.AttemptedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan failed login attempt: %w", err)
		}

		attempt.ID = strconv.FormatInt(id, 10)
		attempts = append(attempts, &attempt)
	}

	return attempts, rows.Err()
}

// CleanupOldFailedLoginAttempts removes old failed login records
func (r *UserRepository) CleanupOldFailedLoginAttempts(ctx context.Context, days int) (int64, error) {
	query := `
		DELETE FROM failed_login_attempts
		WHERE attempted_at < NOW() - ($1 || ' days')::INTERVAL
	`

	result, err := r.db.Exec(ctx, query, days)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup failed login attempts: %w", err)
	}

	return result.RowsAffected(), nil
}

// ================================
// ACCOUNT LOCKOUT OPERATIONS
// ================================

// CreateAccountLockout creates a new account lockout
func (r *UserRepository) CreateAccountLockout(ctx context.Context, lockout *domain.AccountLockout) error {
	query := `
		INSERT INTO account_lockouts (
			user_id, reason, locked_by, unlock_at, metadata
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, locked_at
	`

	userID, _ := strconv.ParseInt(lockout.UserID, 10, 64)
	var lockedBy *int64
	if lockout.LockedBy != nil {
		lb, _ := strconv.ParseInt(*lockout.LockedBy, 10, 64)
		lockedBy = &lb
	}

	metadataJSON, _ := json.Marshal(lockout.Metadata)

	var id int64
	err := r.db.QueryRow(ctx, query,
		userID,
		lockout.Reason,
		lockedBy,
		lockout.UnlockAt,
		metadataJSON,
	).Scan(&id, &lockout.LockedAt)

	if err != nil {
		return fmt.Errorf("failed to create account lockout: %w", err)
	}

	lockout.ID = strconv.FormatInt(id, 10)
	lockout.IsActive = true
	return nil
}

// IsAccountLocked checks if an account is currently locked
func (r *UserRepository) IsAccountLocked(ctx context.Context, userID string) (bool, error) {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM account_lockouts 
			WHERE user_id = $1 
			  AND is_active = true
			  AND (unlock_at IS NULL OR unlock_at > NOW())
		)
	`

	var isLocked bool
	err := r.db.QueryRow(ctx, query, uid).Scan(&isLocked)
	return isLocked, err
}

// GetActiveLockout retrieves the active lockout for a user
func (r *UserRepository) GetActiveLockout(ctx context.Context, userID string) (*domain.AccountLockout, error) {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		SELECT 
			id, user_id, reason, locked_by, locked_at, unlock_at, 
			unlocked_at, unlocked_by, is_active, metadata
		FROM account_lockouts
		WHERE user_id = $1 
		  AND is_active = true
		ORDER BY locked_at DESC
		LIMIT 1
	`

	var lockout domain.AccountLockout
	var id, userID64 int64
	var lockedBy, unlockedBy *int64
	var metadataJSON []byte

	err := r.db.QueryRow(ctx, query, uid).Scan(
		&id,
		&userID64,
		&lockout.Reason,
		&lockedBy,
		&lockout.LockedAt,
		&lockout.UnlockAt,
		&lockout.UnlockedAt,
		&unlockedBy,
		&lockout.IsActive,
		&metadataJSON,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active lockout: %w", err)
	}

	lockout.ID = strconv.FormatInt(id, 10)
	lockout.UserID = strconv.FormatInt(userID64, 10)
	if lockedBy != nil {
		lb := strconv.FormatInt(*lockedBy, 10)
		lockout.LockedBy = &lb
	}
	if unlockedBy != nil {
		ub := strconv.FormatInt(*unlockedBy, 10)
		lockout.UnlockedBy = &ub
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &lockout.Metadata)
	}

	return &lockout, nil
}

// UnlockAccount unlocks a user account
func (r *UserRepository) UnlockAccount(ctx context.Context, userID, unlockedBy string) error {
	uid, _ := strconv.ParseInt(userID, 10, 64)
	ub, _ := strconv.ParseInt(unlockedBy, 10, 64)

	query := `
		UPDATE account_lockouts
		SET 
			is_active = false,
			unlocked_at = NOW(),
			unlocked_by = $1
		WHERE user_id = $2 AND is_active = true
	`

	_, err := r.db.Exec(ctx, query, ub, uid)
	return err
}

// AutoUnlockExpiredLockouts automatically unlocks accounts whose lockout period has expired
func (r *UserRepository) AutoUnlockExpiredLockouts(ctx context.Context) (int64, error) {
	query := `
		UPDATE account_lockouts
		SET 
			is_active = false,
			unlocked_at = NOW()
		WHERE is_active = true
		  AND unlock_at IS NOT NULL
		  AND unlock_at <= NOW()
	`

	result, err := r.db.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to auto-unlock expired lockouts: %w", err)
	}

	return result.RowsAffected(), nil
}

// ================================
// SUSPICIOUS ACTIVITY OPERATIONS
// ================================

// CreateSuspiciousActivity records suspicious activity
func (r *UserRepository) CreateSuspiciousActivity(ctx context.Context, activity *domain.SuspiciousActivity) error {
	query := `
		INSERT INTO suspicious_activity (
			user_id, activity_type, risk_score, ip_address, details
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	userID, _ := strconv.ParseInt(activity.UserID, 10, 64)
	detailsJSON, _ := json.Marshal(activity.Details)

	var id int64
	err := r.db.QueryRow(ctx, query,
		userID,
		activity.ActivityType,
		activity.RiskScore,
		activity.IPAddress,
		detailsJSON,
	).Scan(&id, &activity.CreatedAt, &activity.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create suspicious activity: %w", err)
	}

	activity.ID = strconv.FormatInt(id, 10)
	activity.Status = domain.ActivityStatusActive
	return nil
}

// GetActiveSuspiciousActivities retrieves active suspicious activities for a user
func (r *UserRepository) GetActiveSuspiciousActivities(ctx context.Context, userID string) ([]*domain.SuspiciousActivity, error) {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		SELECT 
			id, user_id, activity_type, risk_score, ip_address, details,
			status, resolved_by, resolved_at, created_at, updated_at
		FROM suspicious_activity
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to query suspicious activities: %w", err)
	}
	defer rows.Close()

	return r.scanSuspiciousActivities(rows)
}

// GetHighRiskActivities retrieves high-risk suspicious activities across all users
func (r *UserRepository) GetHighRiskActivities(ctx context.Context, minRiskScore int, limit int) ([]*domain.SuspiciousActivity, error) {
	query := `
		SELECT 
			id, user_id, activity_type, risk_score, ip_address, details,
			status, resolved_by, resolved_at, created_at, updated_at
		FROM suspicious_activity
		WHERE status = 'active' AND risk_score >= $1
		ORDER BY risk_score DESC, created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, minRiskScore, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query high-risk activities: %w", err)
	}
	defer rows.Close()

	return r.scanSuspiciousActivities(rows)
}

// ResolveSuspiciousActivity marks a suspicious activity as resolved
func (r *UserRepository) ResolveSuspiciousActivity(ctx context.Context, activityID, resolvedBy string, status string) error {
	aid, _ := strconv.ParseInt(activityID, 10, 64)
	rb, _ := strconv.ParseInt(resolvedBy, 10, 64)

	query := `
		UPDATE suspicious_activity
		SET 
			status = $1,
			resolved_by = $2,
			resolved_at = NOW(),
			updated_at = NOW()
		WHERE id = $3
	`

	_, err := r.db.Exec(ctx, query, status, rb, aid)
	return err
}

func (r *UserRepository) scanSuspiciousActivities(rows pgx.Rows) ([]*domain.SuspiciousActivity, error) {
	var activities []*domain.SuspiciousActivity

	for rows.Next() {
		var activity domain.SuspiciousActivity
		var id, userID64 int64
		var resolvedBy *int64
		var detailsJSON []byte

		err := rows.Scan(
			&id,
			&userID64,
			&activity.ActivityType,
			&activity.RiskScore,
			&activity.IPAddress,
			&detailsJSON,
			&activity.Status,
			&resolvedBy,
			&activity.ResolvedAt,
			&activity.CreatedAt,
			&activity.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan suspicious activity: %w", err)
		}

		activity.ID = strconv.FormatInt(id, 10)
		activity.UserID = strconv.FormatInt(userID64, 10)
		if resolvedBy != nil {
			rb := strconv.FormatInt(*resolvedBy, 10)
			activity.ResolvedBy = &rb
		}
		if len(detailsJSON) > 0 {
			json.Unmarshal(detailsJSON, &activity.Details)
		}

		activities = append(activities, &activity)
	}

	return activities, rows.Err()
}

// ================================
// ANALYTICS & REPORTING
// ================================

// GetSecurityEventsSummary retrieves security events summary
func (r *UserRepository) GetSecurityEventsSummary(ctx context.Context, startDate, endDate time.Time) ([]*domain.SecurityEventsSummary, error) {
	query := `
		SELECT 
			DATE(created_at) as event_date,
			event_category,
			event_type,
			status,
			severity,
			COUNT(*) as event_count,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT ip_address) as unique_ips
		FROM security_audit_log
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at), event_category, event_type, status, severity
		ORDER BY event_date DESC, event_count DESC
	`

	rows, err := r.db.Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query security events summary: %w", err)
	}
	defer rows.Close()

	var summaries []*domain.SecurityEventsSummary
	for rows.Next() {
		var summary domain.SecurityEventsSummary
		err := rows.Scan(
			&summary.EventDate,
			&summary.EventCategory,
			&summary.EventType,
			&summary.Status,
			&summary.Severity,
			&summary.EventCount,
			&summary.UniqueUsers,
			&summary.UniqueIPs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan security events summary: %w", err)
		}
		summaries = append(summaries, &summary)
	}

	return summaries, rows.Err()
}

// GetUserRiskScore calculates a user's risk score
func (r *UserRepository) GetUserRiskScore(ctx context.Context, userID string) (int, error) {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `SELECT get_user_risk_score($1)`

	var riskScore int
	err := r.db.QueryRow(ctx, query, uid).Scan(&riskScore)
	return riskScore, err
}

// ================================
// CLEANUP OPERATIONS
// ================================

// CleanupOldAuditLogs removes audit logs based on retention policy
func (r *UserRepository) CleanupOldAuditLogs(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM security_audit_log sal
		USING audit_retention_policy arp
		WHERE sal.event_category = arp.event_category
		  AND sal.created_at < NOW() - (arp.retention_days || ' days')::INTERVAL
	`

	result, err := r.db.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old audit logs: %w", err)
	}

	return result.RowsAffected(), nil
}

// RefreshSecurityEventsSummary refreshes the materialized view
func (r *UserRepository) RefreshSecurityEventsSummary(ctx context.Context) error {
	_, err := r.db.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY security_events_summary")
	return err
}

// ================================
// HELPER FUNCTIONS
// ================================

func coalesce(val, defaultVal string) string {
	if val == "" {
		return defaultVal
	}
	return val
}
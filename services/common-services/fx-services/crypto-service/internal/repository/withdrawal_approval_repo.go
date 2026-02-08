// internal/repository/withdrawal_approval_repository.go
package repository

import (
	"context"
	"crypto-service/internal/domain"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type WithdrawalApprovalRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewWithdrawalApprovalRepository(pool *pgxpool.Pool, logger *zap.Logger) *WithdrawalApprovalRepository {
	return &WithdrawalApprovalRepository{
		pool:   pool,
		logger: logger,
	}
}

// ============================================================================
// HELPER FUNCTIONS (Amount Parsing)
// ============================================================================

// parseBigIntFromNumeric safely parses a numeric string to *big.Int
// Handles cases like "20000.0000", "20000", "1.5e6", etc.
func parseBigIntFromNumeric(numericStr string) (*big.Int, error) {
	if numericStr == "" {
		return big.NewInt(0), nil
	}

	// Trim whitespace
	numericStr = strings.TrimSpace(numericStr)

	// Try direct parsing first (for integers without decimals)
	if result, ok := new(big.Int).SetString(numericStr, 10); ok {
		return result, nil
	}

	// Handle decimal/float strings
	// Parse as big.Float first, then convert to big.Int
	floatVal, _, err := big.ParseFloat(numericStr, 10, 256, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("failed to parse numeric string '%s': %w", numericStr, err)
	}

	// Convert to big.Int (truncates decimal part)
	result, accuracy := floatVal.Int(nil)
	
	// Log if we lost precision
	if accuracy != big.Exact {
		// This is expected for values like "20000.0000" -> 20000
		// Only log if it's not just trailing zeros
		if !strings.Contains(numericStr, ".") || !strings.HasSuffix(strings.TrimRight(numericStr, "0"), ".") {
			// Has non-zero decimal part - potential data loss
			return result, fmt.Errorf("precision loss when parsing '%s': decimal part truncated", numericStr)
		}
	}

	return result, nil
}

// parseBigIntSafe is a safe wrapper that logs errors and returns zero on failure
func (r *WithdrawalApprovalRepository) parseBigIntSafe(numericStr string, context string) *big.Int {
	result, err := parseBigIntFromNumeric(numericStr)
	if err != nil {
		r.logger.Warn("Failed to parse amount",
			zap.String("context", context),
			zap.String("value", numericStr),
			zap.Error(err))
		return big.NewInt(0)
	}
	return result
}

// ============================================================================
// CREATE OPERATIONS
// ============================================================================

// Create creates a new withdrawal approval record
func (r *WithdrawalApprovalRepository) Create(ctx context.Context, approval *domain.WithdrawalApproval) error {
	query := `
		INSERT INTO withdrawal_approvals (
			transaction_id, user_id, amount, asset,
			to_address, risk_score, risk_factors,
			requires_approval, status, auto_approved_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	riskFactorsJSON, err := json.Marshal(approval.RiskFactors)
	if err != nil {
		return fmt.Errorf("failed to marshal risk factors: %w", err)
	}

	err = r.pool.QueryRow(ctx, query,
		approval.TransactionID,
		approval.UserID,
		approval.Amount.String(),
		approval.Asset,
		approval.ToAddress,
		approval.RiskScore,
		riskFactorsJSON,
		approval.RequiresApproval,
		approval.Status,
		approval.AutoApprovedReason,
	).Scan(&approval.ID, &approval.CreatedAt, &approval.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create approval: %w", err)
	}

	r.logger.Info("Withdrawal approval created",
		zap.Int64("approval_id", approval.ID),
		zap.Int64("transaction_id", approval.TransactionID),
		zap.String("user_id", approval.UserID),
		zap.String("status", string(approval.Status)))

	return nil
}

// ============================================================================
// READ OPERATIONS
// ============================================================================

// GetByID gets approval by ID
func (r *WithdrawalApprovalRepository) GetByID(ctx context.Context, id int64) (*domain.WithdrawalApproval, error) {
	query := `
		SELECT 
			wa.id, wa.transaction_id, wa.user_id, wa.amount, wa.asset,
			wa.to_address, wa.risk_score, wa.risk_factors,
			wa.requires_approval, wa.status, wa.approved_by,
			wa.approved_at, wa.rejection_reason, wa.auto_approved_reason,
			wa.created_at, wa.updated_at
		FROM withdrawal_approvals wa
		WHERE wa.id = $1
	`

	approval, err := r.scanApprovalRow(r.pool.QueryRow(ctx, query, id))
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("approval not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get approval: %w", err)
	}

	return approval, nil
}

// GetByTransactionID gets approval by transaction ID
func (r *WithdrawalApprovalRepository) GetByTransactionID(ctx context.Context, txID int64) (*domain.WithdrawalApproval, error) {
	query := `
		SELECT 
			wa.id, wa.transaction_id, wa.user_id, wa.amount, wa.asset,
			wa.to_address, wa.risk_score, wa.risk_factors,
			wa.requires_approval, wa.status, wa.approved_by,
			wa.approved_at, wa.rejection_reason, wa.auto_approved_reason,
			wa.created_at, wa.updated_at
		FROM withdrawal_approvals wa
		WHERE wa.transaction_id = $1
	`

	approval, err := r.scanApprovalRow(r.pool.QueryRow(ctx, query, txID))
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("approval not found for transaction")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get approval by transaction: %w", err)
	}

	return approval, nil
}

// GetPendingApprovals gets all pending approval requests
func (r *WithdrawalApprovalRepository) GetPendingApprovals(ctx context.Context, limit, offset int) ([]*domain.WithdrawalApproval, error) {
	query := `
		SELECT 
			wa.id, wa.transaction_id, wa.user_id, wa.amount, wa.asset,
			wa.to_address, wa.risk_score, wa.risk_factors,
			wa.requires_approval, wa.status, wa.approved_by,
			wa.approved_at, wa.rejection_reason, wa.auto_approved_reason,
			wa.created_at, wa.updated_at
		FROM withdrawal_approvals wa
		WHERE wa.status = 'pending_review'
		ORDER BY wa.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending approvals: %w", err)
	}
	defer rows.Close()

	var approvals []*domain.WithdrawalApproval
	for rows.Next() {
		approval, err := r.scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating approvals: %w", err)
	}

	r.logger.Debug("Retrieved pending approvals",
		zap.Int("count", len(approvals)),
		zap.Int("limit", limit),
		zap.Int("offset", offset))

	return approvals, nil
}

// GetAllPendingApprovals gets all pending approvals (no pagination)
func (r *WithdrawalApprovalRepository) GetAllPendingApprovals(ctx context.Context) ([]*domain.WithdrawalApproval, error) {
	query := `
		SELECT 
			wa.id, wa.transaction_id, wa.user_id, wa.amount, wa.asset,
			wa.to_address, wa.risk_score, wa.risk_factors,
			wa.requires_approval, wa.status, wa.approved_by,
			wa.approved_at, wa.rejection_reason, wa.auto_approved_reason,
			wa.created_at, wa.updated_at
		FROM withdrawal_approvals wa
		WHERE wa.status = 'pending_review'
		ORDER BY wa.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all pending approvals: %w", err)
	}
	defer rows.Close()

	var approvals []*domain.WithdrawalApproval
	for rows.Next() {
		approval, err := r.scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}

	return approvals, nil
}

// GetApprovalsByUser gets approvals for a specific user
func (r *WithdrawalApprovalRepository) GetApprovalsByUser(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*domain.WithdrawalApproval, error) {
	query := `
		SELECT 
			wa.id, wa.transaction_id, wa.user_id, wa.amount, wa.asset,
			wa.to_address, wa.risk_score, wa.risk_factors,
			wa.requires_approval, wa.status, wa.approved_by,
			wa.approved_at, wa.rejection_reason, wa.auto_approved_reason,
			wa.created_at, wa.updated_at
		FROM withdrawal_approvals wa
		WHERE wa.user_id = $1
		ORDER BY wa.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query user approvals: %w", err)
	}
	defer rows.Close()

	var approvals []*domain.WithdrawalApproval
	for rows.Next() {
		approval, err := r.scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}

	return approvals, nil
}

// GetApprovalsByStatus gets approvals by status
func (r *WithdrawalApprovalRepository) GetApprovalsByStatus(
	ctx context.Context,
	status domain.ApprovalStatus,
	limit, offset int,
) ([]*domain.WithdrawalApproval, error) {
	query := `
		SELECT 
			wa.id, wa.transaction_id, wa.user_id, wa.amount, wa.asset,
			wa.to_address, wa.risk_score, wa.risk_factors,
			wa.requires_approval, wa.status, wa.approved_by,
			wa.approved_at, wa.rejection_reason, wa.auto_approved_reason,
			wa.created_at, wa.updated_at
		FROM withdrawal_approvals wa
		WHERE wa.status = $1
		ORDER BY wa.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query approvals by status: %w", err)
	}
	defer rows.Close()

	var approvals []*domain.WithdrawalApproval
	for rows.Next() {
		approval, err := r.scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}

	return approvals, nil
}

// GetHighRiskApprovals gets approvals with risk score above threshold
func (r *WithdrawalApprovalRepository) GetHighRiskApprovals(
	ctx context.Context,
	minRiskScore int,
	limit, offset int,
) ([]*domain.WithdrawalApproval, error) {
	query := `
		SELECT 
			wa.id, wa.transaction_id, wa.user_id, wa.amount, wa.asset,
			wa.to_address, wa.risk_score, wa.risk_factors,
			wa.requires_approval, wa.status, wa.approved_by,
			wa.approved_at, wa.rejection_reason, wa.auto_approved_reason,
			wa.created_at, wa.updated_at
		FROM withdrawal_approvals wa
		WHERE wa.risk_score >= $1
		  AND wa.status = 'pending_review'
		ORDER BY wa.risk_score DESC, wa.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, minRiskScore, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query high risk approvals: %w", err)
	}
	defer rows.Close()

	var approvals []*domain.WithdrawalApproval
	for rows.Next() {
		approval, err := r.scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}

	return approvals, nil
}

// ============================================================================
// UPDATE OPERATIONS
// ============================================================================

// Approve marks approval as approved
func (r *WithdrawalApprovalRepository) Approve(ctx context.Context, id int64, approvedBy, notes string) error {
	query := `
		UPDATE withdrawal_approvals
		SET status = 'approved',
			approved_by = $2,
			approved_at = NOW(),
			rejection_reason = $3,
			updated_at = NOW()
		WHERE id = $1 AND status = 'pending_review'
	`

	result, err := r.pool.Exec(ctx, query, id, approvedBy, notes)
	if err != nil {
		return fmt.Errorf("failed to approve: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("approval not found or already processed")
	}

	r.logger.Info("Withdrawal approval approved",
		zap.Int64("approval_id", id),
		zap.String("approved_by", approvedBy))

	return nil
}

// Reject marks approval as rejected
func (r *WithdrawalApprovalRepository) Reject(ctx context.Context, id int64, rejectedBy, reason string) error {
	query := `
		UPDATE withdrawal_approvals
		SET status = 'rejected',
			approved_by = $2,
			rejection_reason = $3,
			updated_at = NOW()
		WHERE id = $1 AND status = 'pending_review'
	`

	result, err := r.pool.Exec(ctx, query, id, rejectedBy, reason)
	if err != nil {
		return fmt.Errorf("failed to reject: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("approval not found or already processed")
	}

	r.logger.Info("Withdrawal approval rejected",
		zap.Int64("approval_id", id),
		zap.String("rejected_by", rejectedBy),
		zap.String("reason", reason))

	return nil
}

// UpdateStatus updates the approval status
func (r *WithdrawalApprovalRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status domain.ApprovalStatus,
) error {
	query := `
		UPDATE withdrawal_approvals
		SET status = $2,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("approval not found")
	}

	return nil
}

// ============================================================================
// STATISTICS OPERATIONS
// ============================================================================

// GetApprovalStats gets approval statistics
func (r *WithdrawalApprovalRepository) GetApprovalStats(ctx context.Context) (*ApprovalStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'pending_review') as pending,
			COUNT(*) FILTER (WHERE status = 'approved') as approved,
			COUNT(*) FILTER (WHERE status = 'rejected') as rejected,
			COUNT(*) FILTER (WHERE status = 'auto_approved') as auto_approved,
			COUNT(*) FILTER (WHERE status = 'pending_review' AND risk_score >= 61) as high_risk_pending,
			COALESCE(AVG(risk_score), 0) as avg_risk_score
		FROM withdrawal_approvals
	`

	stats := &ApprovalStats{}
	var avgRiskScore float64

	err := r.pool.QueryRow(ctx, query).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Approved,
		&stats.Rejected,
		&stats.AutoApproved,
		&stats.HighRiskPending,
		&avgRiskScore,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get approval stats: %w", err)
	}

	stats.AvgRiskScore = avgRiskScore

	return stats, nil
}

// GetPendingCount gets count of pending approvals
func (r *WithdrawalApprovalRepository) GetPendingCount(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM withdrawal_approvals
		WHERE status = 'pending_review'
	`

	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending count: %w", err)
	}

	return count, nil
}

// ============================================================================
// SCAN HELPER FUNCTIONS
// ============================================================================

// scanApproval scans a row from pgx.Rows
func (r *WithdrawalApprovalRepository) scanApproval(rows pgx.Rows) (*domain.WithdrawalApproval, error) {
	approval := &domain.WithdrawalApproval{}
	var (
		amountStr       string
		riskFactorsJSON []byte
		approvedBy      *string
		approvedAt      *time.Time
		rejectionReason *string
		autoApproved    *string
	)

	err := rows.Scan(
		&approval.ID,
		&approval.TransactionID,
		&approval.UserID,
		&amountStr,
		&approval.Asset,
		&approval.ToAddress,
		&approval.RiskScore,
		&riskFactorsJSON,
		&approval.RequiresApproval,
		&approval.Status,
		&approvedBy,
		&approvedAt,
		&rejectionReason,
		&autoApproved,
		&approval.CreatedAt,
		&approval.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan approval: %w", err)
	}

	//  Parse amount with robust helper
	approval.Amount = r.parseBigIntSafe(amountStr, "approval_amount")

	// Parse risk factors
	if len(riskFactorsJSON) > 0 {
		if err := json.Unmarshal(riskFactorsJSON, &approval.RiskFactors); err != nil {
			r.logger.Warn("Failed to unmarshal risk factors", zap.Error(err))
			approval.RiskFactors = []domain.RiskFactor{}
		}
	}

	// Assign nullable fields
	approval.ApprovedBy = approvedBy
	approval.ApprovedAt = approvedAt
	approval.RejectionReason = rejectionReason
	approval.AutoApprovedReason = autoApproved

	return approval, nil
}

// scanApprovalRow scans a single row from pgx.Row
func (r *WithdrawalApprovalRepository) scanApprovalRow(row pgx.Row) (*domain.WithdrawalApproval, error) {
	approval := &domain.WithdrawalApproval{}
	var (
		amountStr       string
		riskFactorsJSON []byte
		approvedBy      *string
		approvedAt      *time.Time
		rejectionReason *string
		autoApproved    *string
	)

	err := row.Scan(
		&approval.ID,
		&approval.TransactionID,
		&approval.UserID,
		&amountStr,
		&approval.Asset,
		&approval.ToAddress,
		&approval.RiskScore,
		&riskFactorsJSON,
		&approval.RequiresApproval,
		&approval.Status,
		&approvedBy,
		&approvedAt,
		&rejectionReason,
		&autoApproved,
		&approval.CreatedAt,
		&approval.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	//  Parse amount with robust helper
	approval.Amount = r.parseBigIntSafe(amountStr, "approval_amount")

	// Parse risk factors
	if len(riskFactorsJSON) > 0 {
		if err := json.Unmarshal(riskFactorsJSON, &approval.RiskFactors); err != nil {
			r.logger.Warn("Failed to unmarshal risk factors", zap.Error(err))
			approval.RiskFactors = []domain.RiskFactor{}
		}
	}

	// Assign nullable fields
	approval.ApprovedBy = approvedBy
	approval.ApprovedAt = approvedAt
	approval.RejectionReason = rejectionReason
	approval.AutoApprovedReason = autoApproved

	return approval, nil
}

// ============================================================================
// TYPES
// ============================================================================

// ApprovalStats holds approval statistics
type ApprovalStats struct {
	Total           int
	Pending         int
	Approved        int
	Rejected        int
	AutoApproved    int
	HighRiskPending int
	AvgRiskScore    float64
}
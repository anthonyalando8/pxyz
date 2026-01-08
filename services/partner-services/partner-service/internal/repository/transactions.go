package repository

import (
	"context"
	"fmt"
	"partner-service/internal/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateTransaction creates a new partner transaction
func (r *PartnerRepo) CreateTransaction(ctx context.Context, txn *domain.PartnerTransaction) error {
	query := `
		INSERT INTO partner_transactions 
		(partner_id, transaction_ref, user_id, amount, currency, status, payment_method, transaction_type, external_ref, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		txn.PartnerID, 
		txn.TransactionRef, 
		txn.UserID, 
		txn. Amount, 
		txn. Currency,
		txn.Status, 
		txn.PaymentMethod, 
		txn.TransactionType,  // ✅ Added
		txn.ExternalRef, 
		txn.Metadata,
	).Scan(&txn.ID, &txn. CreatedAt, &txn. UpdatedAt)
}

// UpdateTransactionStatus updates the status of a partner transaction
func (r *PartnerRepo) UpdateTransactionStatus(ctx context.Context, txID int64, status, errorMsg string) error {
	query := `
		UPDATE partner_transactions 
		SET 
			status = $1,
			error_message = $2,
			processed_at = CASE 
				WHEN $1 IN ('completed', 'failed') THEN NOW() 
				ELSE processed_at 
			END,
			updated_at = NOW()
		WHERE id = $3
	`
	result, err := r.db.Exec(ctx, query, status, errorMsg, txID)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	rowsAffected := result. RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("transaction not found: id=%d", txID)
	}

	return nil
}

// UpdateTransaction updates a partner transaction with complete data
func (r *PartnerRepo) UpdateTransaction(ctx context.Context, txn *domain.PartnerTransaction) error {
	query := `
		UPDATE partner_transactions 
		SET 
			status = $1,
			payment_method = $2,
			transaction_type = $3,
			external_ref = $4,
			metadata = $5,
			processed_at = $6,
			error_message = $7,
			updated_at = NOW()
		WHERE id = $8
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		txn.Status,
		txn.PaymentMethod,
		txn.TransactionType,  // ✅ Added
		txn.ExternalRef,
		txn.Metadata,
		txn.ProcessedAt,
		txn.ErrorMessage,
		txn.ID,
	).Scan(&txn.UpdatedAt)
}

// UpdateTransactionWithReceipt updates transaction with accounting receipt code
func (r *PartnerRepo) UpdateTransactionWithReceipt(ctx context. Context, txID int64, receiptCode string, journalID int64, status string) error {
	query := `
		UPDATE partner_transactions 
		SET 
			status = $1,
			external_ref = $2,
			metadata = COALESCE(metadata, '{}'::jsonb) || 
				jsonb_build_object('journal_id', $3::bigint) ||
				jsonb_build_object('receipt_code', $2::text),
			processed_at = NOW(),
			updated_at = NOW()
		WHERE id = $4
	`
	result, err := r.db.Exec(ctx, query, status, receiptCode, journalID, txID)
	if err != nil {
		return fmt.Errorf("failed to update transaction with receipt:  %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("transaction not found: id=%d", txID)
	}

	return nil
}

// UpdateTransactionCompletion updates transaction with external reference and completion
func (r *PartnerRepo) UpdateTransactionCompletion(ctx context.Context, txnID int64, externalRef, status string) error {
	query := `
		UPDATE partner_transactions
		SET 
			external_ref = $1,
			status = $2,
			processed_at = NOW(),
			updated_at = NOW()
		WHERE id = $3
	`

	result, err := r.db. Exec(ctx, query, externalRef, status, txnID)
	if err != nil {
		return fmt.Errorf("failed to update transaction completion:  %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found:  %d", txnID)
	}

	return nil
}

// GetTransactionByID retrieves a transaction by its ID
func (r *PartnerRepo) GetTransactionByID(ctx context.Context, txID int64) (*domain.PartnerTransaction, error) {
	query := `
		SELECT 
			id, partner_id, transaction_ref, user_id, amount, currency, 
			status, payment_method, transaction_type, external_ref, metadata, 
			error_message, processed_at, created_at, updated_at
		FROM partner_transactions
		WHERE id = $1
	`
	var txn domain.PartnerTransaction
	err := r.db.QueryRow(ctx, query, txID).Scan(
		&txn.ID,
		&txn.PartnerID,
		&txn.TransactionRef,
		&txn.UserID,
		&txn.Amount,
		&txn.Currency,
		&txn.Status,
		&txn.PaymentMethod,
		&txn.TransactionType,  // ✅ Added
		&txn.ExternalRef,
		&txn. Metadata,
		&txn. ErrorMessage,     // ✅ Added
		&txn.ProcessedAt,
		&txn.CreatedAt,
		&txn. UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("transaction not found")
		}
		return nil, err
	}
	return &txn, nil
}

// GetTransactionByRef retrieves a transaction by partner ID and transaction reference
func (r *PartnerRepo) GetTransactionByRef(ctx context.Context, partnerID, transactionRef string) (*domain.PartnerTransaction, error) {
	query := `
		SELECT 
			id, partner_id, transaction_ref, user_id, amount, currency, 
			status, payment_method, transaction_type, external_ref, metadata, 
			error_message, processed_at, created_at, updated_at
		FROM partner_transactions
		WHERE partner_id = $1 AND transaction_ref = $2
	`
	var txn domain.PartnerTransaction
	err := r.db.QueryRow(ctx, query, partnerID, transactionRef).Scan(
		&txn.ID,
		&txn.PartnerID,
		&txn.TransactionRef,
		&txn.UserID,
		&txn.Amount,
		&txn.Currency,
		&txn.Status,
		&txn.PaymentMethod,
		&txn.TransactionType,  // ✅ Added
		&txn.ExternalRef,
		&txn.Metadata,
		&txn.ErrorMessage,     // ✅ Added
		&txn.ProcessedAt,
		&txn.CreatedAt,
		&txn.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("transaction not found")
		}
		return nil, err
	}
	return &txn, nil
}

// ListTransactions retrieves paginated partner transactions with filters
func (r *PartnerRepo) ListTransactions(ctx context.Context, partnerID string, limit, offset int, status *string) ([]domain.PartnerTransaction, int64, error) {
	var transactions []domain.PartnerTransaction
	var total int64

	// Build query with filters
	baseQuery := `
		SELECT 
			id, partner_id, transaction_ref, user_id, amount, currency, 
			status, payment_method, transaction_type, external_ref, metadata, 
			error_message, processed_at, created_at, updated_at
		FROM partner_transactions
		WHERE partner_id = $1
	`
	countQuery := `SELECT COUNT(*) FROM partner_transactions WHERE partner_id = $1`

	args := []interface{}{partnerID}
	argIndex := 2

	// Add status filter if provided
	if status != nil && *status != "" {
		baseQuery += fmt. Sprintf(" AND status = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	// Get total count
	if err := r. db.QueryRow(ctx, countQuery, args[:argIndex-1]...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Add ordering and pagination
	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	// Execute query
	rows, err := r.db. Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list transactions: %w", err)
	}
	defer rows. Close()

	for rows.Next() {
		var txn domain.PartnerTransaction
		if err := rows.Scan(
			&txn.ID,
			&txn.PartnerID,
			&txn. TransactionRef,
			&txn.UserID,
			&txn.Amount,
			&txn.Currency,
			&txn.Status,
			&txn.PaymentMethod,
			&txn.TransactionType,  // ✅ Added
			&txn.ExternalRef,
			&txn. Metadata,
			&txn.ErrorMessage,     // ✅ Added
			&txn.ProcessedAt,
			&txn.CreatedAt,
			&txn.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, txn)
	}

	if err := rows. Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating transactions: %w", err)
	}

	return transactions, total, nil
}

// GetTransactionStats returns transaction statistics for a partner
func (r *PartnerRepo) GetTransactionStats(ctx context.Context, partnerID string, from, to time.Time) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_count,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_count,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE transaction_type = 'deposit') as deposit_count,
			COUNT(*) FILTER (WHERE transaction_type = 'withdrawal') as withdrawal_count,
			COALESCE(SUM(amount) FILTER (WHERE status = 'completed'), 0) as total_amount,
			COALESCE(SUM(amount) FILTER (WHERE status = 'completed' AND transaction_type = 'deposit'), 0) as total_deposits,
			COALESCE(SUM(amount) FILTER (WHERE status = 'completed' AND transaction_type = 'withdrawal'), 0) as total_withdrawals,
			COALESCE(AVG(amount) FILTER (WHERE status = 'completed'), 0) as avg_amount,
			COALESCE(MIN(amount) FILTER (WHERE status = 'completed'), 0) as min_amount,
			COALESCE(MAX(amount) FILTER (WHERE status = 'completed'), 0) as max_amount
		FROM partner_transactions
		WHERE partner_id = $1 
		AND created_at BETWEEN $2 AND $3
	`
	
	var stats struct {
		TotalCount       int64
		CompletedCount   int64
		FailedCount      int64
		PendingCount     int64
		DepositCount     int64    // ✅ Added
		WithdrawalCount  int64    // ✅ Added
		TotalAmount      float64
		TotalDeposits    float64  // ✅ Added
		TotalWithdrawals float64  // ✅ Added
		AvgAmount        float64
		MinAmount        float64
		MaxAmount        float64
	}

	err := r.db. QueryRow(ctx, query, partnerID, from, to).Scan(
		&stats.TotalCount,
		&stats. CompletedCount,
		&stats.FailedCount,
		&stats.PendingCount,
		&stats.DepositCount,      // ✅ Added
		&stats.WithdrawalCount,   // ✅ Added
		&stats.TotalAmount,
		&stats.TotalDeposits,     // ✅ Added
		&stats.TotalWithdrawals,  // ✅ Added
		&stats.AvgAmount,
		&stats. MinAmount,
		&stats. MaxAmount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction stats: %w", err)
	}

	return map[string]interface{}{
		"total_count":       stats.TotalCount,
		"completed_count":   stats. CompletedCount,
		"failed_count":      stats. FailedCount,
		"pending_count":     stats.PendingCount,
		"deposit_count":     stats.DepositCount,      // ✅ Added
		"withdrawal_count":   stats.WithdrawalCount,   // ✅ Added
		"total_amount":      stats.TotalAmount,
		"total_deposits":    stats. TotalDeposits,     // ✅ Added
		"total_withdrawals": stats.TotalWithdrawals,  // ✅ Added
		"avg_amount":        stats. AvgAmount,
		"min_amount":        stats.MinAmount,
		"max_amount":         stats.MaxAmount,
	}, nil
}

// BulkUpdateTransactionStatus updates status for multiple transactions
func (r *PartnerRepo) BulkUpdateTransactionStatus(ctx context. Context, txIDs []int64, status string) error {
	query := `
		UPDATE partner_transactions 
		SET 
			status = $1,
			processed_at = CASE 
				WHEN $1 IN ('completed', 'failed') THEN NOW() 
				ELSE processed_at 
			END,
			updated_at = NOW()
		WHERE id = ANY($2)
	`
	result, err := r.db.Exec(ctx, query, status, txIDs)
	if err != nil {
		return fmt.Errorf("failed to bulk update transaction status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no transactions updated")
	}

	return nil
}

// ✅ NEW: Get transactions by type
func (r *PartnerRepo) GetTransactionsByType(ctx context.Context, partnerID, transactionType string, limit, offset int) ([]domain.PartnerTransaction, int64, error) {
	var transactions []domain.PartnerTransaction
	var total int64

	// Count query
	countQuery := `
		SELECT COUNT(*) 
		FROM partner_transactions 
		WHERE partner_id = $1 AND transaction_type = $2
	`
	if err := r.db.QueryRow(ctx, countQuery, partnerID, transactionType).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions:  %w", err)
	}

	// Data query
	query := `
		SELECT 
			id, partner_id, transaction_ref, user_id, amount, currency, 
			status, payment_method, transaction_type, external_ref, metadata, 
			error_message, processed_at, created_at, updated_at
		FROM partner_transactions
		WHERE partner_id = $1 AND transaction_type = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Query(ctx, query, partnerID, transactionType, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var txn domain. PartnerTransaction
		if err := rows.Scan(
			&txn.ID,
			&txn.PartnerID,
			&txn.TransactionRef,
			&txn.UserID,
			&txn.Amount,
			&txn.Currency,
			&txn.Status,
			&txn.PaymentMethod,
			&txn.TransactionType,
			&txn.ExternalRef,
			&txn. Metadata,
			&txn.ErrorMessage,
			&txn.ProcessedAt,
			&txn.CreatedAt,
			&txn.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, txn)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating transactions: %w", err)
	}

	return transactions, total, nil
}
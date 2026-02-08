// internal/repository/crypto_deposit_repo.go
package repository

import (
	"context"
	"fmt"
	"math/big"
	"crypto-service/internal/domain"


	//"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CryptoDepositRepository struct {
	pool *pgxpool.Pool
}

func NewCryptoDepositRepository(pool *pgxpool.Pool) *CryptoDepositRepository {
	return &CryptoDepositRepository{pool: pool}
}

// ============================================================================
// CORE CRUD OPERATIONS
// ============================================================================

// Create creates a new deposit record
func (r *CryptoDepositRepository) Create(ctx context.Context, deposit *domain.CryptoDeposit) error {
	query := `
		INSERT INTO crypto_deposits (
			deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, user_notified, notification_sent,
			detected_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7, $8,
			$9, $10, $11,
			$12, $13,
			$14, $15, $16,
			$17
		)
		RETURNING id, created_at, updated_at
	`

	// Generate UUID if not set
	if deposit.DepositID == "" {
		deposit.DepositID = uuid.New().String()
	}

	// Convert amount to string
	amountStr := "0"
	if deposit.Amount != nil {
		amountStr = deposit.Amount.String()
	}

	err := r.pool.QueryRow(
		ctx, query,
		deposit.DepositID,
		deposit.WalletID,
		deposit.UserID,
		deposit.Chain,
		deposit.Asset,
		deposit.FromAddress,
		deposit.ToAddress,
		amountStr,
		deposit.TxHash,
		deposit.BlockNumber,
		deposit.BlockTimestamp,
		deposit.Confirmations,
		deposit.RequiredConfirmations,
		deposit.Status,
		deposit.UserNotified,
		deposit.NotificationSent,
		deposit.DetectedAt,
	).Scan(&deposit.ID, &deposit.CreatedAt, &deposit.UpdatedAt)

	if err != nil {
		// Check for duplicate
		if err.Error() == "duplicate key value violates unique constraint" {
			return fmt.Errorf("deposit already exists")
		}
		return fmt.Errorf("failed to create deposit: %w", err)
	}

	return nil
}

// GetByID retrieves a deposit by ID
func (r *CryptoDepositRepository) GetByID(ctx context.Context, id int64) (*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE id = $1
	`

	deposit := &domain.CryptoDeposit{}
	err := r.scanDeposit(r.pool.QueryRow(ctx, query, id), deposit)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("deposit not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deposit: %w", err)
	}

	return deposit, nil
}

// GetByDepositID retrieves a deposit by UUID
func (r *CryptoDepositRepository) GetByDepositID(ctx context.Context, depositID string) (*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE deposit_id = $1
	`

	deposit := &domain.CryptoDeposit{}
	err := r.scanDeposit(r.pool.QueryRow(ctx, query, depositID), deposit)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("deposit not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deposit: %w", err)
	}

	return deposit, nil
}

// GetByTxHash retrieves a deposit by transaction hash and address
func (r *CryptoDepositRepository) GetByTxHash(ctx context.Context, txHash, toAddress string) (*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE tx_hash = $1 AND to_address = $2
	`

	deposit := &domain.CryptoDeposit{}
	err := r.scanDeposit(r.pool.QueryRow(ctx, query, txHash, toAddress), deposit)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("deposit not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deposit: %w", err)
	}

	return deposit, nil
}

// Update updates a deposit
func (r *CryptoDepositRepository) Update(ctx context.Context, deposit *domain.CryptoDeposit) error {
	query := `
		UPDATE crypto_deposits
		SET 
			block_timestamp = $1,
			confirmations = $2,
			status = $3,
			transaction_id = $4,
			user_notified = $5,
			notified_at = $6,
			notification_sent = $7,
			confirmed_at = $8,
			credited_at = $9,
			updated_at = NOW()
		WHERE id = $10
	`

	result, err := r.pool.Exec(
		ctx, query,
		deposit.BlockTimestamp,
		deposit.Confirmations,
		deposit.Status,
		deposit.TransactionID,
		deposit.UserNotified,
		deposit.NotifiedAt,
		deposit.NotificationSent,
		deposit.ConfirmedAt,
		deposit.CreditedAt,
		deposit.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update deposit: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("deposit not found")
	}

	return nil
}

// ============================================================================
// STATUS UPDATE OPERATIONS
// ============================================================================

// UpdateStatus updates deposit status
func (r *CryptoDepositRepository) UpdateStatus(ctx context.Context, id int64, status domain.DepositStatus) error {
	query := `
		UPDATE crypto_deposits
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("deposit not found")
	}

	return nil
}

// UpdateConfirmations updates confirmation count
func (r *CryptoDepositRepository) UpdateConfirmations(ctx context.Context, id int64, confirmations int) error {
	query := `
		UPDATE crypto_deposits
		SET confirmations = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.pool.Exec(ctx, query, confirmations, id)
	if err != nil {
		return fmt.Errorf("failed to update confirmations:  %w", err)
	}

	return nil
}

// MarkAsConfirmed marks deposit as confirmed
func (r *CryptoDepositRepository) MarkAsConfirmed(ctx context.Context, id int64) error {
	query := `
		UPDATE crypto_deposits
		SET 
			status = $1,
			confirmed_at = NOW(),
			updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.pool.Exec(ctx, query, domain.DepositStatusConfirmed, id)
	if err != nil {
		return fmt.Errorf("failed to mark as confirmed:  %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("deposit not found")
	}

	return nil
}

// MarkAsCredited marks deposit as credited with transaction link
func (r *CryptoDepositRepository) MarkAsCredited(ctx context.Context, id int64, transactionID int64) error {
	query := `
		UPDATE crypto_deposits
		SET 
			status = $1,
			transaction_id = $2,
			credited_at = NOW(),
			updated_at = NOW()
		WHERE id = $3
	`

	result, err := r.pool.Exec(ctx, query, domain.DepositStatusCredited, transactionID, id)
	if err != nil {
		return fmt.Errorf("failed to mark as credited: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("deposit not found")
	}

	return nil
}

// MarkAsNotified marks deposit as user notified
func (r *CryptoDepositRepository) MarkAsNotified(ctx context.Context, id int64) error {
	query := `
		UPDATE crypto_deposits
		SET 
			user_notified = true,
			notified_at = NOW(),
			notification_sent = true,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark as notified: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("deposit not found")
	}

	return nil
}

// ============================================================================
// QUERY OPERATIONS
// ============================================================================

// GetPendingDeposits retrieves all pending deposits
func (r *CryptoDepositRepository) GetPendingDeposits(ctx context.Context) ([]*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE status IN ($1, $2)
		ORDER BY detected_at ASC
	`

	rows, err := r.pool.Query(ctx, query, domain.DepositStatusDetected, domain.DepositStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending deposits: %w", err)
	}
	defer rows.Close()

	var deposits []*domain.CryptoDeposit
	for rows.Next() {
		deposit := &domain.CryptoDeposit{}
		if err := r.scanDeposit(rows, deposit); err != nil {
			return nil, err
		}
		deposits = append(deposits, deposit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deposits: %w", err)
	}

	return deposits, nil
}

// GetUserDeposits retrieves user deposits with pagination
func (r *CryptoDepositRepository) GetUserDeposits(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE user_id = $1
		ORDER BY detected_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query user deposits: %w", err)
	}
	defer rows.Close()

	var deposits []*domain.CryptoDeposit
	for rows.Next() {
		deposit := &domain.CryptoDeposit{}
		if err := r.scanDeposit(rows, deposit); err != nil {
			return nil, err
		}
		deposits = append(deposits, deposit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deposits:  %w", err)
	}

	return deposits, nil
}

// GetWalletDeposits retrieves deposits for a specific wallet
func (r *CryptoDepositRepository) GetWalletDeposits(ctx context.Context, walletID int64) ([]*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE wallet_id = $1
		ORDER BY detected_at DESC
	`

	rows, err := r.pool.Query(ctx, query, walletID)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallet deposits: %w", err)
	}
	defer rows.Close()

	var deposits []*domain.CryptoDeposit
	for rows.Next() {
		deposit := &domain.CryptoDeposit{}
		if err := r.scanDeposit(rows, deposit); err != nil {
			return nil, err
		}
		deposits = append(deposits, deposit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deposits: %w", err)
	}

	return deposits, nil
}

// GetUnnotifiedDeposits retrieves deposits that haven't been notified
func (r *CryptoDepositRepository) GetUnnotifiedDeposits(ctx context.Context) ([]*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE user_notified = false 
		  AND status IN ($1, $2, $3)
		ORDER BY detected_at ASC
	`

	rows, err := r.pool.Query(
		ctx, query,
		domain.DepositStatusDetected,
		domain.DepositStatusPending,
		domain.DepositStatusConfirmed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query unnotified deposits: %w", err)
	}
	defer rows.Close()

	var deposits []*domain.CryptoDeposit
	for rows.Next() {
		deposit := &domain.CryptoDeposit{}
		if err := r.scanDeposit(rows, deposit); err != nil {
			return nil, err
		}
		deposits = append(deposits, deposit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deposits: %w", err)
	}

	return deposits, nil
}

// ============================================================================
// DUPLICATE CHECK
// ============================================================================

// DepositExists checks if a deposit already exists
func (r *CryptoDepositRepository) DepositExists(ctx context.Context, txHash, toAddress string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 
			FROM crypto_deposits 
			WHERE tx_hash = $1 AND to_address = $2
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, txHash, toAddress).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check deposit existence: %w", err)
	}

	return exists, nil
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

// GetDepositsByStatus retrieves deposits by status
func (r *CryptoDepositRepository) GetDepositsByStatus(
	ctx context.Context,
	status domain.DepositStatus,
	limit int,
) ([]*domain.CryptoDeposit, error) {
	query := `
		SELECT 
			id, deposit_id, wallet_id, user_id,
			chain, asset, from_address, to_address, amount,
			tx_hash, block_number, block_timestamp,
			confirmations, required_confirmations,
			status, transaction_id,
			user_notified, notified_at, notification_sent,
			detected_at, confirmed_at, credited_at,
			created_at, updated_at
		FROM crypto_deposits
		WHERE status = $1
		ORDER BY detected_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, status, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query deposits by status: %w", err)
	}
	defer rows.Close()

	var deposits []*domain.CryptoDeposit
	for rows.Next() {
		deposit := &domain.CryptoDeposit{}
		if err := r.scanDeposit(rows, deposit); err != nil {
			return nil, err
		}
		deposits = append(deposits, deposit)
	}

	return deposits, nil
}

// UpdateConfirmationsBatch updates confirmations for multiple deposits
func (r *CryptoDepositRepository) UpdateConfirmationsBatch(
	ctx context.Context,
	updates map[int64]int, // depositID -> confirmations
) error {
	// Start transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE crypto_deposits
		SET confirmations = $1, updated_at = NOW()
		WHERE id = $2
	`

	for depositID, confirmations := range updates {
		_, err := tx.Exec(ctx, query, confirmations, depositID)
		if err != nil {
			return fmt.Errorf("failed to update confirmations for deposit %d: %w", depositID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit batch update: %w", err)
	}

	return nil
}

// GetDepositNotifications retrieves deposits needing notification
func (r *CryptoDepositRepository) GetDepositNotifications(ctx context.Context, limit int) ([]*domain.DepositNotification, error) {
	query := `
		SELECT 
			user_id, deposit_id, chain, asset, amount,
			tx_hash, confirmations, required_confirmations,
			status, detected_at
		FROM crypto_deposits
		WHERE user_notified = false 
		  AND status IN ($1, $2, $3)
		ORDER BY detected_at ASC
		LIMIT $4
	`

	rows, err := r.pool.Query(
		ctx, query,
		domain.DepositStatusDetected,
		domain.DepositStatusPending,
		domain.DepositStatusConfirmed,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query deposit notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*domain.DepositNotification
	for rows.Next() {
		var amountStr string
		notif := &domain.DepositNotification{}

		err := rows.Scan(
			&notif.UserID,
			&notif.DepositID,
			&notif.Chain,
			&notif.Asset,
			&amountStr,
			&notif.TxHash,
			&notif.Confirmations,
			&notif.Required,
			&notif.Status,
			&notif.DetectedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}

		// Format amount (simplified - would use proper formatting)
		notif.Amount = amountStr

		notifications = append(notifications, notif)
	}

	return notifications, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// scanDeposit scans a row into CryptoDeposit
func (r *CryptoDepositRepository) scanDeposit(row pgx.Row, deposit *domain.CryptoDeposit) error {
	var amountStr string

	err := row.Scan(
		&deposit.ID,
		&deposit.DepositID,
		&deposit.WalletID,
		&deposit.UserID,
		&deposit.Chain,
		&deposit.Asset,
		&deposit.FromAddress,
		&deposit.ToAddress,
		&amountStr,
		&deposit.TxHash,
		&deposit.BlockNumber,
		&deposit.BlockTimestamp,
		&deposit.Confirmations,
		&deposit.RequiredConfirmations,
		&deposit.Status,
		&deposit.TransactionID,
		&deposit.UserNotified,
		&deposit.NotifiedAt,
		&deposit.NotificationSent,
		&deposit.DetectedAt,
		&deposit.ConfirmedAt,
		&deposit.CreditedAt,
		&deposit.CreatedAt,
		&deposit.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to scan deposit: %w", err)
	}

	// Parse amount
	deposit.Amount, _ = new(big.Int).SetString(amountStr, 10)
	if deposit.Amount == nil {
		deposit.Amount = big.NewInt(0)
	}

	return nil
}

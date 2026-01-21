// internal/repository/crypto_transaction_repo.go
package repository

import (
	"context"
	"crypto-service/internal/domain"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CryptoTransactionRepository struct {
	pool *pgxpool. Pool
}

func NewCryptoTransactionRepository(pool *pgxpool.Pool) *CryptoTransactionRepository {
	return &CryptoTransactionRepository{pool: pool}
}

// ============================================================================
// CORE CRUD OPERATIONS
// ============================================================================

// Create creates a new crypto transaction
func (r *CryptoTransactionRepository) Create(ctx context.Context, tx *domain.CryptoTransaction) error {
	query := `
		INSERT INTO crypto_transactions (
			transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21,
			$22, $23, $24, $25,
			$26, $27, $28, $29, $30,
			$31
		)
		RETURNING id, created_at, updated_at
	`

	// Generate UUID if not set
	if tx.TransactionID == "" {
		tx.TransactionID = uuid.New().String()
	}

	// Serialize metadata to JSON
	var metadataJSON []byte
	if tx.Metadata != nil {
		metadataJSON, _ = json.Marshal(tx.Metadata)
	}

	// Convert amounts to strings
	amountStr := "0"
	if tx.Amount != nil {
		amountStr = tx.Amount.String()
	}

	networkFeeStr := "0"
	if tx.NetworkFee != nil {
		networkFeeStr = tx.NetworkFee.String()
	}

	platformFeeStr := "0"
	if tx.PlatformFee != nil {
		platformFeeStr = tx.PlatformFee.String()
	}

	totalFeeStr := "0"
	if tx.TotalFee != nil {
		totalFeeStr = tx.TotalFee.String()
	}

	gasPriceStr := sql.NullString{}
	if tx.GasPrice != nil {
		gasPriceStr. String = tx.GasPrice.String()
		gasPriceStr.Valid = true
	}

	err := r.pool.QueryRow(
		ctx, query,
		tx.TransactionID,
		tx.UserID,
		tx.Type,
		tx.Chain,
		tx.Asset,
		tx.FromWalletID,
		tx.FromAddress,
		tx.ToWalletID,
		tx.ToAddress,
		tx.IsInternal,
		amountStr,
		networkFeeStr,
		tx.NetworkFeeCurrency,
		platformFeeStr,
		tx.PlatformFeeCurrency,
		totalFeeStr,
		tx.TxHash,
		tx.BlockNumber,
		tx.BlockTimestamp,
		tx.Confirmations,
		tx.RequiredConfirmations,
		tx.GasUsed,
		gasPriceStr,
		tx.EnergyUsed,
		tx.BandwidthUsed,
		tx.Status,
		tx.StatusMessage,
		tx.AccountingTxID,
		tx.Memo,
		metadataJSON,
		tx.InitiatedAt,
	).Scan(&tx.ID, &tx.CreatedAt, &tx. UpdatedAt)

	if err != nil {
		return fmt. Errorf("failed to create transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a transaction by ID
func (r *CryptoTransactionRepository) GetByID(ctx context.Context, id int64) (*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE id = $1
	`

	tx := &domain.CryptoTransaction{}
	err := r.scanTransaction(r.pool.QueryRow(ctx, query, id), tx)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return tx, nil
}

// GetByTransactionID retrieves a transaction by UUID
func (r *CryptoTransactionRepository) GetByTransactionID(ctx context.Context, txID string) (*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE transaction_id = $1
	`

	tx := &domain.CryptoTransaction{}
	err := r.scanTransaction(r.pool.QueryRow(ctx, query, txID), tx)

	if err == pgx. ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return tx, nil
}

// GetByTxHash retrieves a transaction by blockchain hash
func (r *CryptoTransactionRepository) GetByTxHash(ctx context.Context, txHash string) (*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE tx_hash = $1
	`

	tx := &domain.CryptoTransaction{}
	err := r.scanTransaction(r.pool.QueryRow(ctx, query, txHash), tx)

	if err == pgx. ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return tx, nil
}

// GetByAccountingTxID retrieves a transaction by accounting transaction ID
func (r *CryptoTransactionRepository) GetByAccountingTxID(ctx context.Context, accountingTxID string) (*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE accounting_tx_id = $1
	`

	tx := &domain.CryptoTransaction{}
	err := r.scanTransaction(r.pool. QueryRow(ctx, query, accountingTxID), tx)

	if err == pgx.ErrNoRows {
		return nil, nil // Return nil for idempotency check (not found is not an error)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction by accounting ID: %w", err)
	}

	return tx, nil
}

// Update updates a transaction
func (r *CryptoTransactionRepository) Update(ctx context.Context, tx *domain. CryptoTransaction) error {
	query := `
		UPDATE crypto_transactions
		SET 
			status = $1,
			status_message = $2,
			confirmations = $3,
			block_number = $4,
			block_timestamp = $5,
			tx_hash = $6,
			gas_used = $7,
			energy_used = $8,
			bandwidth_used = $9,
			broadcasted_at = $10,
			confirmed_at = $11,
			completed_at = $12,
			failed_at = $13,
			updated_at = NOW()
		WHERE id = $14
	`

	result, err := r.pool.Exec(
		ctx, query,
		tx.Status,
		tx.StatusMessage,
		tx.Confirmations,
		tx.BlockNumber,
		tx.BlockTimestamp,
		tx.TxHash,
		tx.GasUsed,
		tx.EnergyUsed,
		tx.BandwidthUsed,
		tx. BroadcastedAt,
		tx.ConfirmedAt,
		tx.CompletedAt,
		tx.FailedAt,
		tx.ID,
	)

	if err != nil {
		return fmt. Errorf("failed to update transaction: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}

	return nil
}

// ============================================================================
// STATUS UPDATE OPERATIONS
// ============================================================================

// UpdateStatus updates transaction status
func (r *CryptoTransactionRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status domain.TransactionStatus,
	message *string,
) error {
	query := `
		UPDATE crypto_transactions
		SET status = $1, status_message = $2, updated_at = NOW()
		WHERE id = $3
	`

	result, err := r. pool.Exec(ctx, query, status, message, id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt. Errorf("transaction not found")
	}

	return nil
}

// UpdateConfirmations updates confirmation count
func (r *CryptoTransactionRepository) UpdateConfirmations(ctx context.Context, id int64, confirmations int) error {
	query := `
		UPDATE crypto_transactions
		SET confirmations = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.pool.Exec(ctx, query, confirmations, id)
	if err != nil {
		return fmt.Errorf("failed to update confirmations:  %w", err)
	}

	return nil
}

// MarkAsBroadcasted marks transaction as broadcasted
func (r *CryptoTransactionRepository) MarkAsBroadcasted(ctx context.Context, id int64, txHash string) error {
	query := `
		UPDATE crypto_transactions
		SET 
			status = $1,
			tx_hash = $2,
			broadcasted_at = NOW(),
			updated_at = NOW()
		WHERE id = $3
	`

	result, err := r.pool. Exec(ctx, query, domain.TransactionStatusBroadcasted, txHash, id)
	if err != nil {
		return fmt.Errorf("failed to mark as broadcasted: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}

	return nil
}

// MarkAsConfirmed marks transaction as confirmed
func (r *CryptoTransactionRepository) MarkAsConfirmed(
	ctx context.Context,
	id int64,
	blockNumber int64,
	blockTime time.Time,
) error {
	query := `
		UPDATE crypto_transactions
		SET 
			status = $1,
			block_number = $2,
			block_timestamp = $3,
			confirmed_at = NOW(),
			updated_at = NOW()
		WHERE id = $4
	`

	result, err := r.pool.Exec(ctx, query, domain.TransactionStatusConfirmed, blockNumber, blockTime, id)
	if err != nil {
		return fmt.Errorf("failed to mark as confirmed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}

	return nil
}

// MarkAsFailed marks transaction as failed
func (r *CryptoTransactionRepository) MarkAsFailed(ctx context.Context, id int64, reason string) error {
	query := `
		UPDATE crypto_transactions
		SET 
			status = $1,
			status_message = $2,
			failed_at = NOW(),
			updated_at = NOW()
		WHERE id = $3
	`

	result, err := r. pool.Exec(ctx, query, domain.TransactionStatusFailed, reason, id)
	if err != nil {
		return fmt.Errorf("failed to mark as failed: %w", err)
	}

	if result. RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}

	return nil
}

// ============================================================================
// QUERY OPERATIONS
// ============================================================================

// GetUserTransactions retrieves user transactions with pagination
func (r *CryptoTransactionRepository) GetUserTransactions(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*domain.CryptoTransaction
	for rows.Next() {
		tx := &domain.CryptoTransaction{}
		if err := r.scanTransaction(rows, tx); err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	return transactions, nil
}

// GetPendingTransactions retrieves all pending transactions
func (r *CryptoTransactionRepository) GetPendingTransactions(ctx context.Context) ([]*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE status IN ($1, $2, $3, $4)
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(
		ctx, query,
		domain.TransactionStatusPending,
		domain.TransactionStatusBroadcasting,
		domain.TransactionStatusBroadcasted,
		domain.TransactionStatusConfirming,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*domain.CryptoTransaction
	for rows.Next() {
		tx := &domain.CryptoTransaction{}
		if err := r.scanTransaction(rows, tx); err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetTransactionsByStatus retrieves transactions by status
func (r *CryptoTransactionRepository) GetTransactionsByStatus(
	ctx context.Context,
	status domain.TransactionStatus,
) ([]*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := r. pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt. Errorf("failed to query transactions:  %w", err)
	}
	defer rows.Close()

	var transactions []*domain.CryptoTransaction
	for rows. Next() {
		tx := &domain.CryptoTransaction{}
		if err := r.scanTransaction(rows, tx); err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetRecentTransactions retrieves recent transaction summaries
func (r *CryptoTransactionRepository) GetRecentTransactions(ctx context.Context, limit int) ([]*domain.TransactionSummary, error) {
	query := `
		SELECT 
			transaction_id, type, chain, asset,
			amount, total_fee, status, tx_hash, is_internal,
			created_at, confirmed_at
		FROM crypto_transactions
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.pool. Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var summaries []*domain.TransactionSummary
	for rows.Next() {
		var amountStr, totalFeeStr string
		summary := &domain.TransactionSummary{}

		err := rows.Scan(
			&summary.ID,
			&summary.Type,
			&summary.Chain,
			&summary. Asset,
			&amountStr,
			&totalFeeStr,
			&summary.Status,
			&summary.TxHash,
			&summary. IsInternal,
			&summary.CreatedAt,
			&summary.ConfirmedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan summary: %w", err)
		}

		// Format amounts (simplified)
		summary.Amount = amountStr
		summary.Fee = totalFeeStr

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetInternalTransfers retrieves internal transfers for user
func (r *CryptoTransactionRepository) GetInternalTransfers(ctx context.Context, userID string) ([]*domain.CryptoTransaction, error) {
	query := `
		SELECT 
			id, transaction_id, user_id, type, chain, asset,
			from_wallet_id, from_address, to_wallet_id, to_address, is_internal,
			amount, network_fee, network_fee_currency, platform_fee, platform_fee_currency, total_fee,
			tx_hash, block_number, block_timestamp, confirmations, required_confirmations,
			gas_used, gas_price, energy_used, bandwidth_used,
			status, status_message, accounting_tx_id, memo, metadata,
			initiated_at, broadcasted_at, confirmed_at, completed_at, failed_at,
			created_at, updated_at
		FROM crypto_transactions
		WHERE user_id = $1 AND is_internal = true
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt. Errorf("failed to query internal transfers: %w", err)
	}
	defer rows.Close()

	var transactions []*domain.CryptoTransaction
	for rows.Next() {
		tx := &domain.CryptoTransaction{}
		if err := r.scanTransaction(rows, tx); err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// ============================================================================
// STATISTICS OPERATIONS
// ============================================================================

// GetTransactionCount gets total transaction count for user
func (r *CryptoTransactionRepository) GetTransactionCount(ctx context.Context, userID string) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM crypto_transactions
		WHERE user_id = $1
	`

	var count int64
	err := r.pool. QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return count, nil
}

// GetTransactionVolume gets transaction volume for period
func (r *CryptoTransactionRepository) GetTransactionVolume(
	ctx context.Context,
	userID, chain, asset string,
	from, to time.Time,
) (*big.Int, error) {
	query := `
		SELECT COALESCE(SUM(amount:: numeric), 0)
		FROM crypto_transactions
		WHERE user_id = $1 
		  AND chain = $2 
		  AND asset = $3
		  AND created_at BETWEEN $4 AND $5
		  AND status = $6
	`

	var volumeStr string
	err := r.pool.QueryRow(
		ctx, query,
		userID, chain, asset, from, to,
		domain.TransactionStatusConfirmed,
	).Scan(&volumeStr)

	if err != nil {
		return big.NewInt(0), fmt.Errorf("failed to get volume: %w", err)
	}

	volume, _ := new(big.Int).SetString(volumeStr, 10)
	if volume == nil {
		volume = big.NewInt(0)
	}

	return volume, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// scanTransaction scans a row into CryptoTransaction
func (r *CryptoTransactionRepository) scanTransaction(row pgx.Row, tx *domain. CryptoTransaction) error {
	var (
		amountStr, networkFeeStr, platformFeeStr, totalFeeStr, gasPriceStr string
		metadataJSON                                                       []byte
	)

	err := row.Scan(
		&tx.ID,
		&tx.TransactionID,
		&tx. UserID,
		&tx. Type,
		&tx.Chain,
		&tx.Asset,
		&tx.FromWalletID,
		&tx. FromAddress,
		&tx. ToWalletID,
		&tx.ToAddress,
		&tx.IsInternal,
		&amountStr,
		&networkFeeStr,
		&tx.NetworkFeeCurrency,
		&platformFeeStr,
		&tx.PlatformFeeCurrency,
		&totalFeeStr,
		&tx.TxHash,
		&tx.BlockNumber,
		&tx.BlockTimestamp,
		&tx. Confirmations,
		&tx. RequiredConfirmations,
		&tx.GasUsed,
		&gasPriceStr,
		&tx.EnergyUsed,
		&tx.BandwidthUsed,
		&tx.Status,
		&tx. StatusMessage,
		&tx. AccountingTxID,
		&tx.Memo,
		&metadataJSON,
		&tx.InitiatedAt,
		&tx.BroadcastedAt,
		&tx.ConfirmedAt,
		&tx.CompletedAt,
		&tx.FailedAt,
		&tx. CreatedAt,
		&tx.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to scan transaction: %w", err)
	}

	// Parse big. Int fields
	tx.Amount, _ = new(big.Int).SetString(amountStr, 10)
	tx.NetworkFee, _ = new(big.Int).SetString(networkFeeStr, 10)
	tx.PlatformFee, _ = new(big. Int).SetString(platformFeeStr, 10)
	tx.TotalFee, _ = new(big.Int).SetString(totalFeeStr, 10)

	if gasPriceStr != "" {
		tx.GasPrice, _ = new(big.Int).SetString(gasPriceStr, 10)
	}

	// Parse metadata JSON
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &tx.Metadata)
	}

	return nil
}
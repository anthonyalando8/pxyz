// internal/repository/crypto_wallet_repo.go
package repository

import (
	"context"
	"crypto-service/internal/domain"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CryptoWalletRepository struct {
	pool *pgxpool.Pool
}

func NewCryptoWalletRepository(pool *pgxpool.Pool) *CryptoWalletRepository {
	return &CryptoWalletRepository{pool: pool}
}

// ============================================================================
// CORE CRUD OPERATIONS
// ============================================================================

// Create creates a new crypto wallet
func (r *CryptoWalletRepository) Create(ctx context.Context, wallet *domain.CryptoWallet) error {
	query := `
		INSERT INTO crypto_wallets (
			user_id, chain, asset, address, public_key, 
			encrypted_private_key, encryption_version, label, 
			is_primary, is_active, balance
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`
	
	balanceStr := "0"
	if wallet. Balance != nil {
		balanceStr = wallet.Balance.String()
	}
	
	err := r.pool.QueryRow(
		ctx, query,
		wallet.UserID,
		wallet.Chain,
		wallet.Asset,
		wallet.Address,
		wallet.PublicKey,
		wallet. EncryptedPrivateKey,
		wallet.EncryptionVersion,
		wallet.Label,
		wallet.IsPrimary,
		wallet.IsActive,
		balanceStr,
	).Scan(&wallet.ID, &wallet.CreatedAt, &wallet.UpdatedAt)
	
	if err != nil {
		// Check for unique constraint violation
		if err. Error() == "duplicate key value violates unique constraint" {
			return fmt.Errorf("wallet address already exists")
		}
		return fmt.Errorf("failed to create wallet: %w", err)
	}
	
	return nil
}

// GetByID retrieves a wallet by ID
func (r *CryptoWalletRepository) GetByID(ctx context.Context, id int64) (*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE id = $1
	`
	
	wallet := &domain.CryptoWallet{}
	var balanceStr string
	
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.Chain,
		&wallet.Asset,
		&wallet.Address,
		&wallet.PublicKey,
		&wallet.EncryptedPrivateKey,
		&wallet. EncryptionVersion,
		&wallet.Label,
		&wallet.IsPrimary,
		&wallet.IsActive,
		&balanceStr,
		&wallet. LastBalanceUpdate,
		&wallet.LastDepositCheck,
		&wallet.LastTransactionBlock,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	
	if err == pgx.ErrNoRows {
		return nil, fmt. Errorf("wallet not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	
	// Parse balance
	wallet.Balance, _ = new(big.Int).SetString(balanceStr, 10)
	if wallet.Balance == nil {
		wallet.Balance = big.NewInt(0)
	}
	
	return wallet, nil
}

// GetByAddress retrieves a wallet by blockchain address
func (r *CryptoWalletRepository) GetByAddress(ctx context.Context, address string) (*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE address = $1
	`
	
	wallet := &domain.CryptoWallet{}
	var balanceStr string
	
	err := r.pool.QueryRow(ctx, query, address).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.Chain,
		&wallet. Asset,
		&wallet.Address,
		&wallet.PublicKey,
		&wallet. EncryptedPrivateKey,
		&wallet.EncryptionVersion,
		&wallet.Label,
		&wallet.IsPrimary,
		&wallet. IsActive,
		&balanceStr,
		&wallet.LastBalanceUpdate,
		&wallet.LastDepositCheck,
		&wallet.LastTransactionBlock,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("wallet not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	
	wallet.Balance, _ = new(big.Int).SetString(balanceStr, 10)
	if wallet.Balance == nil {
		wallet.Balance = big.NewInt(0)
	}
	
	return wallet, nil
}

// Update updates a wallet
func (r *CryptoWalletRepository) Update(ctx context.Context, wallet *domain.CryptoWallet) error {
	query := `
		UPDATE crypto_wallets
		SET 
			label = $1,
			is_primary = $2,
			is_active = $3,
			balance = $4,
			last_balance_update = $5,
			updated_at = NOW()
		WHERE id = $6
	`
	
	balanceStr := "0"
	if wallet.Balance != nil {
		balanceStr = wallet.Balance.String()
	}
	
	result, err := r.pool. Exec(
		ctx, query,
		wallet.Label,
		wallet.IsPrimary,
		wallet. IsActive,
		balanceStr,
		wallet.LastBalanceUpdate,
		wallet. ID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("wallet not found")
	}
	
	return nil
}

// Delete soft deletes a wallet
func (r *CryptoWalletRepository) Delete(ctx context.Context, id int64) error {
	query := `
		UPDATE crypto_wallets
		SET is_active = false, updated_at = NOW()
		WHERE id = $1
	`
	
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete wallet: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("wallet not found")
	}
	
	return nil
}

// ============================================================================
// USER WALLET OPERATIONS
// ============================================================================

// GetUserWallets retrieves all wallets for a user
func (r *CryptoWalletRepository) GetUserWallets(ctx context.Context, userID string) ([]*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE user_id = $1 AND is_active = true
		ORDER BY is_primary DESC, chain, asset
	`
	
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()
	
	var wallets []*domain.CryptoWallet
	for rows.Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating wallets: %w", err)
	}
	
	return wallets, nil
}

// GetUserWalletByChainAsset gets user's wallet for specific chain and asset
func (r *CryptoWalletRepository) GetUserWalletByChainAsset(
	ctx context.Context,
	userID, chain, asset string,
) (*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE user_id = $1 
		  AND chain = $2 
		  AND asset = $3
		  AND is_active = true
		ORDER BY is_primary DESC
		LIMIT 1
	`
	
	wallet := &domain.CryptoWallet{}
	var balanceStr string
	
	err := r.pool.QueryRow(ctx, query, userID, chain, asset).Scan(
		&wallet.ID,
		&wallet. UserID,
		&wallet. Chain,
		&wallet.Asset,
		&wallet.Address,
		&wallet.PublicKey,
		&wallet.EncryptedPrivateKey,
		&wallet.EncryptionVersion,
		&wallet.Label,
		&wallet.IsPrimary,
		&wallet.IsActive,
		&balanceStr,
		&wallet.LastBalanceUpdate,
		&wallet. LastDepositCheck,
		&wallet.LastTransactionBlock,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("wallet not found for user")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	
	wallet.Balance, _ = new(big.Int).SetString(balanceStr, 10)
	if wallet.Balance == nil {
		wallet.Balance = big.NewInt(0)
	}
	
	return wallet, nil
}

// GetUserPrimaryWallet gets user's primary wallet for chain/asset
func (r *CryptoWalletRepository) GetUserPrimaryWallet(
	ctx context.Context,
	userID, chain, asset string,
) (*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE user_id = $1 
		  AND chain = $2 
		  AND asset = $3 
		  AND is_primary = true
		  AND is_active = true
	`
	
	wallet := &domain.CryptoWallet{}
	var balanceStr string
	
	err := r.pool.QueryRow(ctx, query, userID, chain, asset).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.Chain,
		&wallet.Asset,
		&wallet.Address,
		&wallet.PublicKey,
		&wallet.EncryptedPrivateKey,
		&wallet.EncryptionVersion,
		&wallet.Label,
		&wallet.IsPrimary,
		&wallet.IsActive,
		&balanceStr,
		&wallet.LastBalanceUpdate,
		&wallet.LastDepositCheck,
		&wallet. LastTransactionBlock,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	
	if err == pgx.ErrNoRows {
		return nil, fmt. Errorf("primary wallet not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get primary wallet: %w", err)
	}
	
	wallet.Balance, _ = new(big.Int).SetString(balanceStr, 10)
	if wallet.Balance == nil {
		wallet.Balance = big.NewInt(0)
	}
	
	return wallet, nil
}

// ============================================================================
// BALANCE OPERATIONS
// ============================================================================

// UpdateBalance updates wallet balance
func (r *CryptoWalletRepository) UpdateBalance(ctx context.Context, walletID int64, balance *big.Int) error {
	query := `
		UPDATE crypto_wallets
		SET 
			balance = $1, 
			last_balance_update = $2, 
			updated_at = NOW()
		WHERE id = $3
	`
	
	result, err := r.pool. Exec(ctx, query, balance. String(), time.Now(), walletID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("wallet not found")
	}
	
	return nil
}

// GetWalletBalance retrieves formatted wallet balance
func (r *CryptoWalletRepository) GetWalletBalance(ctx context.Context, walletID int64) (*domain.WalletBalance, error) {
	query := `
		SELECT 
			id, address, chain, asset, balance, last_balance_update
		FROM crypto_wallets
		WHERE id = $1
	`
	
	var balanceStr string
	wb := &domain.WalletBalance{}
	
	err := r.pool.QueryRow(ctx, query, walletID).Scan(
		&wb.WalletID,
		&wb.Address,
		&wb. Chain,
		&wb.Asset,
		&balanceStr,
		&wb. UpdatedAt,
	)
	
	if err == pgx. ErrNoRows {
		return nil, fmt.Errorf("wallet not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}
	
	wb.Balance, _ = new(big.Int).SetString(balanceStr, 10)
	if wb.Balance == nil {
		wb.Balance = big.NewInt(0)
	}
	
	// Format balance (would use decimals from asset config)
	wb.Decimals = 6 // Default, should come from asset
	wb. BalanceFormatted = formatBalance(wb.Balance, wb.Decimals, wb.Asset)
	
	return wb, nil
}

// ============================================================================
// MONITORING OPERATIONS
// ============================================================================

// GetWalletsForDepositCheck gets wallets needing deposit monitoring
func (r *CryptoWalletRepository) GetWalletsForDepositCheck(ctx context.Context, limit int) ([]*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE is_active = true
		  AND (last_deposit_check IS NULL OR last_deposit_check < NOW() - INTERVAL '1 minute')
		ORDER BY last_deposit_check ASC NULLS FIRST
		LIMIT $1
	`
	
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets:  %w", err)
	}
	defer rows.Close()
	
	var wallets []*domain.CryptoWallet
	for rows.Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating wallets: %w", err)
	}
	
	return wallets, nil
}

// UpdateLastDepositCheck updates last deposit check timestamp
func (r *CryptoWalletRepository) UpdateLastDepositCheck(ctx context.Context, walletID int64) error {
	query := `
		UPDATE crypto_wallets
		SET last_deposit_check = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	
	_, err := r.pool.Exec(ctx, query, walletID)
	if err != nil {
		return fmt.Errorf("failed to update deposit check: %w", err)
	}
	
	return nil
}

// UpdateLastTransactionBlock updates last processed block number
func (r *CryptoWalletRepository) UpdateLastTransactionBlock(ctx context.Context, walletID int64, blockNumber int64) error {
	query := `
		UPDATE crypto_wallets
		SET last_transaction_block = $1, updated_at = NOW()
		WHERE id = $2
	`
	
	_, err := r. pool.Exec(ctx, query, blockNumber, walletID)
	if err != nil {
		return fmt.Errorf("failed to update last block: %w", err)
	}
	
	return nil
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

// GetWalletsByChain retrieves all wallets for a specific chain
func (r *CryptoWalletRepository) GetWalletsByChain(ctx context.Context, chain string) ([]*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE chain = $1 AND is_active = true
		ORDER BY created_at DESC
	`
	
	rows, err := r. pool.Query(ctx, query, chain)
	if err != nil {
		return nil, fmt. Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()
	
	var wallets []*domain.CryptoWallet
	for rows.Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	
	return wallets, nil
}

// GetActiveWallets retrieves all active wallets
func (r *CryptoWalletRepository) GetActiveWallets(ctx context.Context) ([]*domain.CryptoWallet, error) {
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE is_active = true
		ORDER BY created_at DESC
	`
	
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt. Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()
	
	var wallets []*domain.CryptoWallet
	for rows.Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	
	return wallets, nil
}

// internal/repository/crypto_wallet_repo.go

// Add this method to CryptoWalletRepository: 

// GetWalletsWithBalance retrieves wallets with balance above minimum
func (r *CryptoWalletRepository) GetWalletsWithBalance(
	ctx context.Context,
	chain, asset string,
	minBalance *big.Int,
) ([]*domain.CryptoWallet, error) {
	
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE chain = $1 
		  AND asset = $2
		  AND is_active = true
		  AND balance:: numeric >= $3
		  AND user_id != 'SYSTEM'  -- Exclude system wallets
		ORDER BY balance DESC
	`

	rows, err := r.pool.Query(ctx, query, chain, asset, minBalance. String())
	if err != nil {
		return nil, fmt. Errorf("failed to query wallets with balance: %w", err)
	}
	defer rows.  Close()

	var wallets []*domain.CryptoWallet
	for rows. Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan wallet: %w", err)
		}
		wallets = append(wallets, wallet)
	}

	if err := rows. Err(); err != nil {
		return nil, fmt.Errorf("error iterating wallets:  %w", err)
	}

	return wallets, nil
}

// GetWalletsByChainAsset retrieves all wallets for a chain/asset combination
func (r *CryptoWalletRepository) GetWalletsByChainAsset(
	ctx context.  Context,
	chain, asset string,
) ([]*domain.CryptoWallet, error) {
	
	query := `
		SELECT 
			id, user_id, chain, asset, address, public_key,
			encrypted_private_key, encryption_version, label,
			is_primary, is_active, balance, last_balance_update,
			last_deposit_check, last_transaction_block,
			created_at, updated_at
		FROM crypto_wallets
		WHERE chain = $1 
		  AND asset = $2
		  AND is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, chain, asset)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()

	var wallets []*domain.CryptoWallet
	for rows. Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}

	return wallets, nil
}

// CountWalletsWithBalance counts wallets with balance above minimum
func (r *CryptoWalletRepository) CountWalletsWithBalance(
	ctx context.Context,
	chain, asset string,
	minBalance *big.Int,
) (int64, error) {
	
	query := `
		SELECT COUNT(*)
		FROM crypto_wallets
		WHERE chain = $1 
		  AND asset = $2
		  AND is_active = true
		  AND balance::numeric >= $3
		  AND user_id != 'SYSTEM'
	`

	var count int64
	err := r.pool. QueryRow(ctx, query, chain, asset, minBalance.String()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count wallets:  %w", err)
	}

	return count, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================
// internal/repository/crypto_wallet_repo. go

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// Scanner interface for both Row and Rows
type Scanner interface {
	Scan(dest ...interface{}) error
}

// scanWallet scans using the Scanner interface (works for both Row and Rows)
func scanWallet(scanner Scanner) (*domain.CryptoWallet, error) {
	wallet := &domain.CryptoWallet{}
	var balanceStr string
	var lastBalanceUpdate, lastDepositCheck sql.NullTime
	var lastTransactionBlock sql.NullInt64
	
	err := scanner. Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.  Chain,
		&wallet.Asset,
		&wallet.Address,
		&wallet.PublicKey,
		&wallet.EncryptedPrivateKey,
		&wallet.EncryptionVersion,
		&wallet.Label,
		&wallet.IsPrimary,
		&wallet.IsActive,
		&balanceStr,
		&lastBalanceUpdate,
		&lastDepositCheck,
		&lastTransactionBlock,
		&wallet. CreatedAt,
		&wallet.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt. Errorf("failed to scan wallet: %w", err)
	}
	
	// Parse balance
	wallet.Balance, _ = new(big.Int).SetString(balanceStr, 10)
	if wallet.Balance == nil {
		wallet.Balance = big.NewInt(0)
	}
	
	// Handle nullable fields
	if lastBalanceUpdate. Valid {
		wallet.LastBalanceUpdate = &lastBalanceUpdate.Time
	}
	
	if lastDepositCheck.Valid {
		wallet.LastDepositCheck = &lastDepositCheck. Time
	}
	
	if lastTransactionBlock.Valid {
		wallet.LastTransactionBlock = &lastTransactionBlock.Int64
	}
	
	return wallet, nil
}

// formatBalance formats big.Int balance to human-readable string
func formatBalance(balance *big.Int, decimals int, asset string) string {
	if balance == nil {
		return "0 " + asset
	}
	
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	wholePart := new(big.  Int).Div(balance, divisor)
	remainder := new(big.Int).Mod(balance, divisor)
	
	if remainder.Cmp(big.NewInt(0)) == 0 {
		return fmt. Sprintf("%s %s", wholePart.String(), asset)
	}
	
	// Format with decimals
	decimalPart := new(big.Float).Quo(
		new(big.Float).SetInt(remainder),
		new(big.Float).SetInt(divisor),
	)
	
	formatted := fmt.Sprintf("%s%s", wholePart.String(), decimalPart.Text('f', decimals)[1:])
	// Trim trailing zeros
	formatted = strings.TrimRight(strings. TrimRight(formatted, "0"), ".")
	
	return fmt.Sprintf("%s %s", formatted, asset)
}
package repository

import (
	"accounting-service/internal/domain"
	"context"
	"errors"
	"fmt"
	"time"

	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BalanceRepository interface {
	// Basic CRUD
	GetByAccountID(ctx context.Context, accountID int64) (*domain.Balance, error)
	GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Balance, error)
	GetByAccountIDWithLock(ctx context.Context, tx pgx.Tx, accountID int64) (*domain.Balance, error)
	GetMultipleByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*domain.Balance, error)

	// Balance updates
	UpdateBalance(ctx context.Context, tx pgx.Tx, update *domain.BalanceUpdate) error
	UpdateBalanceBatch(ctx context.Context, tx pgx.Tx, updates []*domain.BalanceUpdate) error

	// Optimistic locking
	UpdateBalanceOptimistic(ctx context.Context, tx pgx.Tx, update *domain.BalanceUpdate, expectedVersion int64) error

	// Pending balance operations
	ReserveFunds(ctx context.Context, tx pgx.Tx, accountID int64, amount float64) error
	ReleaseFunds(ctx context.Context, tx pgx.Tx, accountID int64, amount float64, complete bool) error

	// Utility methods
	GetCachedBalance(ctx context.Context, accountNumber string) (*domain.Balance, error)
	EnsureBalanceExists(ctx context.Context, tx pgx.Tx, accountID int64) error
}

type balanceRepo struct {
	db *pgxpool.Pool
}

func NewBalanceRepo(db *pgxpool.Pool) BalanceRepository {
	return &balanceRepo{db: db}
}

// GetByAccountNumber retrieves balance by account number (optimized single query)
func (r *balanceRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Balance, error) {
	query := `
		SELECT 
			b.account_id,
			b.balance,
			b.available_balance,
			b.pending_debit,
			b.pending_credit,
			b.last_ledger_id,
			b.version,
			b.updated_at
		FROM balances b
		INNER JOIN accounts a ON a.id = b.account_id
		WHERE a.account_number = $1 
		AND a.is_active = true
	`

	var balance domain.Balance
	err := r.db.QueryRow(ctx, query).Scan(
		&balance.AccountID,
		&balance.Balance,
		&balance.AvailableBalance,
		&balance.PendingDebit,
		&balance.PendingCredit,
		&balance.LastLedgerID,
		&balance.Version,
		&balance.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("balance not found for account: %s", accountNumber)
		}
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return &balance, nil
}

// GetByAccountNumber retrieves balance by account number
// func (r *balanceRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Balance, error) {
// 	// First, get account ID from account number
// 	query := `
// 		SELECT a.id
// 		FROM accounts a
// 		WHERE a.account_number = $1
// 		AND a.is_active = true
// 	`

// 	var accountID int64
// 	err := r.db.QueryRow(ctx, query, accountNumber).Scan(&accountID)
// 	if err != nil {
// 		if err == pgx.ErrNoRows {
// 			return nil, fmt.Errorf("account not found: %s", accountNumber)
// 		}
// 		return nil, fmt.Errorf("failed to get account: %w", err)
// 	}

// 	// Now get balance using existing method
// 	return r.GetByAccountID(ctx, accountID)
// }

// GetByAccountID fetches the balance for a specific account (read-only, no lock)
func (r *balanceRepo) GetByAccountID(ctx context.Context, accountID int64) (*domain.Balance, error) {
	query := `
		SELECT 
			account_id, 
			balance, 
			available_balance, 
			pending_debit, 
			pending_credit, 
			last_ledger_id, 
			version, 
			updated_at
		FROM balances
		WHERE account_id = $1
	`

	var b domain.Balance
	err := r.db.QueryRow(ctx, query, accountID).Scan(
		&b.AccountID,
		&b.Balance,
		&b.AvailableBalance,
		&b.PendingDebit,
		&b.PendingCredit,
		&b.LastLedgerID,
		&b.Version,
		&b.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return &b, nil
}

// GetByAccountIDWithLock fetches balance with pessimistic lock (SELECT FOR UPDATE)
// Use this when you need to prevent concurrent modifications
func (r *balanceRepo) GetByAccountIDWithLock(ctx context.Context, tx pgx.Tx, accountID int64) (*domain.Balance, error) {
	if tx == nil {
		return nil, errors.New("transaction cannot be nil for locked query")
	}

	query := `
		SELECT 
			account_id, 
			balance, 
			available_balance, 
			pending_debit, 
			pending_credit, 
			last_ledger_id, 
			version, 
			updated_at
		FROM balances
		WHERE account_id = $1
		FOR UPDATE
	`

	var b domain.Balance
	err := tx.QueryRow(ctx, query, accountID).Scan(
		&b.AccountID,
		&b.Balance,
		&b.AvailableBalance,
		&b.PendingDebit,
		&b.PendingCredit,
		&b.LastLedgerID,
		&b.Version,
		&b.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get balance with lock: %w", err)
	}

	return &b, nil
}

// GetMultipleByAccountIDs fetches multiple balances in a single query (bulk read)
func (r *balanceRepo) GetMultipleByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*domain.Balance, error) {
	if len(accountIDs) == 0 {
		return make(map[int64]*domain.Balance), nil
	}

	query := `
		SELECT 
			account_id, 
			balance, 
			available_balance, 
			pending_debit, 
			pending_credit, 
			last_ledger_id, 
			version, 
			updated_at
		FROM balances
		WHERE account_id = ANY($1)
	`

	rows, err := r.db.Query(ctx, query, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query multiple balances: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]*domain.Balance, len(accountIDs))
	for rows.Next() {
		var b domain.Balance
		err := rows.Scan(
			&b.AccountID,
			&b.Balance,
			&b.AvailableBalance,
			&b.PendingDebit,
			&b.PendingCredit,
			&b.LastLedgerID,
			&b.Version,
			&b.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan balance row: %w", err)
		}
		result[b.AccountID] = &b
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating balance rows: %w", err)
	}

	return result, nil
}

// UpdateBalance updates balance within a transaction (pessimistic locking - uses FOR UPDATE internally)
func (r *balanceRepo) UpdateBalance(ctx context.Context, tx pgx.Tx, update *domain.BalanceUpdate) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	if update.DrCr != "DR" && update.DrCr != "CR" {
		return errors.New("invalid DR/CR value")
	}

	// Fetch current balance with lock
	balance, err := r.GetByAccountIDWithLock(ctx, tx, update.AccountID)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			// Balance doesn't exist, create it
			return r.createInitialBalance(ctx, tx, update)
		}
		return fmt.Errorf("failed to fetch balance: %w", err)
	}

	// Calculate new balance
	newBalance := balance.Balance
	newAvailable := balance.AvailableBalance

	if update.DrCr == "CR" {
		newBalance += update.Amount
		newAvailable += update.Amount
	} else { // DR
		newBalance -= update.Amount
		newAvailable -= update.Amount
	}

	// Check for negative available balance (overdraft protection)
	if newAvailable < 0 {
		return xerrors.ErrInsufficientFunds
	}

	// Update balance
	query := `
		UPDATE balances
		SET 
			balance = $1,
			available_balance = $2,
			last_ledger_id = $3,
			version = version + 1,
			updated_at = $4
		WHERE account_id = $5
	`

	cmdTag, err := tx.Exec(ctx, query,
		newBalance,
		newAvailable,
		update.LedgerID,
		time.Now(),
		update.AccountID,
	)

	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// UpdateBalanceBatch updates multiple balances in a single transaction (bulk update)
// More efficient for high-throughput scenarios
func (r *balanceRepo) UpdateBalanceBatch(ctx context.Context, tx pgx.Tx, updates []*domain.BalanceUpdate) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	if len(updates) == 0 {
		return nil
	}

	// Extract account IDs for bulk lock
	accountIDs := make([]int64, len(updates))
	updateMap := make(map[int64]*domain.BalanceUpdate, len(updates))

	for i, update := range updates {
		accountIDs[i] = update.AccountID
		updateMap[update.AccountID] = update
	}

	// Lock all accounts at once
	query := `
		SELECT 
			account_id, 
			balance, 
			available_balance, 
			pending_debit, 
			pending_credit, 
			last_ledger_id, 
			version, 
			updated_at
		FROM balances
		WHERE account_id = ANY($1)
		FOR UPDATE
	`

	rows, err := tx.Query(ctx, query, accountIDs)
	if err != nil {
		return fmt.Errorf("failed to lock balances: %w", err)
	}
	defer rows.Close()

	balances := make(map[int64]*domain.Balance)
	for rows.Next() {
		var b domain.Balance
		err := rows.Scan(
			&b.AccountID,
			&b.Balance,
			&b.AvailableBalance,
			&b.PendingDebit,
			&b.PendingCredit,
			&b.LastLedgerID,
			&b.Version,
			&b.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to scan balance: %w", err)
		}
		balances[b.AccountID] = &b
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating balance rows: %w", err)
	}

	// Build batch update using pgx.Batch for maximum performance
	batch := &pgx.Batch{}

	updateSQL := `
		UPDATE balances
		SET 
			balance = $1,
			available_balance = $2,
			last_ledger_id = $3,
			version = version + 1,
			updated_at = $4
		WHERE account_id = $5
	`

	now := time.Now()
	for _, update := range updates {
		balance, exists := balances[update.AccountID]
		if !exists {
			// Handle missing balance - create it
			if err := r.createInitialBalance(ctx, tx, update); err != nil {
				return fmt.Errorf("failed to create initial balance for account %d: %w", update.AccountID, err)
			}
			continue
		}

		// Calculate new values
		newBalance := balance.Balance
		newAvailable := balance.AvailableBalance

		if update.DrCr == "CR" {
			newBalance += update.Amount
			newAvailable += update.Amount
		} else {
			newBalance -= update.Amount
			newAvailable -= update.Amount
		}

		// Overdraft check
		if newAvailable < 0 {
			return fmt.Errorf("insufficient funds for account %d: %w", update.AccountID, xerrors.ErrInsufficientFunds)
		}

		batch.Queue(updateSQL, newBalance, newAvailable, update.LedgerID, now, update.AccountID)
	}

	// Execute batch
	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("failed to execute batch update at index %d: %w", i, err)
		}
	}

	return nil
}

// UpdateBalanceOptimistic updates balance using optimistic locking (version checking)
// Returns xerrors.ErrVersionMismatch if concurrent modification detected
func (r *balanceRepo) UpdateBalanceOptimistic(ctx context.Context, tx pgx.Tx, update *domain.BalanceUpdate, expectedVersion int64) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	if update.DrCr != "DR" && update.DrCr != "CR" {
		return errors.New("invalid DR/CR value")
	}

	query := `
		UPDATE balances
		SET 
			balance = balance + $1,
			available_balance = available_balance + $2,
			last_ledger_id = $3,
			version = version + 1,
			updated_at = $4
		WHERE account_id = $5 AND version = $6
		RETURNING balance, available_balance, version
	`

	delta := update.Amount
	if update.DrCr == "DR" {
		delta = -delta
	}

	var newBalance, newAvailable float64  // ✅ Changed to float64
	var newVersion int64                   // ✅ Version stays int64
	err := tx.QueryRow(ctx, query,
		delta,
		delta,
		update.LedgerID,
		time.Now(),
		update.AccountID,
		expectedVersion,
	). Scan(&newBalance, &newAvailable, &newVersion) 

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return xerrors.ErrVersionMismatch
		}
		return fmt.Errorf("failed to update balance optimistically: %w", err)
	}

	// Check for overdraft
	if newAvailable < 0 {
		return xerrors.ErrInsufficientFunds
	}

	return nil
}

// ReserveFunds reserves funds for pending transactions (locks available balance)
func (r *balanceRepo) ReserveFunds(ctx context.Context, tx pgx.Tx, accountID int64, amount float64) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	query := `
		UPDATE balances
		SET 
			available_balance = available_balance - $1,
			pending_debit = pending_debit + $1,
			version = version + 1,
			updated_at = $2
		WHERE account_id = $3
		RETURNING available_balance
	`

	var newAvailable float64
	err := tx.QueryRow(ctx, query, amount, time.Now(), accountID).Scan(&newAvailable)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return xerrors.ErrNotFound
		}
		return fmt.Errorf("failed to reserve funds: %w", err)
	}

	if newAvailable < 0 {
		return xerrors.ErrInsufficientFunds
	}

	return nil
}

// ReleaseFunds releases reserved funds (complete=true: deduct from balance, complete=false: return to available)
func (r *balanceRepo) ReleaseFunds(ctx context.Context, tx pgx.Tx, accountID int64, amount float64, complete bool) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	var query string
	if complete {
		// Transaction completed - deduct from total balance and remove from pending
		query = `
			UPDATE balances
			SET 
				balance = balance - $1,
				pending_debit = pending_debit - $1,
				version = version + 1,
				updated_at = $2
			WHERE account_id = $3
		`
	} else {
		// Transaction cancelled - return to available balance
		query = `
			UPDATE balances
			SET 
				available_balance = available_balance + $1,
				pending_debit = pending_debit - $1,
				version = version + 1,
				updated_at = $2
			WHERE account_id = $3
		`
	}

	cmdTag, err := tx.Exec(ctx, query, amount, time.Now(), accountID)
	if err != nil {
		return fmt.Errorf("failed to release funds: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// GetCachedBalance fetches balance by account number
func (r *balanceRepo) GetCachedBalance(ctx context.Context, accountNumber string) (*domain.Balance, error) {
	query := `
		SELECT 
			b.account_id, 
			b.balance, 
			b.available_balance, 
			b.pending_debit, 
			b.pending_credit, 
			b.last_ledger_id, 
			b.version, 
			b.updated_at
		FROM balances b
		JOIN accounts a ON a.id = b.account_id
		WHERE a.account_number = $1
	`

	var b domain.Balance
	err := r.db.QueryRow(ctx, query, accountNumber).Scan(
		&b.AccountID,
		&b.Balance,
		&b.AvailableBalance,
		&b.PendingDebit,
		&b.PendingCredit,
		&b.LastLedgerID,
		&b.Version,
		&b.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get cached balance: %w", err)
	}

	return &b, nil
}

// EnsureBalanceExists creates a balance record if it doesn't exist (idempotent)
func (r *balanceRepo) EnsureBalanceExists(ctx context.Context, tx pgx.Tx, accountID int64) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	query := `
		INSERT INTO balances (account_id, balance, available_balance, pending_debit, pending_credit, version, updated_at)
		VALUES ($1, 0, 0, 0, 0, 0, $2)
		ON CONFLICT (account_id) DO NOTHING
	`

	_, err := tx.Exec(ctx, query, accountID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to ensure balance exists: %w", err)
	}

	return nil
}

// createInitialBalance creates a new balance record with initial transaction
func (r *balanceRepo) createInitialBalance(ctx context.Context, tx pgx.Tx, update *domain.BalanceUpdate) error {
	initialBalance := float64(0)
	initialAvailable := float64(0)

	if update.DrCr == "CR" {
		initialBalance = update.Amount
		initialAvailable = update.Amount
	} else {
		return xerrors.ErrInsufficientFunds // Cannot debit from non-existent balance
	}

	query := `
		INSERT INTO balances (
			account_id, 
			balance, 
			available_balance, 
			pending_debit, 
			pending_credit, 
			last_ledger_id, 
			version, 
			updated_at
		)
		VALUES ($1, $2, $3, 0, 0, $4, 0, $5)
		ON CONFLICT (account_id) DO NOTHING
	`

	_, err := tx.Exec(ctx, query,
		update.AccountID,
		initialBalance,
		initialAvailable,
		update.LedgerID,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to create initial balance: %w", err)
	}

	return nil
}

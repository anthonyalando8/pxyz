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

// AccountRepository defines the interface for account persistence operations
// Optimized for high-throughput scenarios (4000+ req/s)
type AccountRepository interface {
	// Single account queries (optimized with proper indexes)
	GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error)
	GetByAccountNumberTx(ctx context.Context, accountNumber string, tx pgx.Tx) (*domain.Account, error)
	GetByID(ctx context.Context, accountID int64) (*domain.Account, error)
	GetByIDTx(ctx context.Context, accountID int64, tx pgx.Tx) (*domain.Account, error)

	// Owner-based queries (most common in transaction processing)
	GetByOwner(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType) ([]*domain.Account, error)
	GetByOwnerTx(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, tx pgx.Tx) ([]*domain.Account, error)

	// User account operations (lazy creation pattern)
	GetOrCreateUserAccounts(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, tx pgx.Tx) ([]*domain.Account, error)

	// Batch operations for high throughput
	Create(ctx context.Context, account *domain.Account, tx pgx.Tx) error
	CreateMany(ctx context.Context, accounts []*domain.Account, tx pgx.Tx) map[int]error
	GetByFilter(ctx context.Context, f *domain.AccountFilter) ([]*domain.Account, error)

	// Update operations
	Update(ctx context.Context, a *domain.Account, tx pgx.Tx) error
	UpdateMany(ctx context.Context, accounts []*domain.Account, tx pgx.Tx) map[int]error

	// Agent relationship queries
	GetByParentAgent(ctx context.Context, agentExternalID string) ([]*domain.Account, error)
	GetByParentAgentTx(ctx context.Context, agentExternalID string, tx pgx.Tx) ([]*domain.Account, error)

	// Status operations
	Lock(ctx context.Context, accountID int64, tx pgx.Tx) error
	Unlock(ctx context.Context, accountID int64, tx pgx.Tx) error

	GetSystemAccount(ctx context.Context, currency string, accountType domain.AccountType) (*domain.Account, error)
	GetSystemFeeAccount(ctx context.Context, currency string) (*domain.Account, error)
	GetAgentAccount(ctx context.Context, agentExternalID string, currency string) (*domain.Account, error)
	GetOrCreateAgentAccount(ctx context.Context, tx pgx.Tx, agentExternalID string, currency string, commissionRate *string) (*domain.Account, error)

	// Transaction helper
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type accountRepo struct {
	db *pgxpool.Pool
}

// NewAccountRepo creates a new account repository
func NewAccountRepo(db *pgxpool.Pool) AccountRepository {
	return &accountRepo{db: db}
}

func (r *accountRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted, // Good balance between performance and consistency
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// Query helpers to reduce duplication
const (
	baseSelectQuery = `
		SELECT id, account_number, owner_type, owner_id, currency, purpose, 
		       account_type, is_active, is_locked, overdraft_limit, 
		       parent_agent_external_id, commission_rate, created_at, updated_at
		FROM accounts`
)

// scanAccount scans a row into a domain.Account
func scanAccount(row pgx.Row) (*domain.Account, error) {
	var a domain.Account
	var parentAgentID *string
	var commissionRate *string

	err := row.Scan(
		&a.ID, &a.AccountNumber, &a.OwnerType, &a.OwnerID, &a.Currency,
		&a.Purpose, &a.AccountType, &a.IsActive, &a.IsLocked, &a.OverdraftLimit,
		&parentAgentID, &commissionRate, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan account: %w", err)
	}

	a.ParentAgentExternalID = parentAgentID
	a.CommissionRate = commissionRate

	return &a, nil
}

// scanAccountRows scans multiple rows into domain.Account slice
func scanAccountRows(rows pgx.Rows) ([]*domain.Account, error) {
	defer rows.Close()
	var accounts []*domain.Account

	for rows.Next() {
		var a domain.Account
		var parentAgentID *string
		var commissionRate *string

		err := rows.Scan(
			&a.ID, &a.AccountNumber, &a.OwnerType, &a.OwnerID, &a.Currency,
			&a.Purpose, &a.AccountType, &a.IsActive, &a.IsLocked, &a.OverdraftLimit,
			&parentAgentID, &commissionRate, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account row: %w", err)
		}

		a.ParentAgentExternalID = parentAgentID
		a.CommissionRate = commissionRate
		accounts = append(accounts, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return accounts, nil
}

// GetByAccountNumber fetches an account by account number (unique index)
// OPTIMIZED: Uses indexed unique account_number column for O(1) lookup
func (r *accountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	row := r.db.QueryRow(ctx, baseSelectQuery+` WHERE account_number=$1`, accountNumber)
	return scanAccount(row)
}

// GetByAccountNumberTx same as GetByAccountNumber but within a transaction
func (r *accountRepo) GetByAccountNumberTx(ctx context.Context, accountNumber string, tx pgx.Tx) (*domain.Account, error) {
	row := tx.QueryRow(ctx, baseSelectQuery+` WHERE account_number=$1`, accountNumber)
	return scanAccount(row)
}

// GetByID fetches an account by ID (primary key)
// OPTIMIZED: Direct primary key lookup
func (r *accountRepo) GetByID(ctx context.Context, accountID int64) (*domain.Account, error) {
	row := r.db.QueryRow(ctx, baseSelectQuery+` WHERE id=$1`, accountID)
	return scanAccount(row)
}

// GetByIDTx same as GetByID but within a transaction
func (r *accountRepo) GetByIDTx(ctx context.Context, accountID int64, tx pgx.Tx) (*domain.Account, error) {
	row := tx.QueryRow(ctx, baseSelectQuery+` WHERE id=$1`, accountID)
	return scanAccount(row)
}

// GetByOwner fetches all accounts for a given owner (most common pattern in transaction processing)
// OPTIMIZED: Uses idx_accounts_real/demo indexes for fast owner lookups
func (r *accountRepo) GetByOwner(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType) ([]*domain.Account, error) {
	rows, err := r.db.Query(ctx,
		baseSelectQuery+` WHERE owner_type=$1 AND owner_id=$2 AND account_type=$3`,
		ownerType, ownerID, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts by owner: %w", err)
	}

	accounts, err := scanAccountRows(rows)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return accounts, nil
}

// GetByOwnerTx same as GetByOwner but within a transaction
func (r *accountRepo) GetByOwnerTx(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, tx pgx.Tx) ([]*domain.Account, error) {
	rows, err := tx.Query(ctx,
		baseSelectQuery+` WHERE owner_type=$1 AND owner_id=$2 AND account_type=$3`,
		ownerType, ownerID, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts by owner in tx: %w", err)
	}

	accounts, err := scanAccountRows(rows)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return accounts, nil
}

// GetOrCreateUserAccounts implements lazy account creation pattern
// If user has no accounts, creates demo accounts for all demo-enabled currencies
// CRITICAL: Must be called within a transaction for atomicity
func (r *accountRepo) GetOrCreateUserAccounts(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
	tx pgx.Tx,
) ([]*domain.Account, error) {
	if tx == nil {
		return nil, errors.New("transaction cannot be nil for GetOrCreateUserAccounts")
	}

	// Try to get existing accounts
	accounts, err := r.GetByOwnerTx(ctx, ownerType, ownerID, accountType, tx)
	if err == nil && len(accounts) > 0 {
		return accounts, nil
	}

	// If error is not "not found", return it
	if err != nil && !errors.Is(err, xerrors.ErrNotFound) {
		return nil, err
	}

	// No accounts exist → create accounts based on account type
	now := time.Now()
	var accountsToCreate []*domain.Account

	if accountType == domain.AccountTypeDemo {
		// For demo accounts, get all demo-enabled currencies
		var demoCurrencies []struct {
			Code               string
			DemoInitialBalance float64
		}

		rows, err := tx.Query(ctx, `
			SELECT code, demo_initial_balance
			FROM currencies
			WHERE demo_enabled = true AND is_active = true
		`)
		if err != nil {
			return nil, fmt.Errorf("failed to query demo currencies: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var curr struct {
				Code               string
				DemoInitialBalance float64
			}
			if err := rows.Scan(&curr.Code, &curr.DemoInitialBalance); err != nil {
				return nil, fmt.Errorf("failed to scan currency: %w", err)
			}
			demoCurrencies = append(demoCurrencies, curr)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating currencies: %w", err)
		}

		// Create demo wallet account for each demo-enabled currency
		for _, curr := range demoCurrencies {
			accountsToCreate = append(accountsToCreate, &domain.Account{
				AccountNumber:  fmt.Sprintf("DEMO-%s-%s-%d", ownerID, curr.Code, now.UnixNano()),
				OwnerType:      ownerType,
				OwnerID:        ownerID,
				Currency:       curr.Code,
				Purpose:        domain.PurposeWallet,
				AccountType:    domain.AccountTypeDemo,
				IsActive:       true,
				IsLocked:       false,
				OverdraftLimit: 0,
				CreatedAt:      now,
				UpdatedAt:      now,
			})
		}
	} else {
		// For real accounts, create default USD wallet
		accountsToCreate = append(accountsToCreate, &domain.Account{
			AccountNumber:  fmt.Sprintf("ACC-%s-USD-%d", ownerID, now.UnixNano()),
			OwnerType:      ownerType,
			OwnerID:        ownerID,
			Currency:       "USD",
			Purpose:        domain.PurposeWallet,
			AccountType:    domain.AccountTypeReal,
			IsActive:       true,
			IsLocked:       false,
			OverdraftLimit: 0,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}

	// Create accounts
	errs := r.CreateMany(ctx, accountsToCreate, tx)
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to create accounts: %v", errs)
	}

	// If demo accounts, initialize with demo balance
	if accountType == domain.AccountTypeDemo {
		for _, acc := range accountsToCreate {
			// Get demo initial balance for this currency
			var demoBalance float64
			err := tx.QueryRow(ctx, `
				SELECT demo_initial_balance
				FROM currencies
				WHERE code = $1
			`, acc.Currency).Scan(&demoBalance)

			if err != nil {
				return nil, fmt.Errorf("failed to get demo balance: %w", err)
			}

			// Update balance to demo initial amount
			_, err = tx.Exec(ctx, `
				UPDATE balances
				SET balance = $1, available_balance = $1, updated_at = $2
				WHERE account_id = $3
			`, demoBalance, now, acc.ID)

			if err != nil {
				return nil, fmt.Errorf("failed to set demo balance: %w", err)
			}
		}
	}

	return accountsToCreate, nil
}

// Create inserts a single account within a transaction
func (r *accountRepo) Create(ctx context.Context, account *domain.Account, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	// Validate
	if !account.IsValid() {
		return errors.New("invalid account data")
	}

	now := time.Now()
	if account.AccountNumber == "" {
		account.AccountNumber = fmt.Sprintf("ACC-%d", time.Now().UnixNano())
	}

	// Insert account
	err := tx.QueryRow(ctx, `
		INSERT INTO accounts (
			owner_type, owner_id, currency, purpose, account_type,
			is_active, is_locked, overdraft_limit,
			parent_agent_external_id, commission_rate,
			account_number, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`,
		account.OwnerType,
		account.OwnerID,
		account.Currency,
		account.Purpose,
		account.AccountType,
		account.IsActive,
		account.IsLocked,
		account.OverdraftLimit,
		account.ParentAgentExternalID,
		account.CommissionRate,
		account.AccountNumber,
		now,
		now,
	).Scan(&account.ID)

	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}

	// Initialize balance
	_, err = tx.Exec(ctx, `
		INSERT INTO balances (account_id, balance, available_balance, pending_debit, pending_credit, version, updated_at)
		VALUES ($1, 0, 0, 0, 0, 0, $2)
		ON CONFLICT (account_id) DO NOTHING
	`, account.ID, now)

	if err != nil {
		return fmt.Errorf("failed to insert balance: %w", err)
	}

	return nil
}

// CreateMany inserts multiple accounts within a transaction
// Continues on error for individual accounts and returns error map by index
// OPTIMIZED: Uses ON CONFLICT for idempotency, batch balance inserts
func (r *accountRepo) CreateMany(
	ctx context.Context,
	accounts []*domain.Account,
	tx pgx.Tx,
) map[int]error {
	if tx == nil {
		return map[int]error{0: errors.New("transaction cannot be nil")}
	}

	errs := make(map[int]error)
	now := time.Now()

	for i, a := range accounts {
		// Validate
		if !a.IsValid() {
			errs[i] = errors.New("invalid account data")
			continue
		}

		// Generate account number if not provided
		if a.AccountNumber == "" {
			a.AccountNumber = fmt.Sprintf("ACC-%d-%d", time.Now().UnixNano(), i)
		}

		// Insert or update account
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (
				owner_type, owner_id, currency, purpose, account_type,
				is_active, is_locked, overdraft_limit,
				parent_agent_external_id, commission_rate,
				account_number, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (owner_type, owner_id, currency, purpose, account_type)
			DO UPDATE SET updated_at = EXCLUDED.updated_at
			RETURNING id, account_number, created_at, updated_at
		`,
			a.OwnerType,
			a.OwnerID,
			a.Currency,
			a.Purpose,
			a.AccountType,
			a.IsActive,
			a.IsLocked,
			a.OverdraftLimit,
			a.ParentAgentExternalID,
			a.CommissionRate,
			a.AccountNumber,
			now,
			now,
		).Scan(&a.ID, &a.AccountNumber, &a.CreatedAt, &a.UpdatedAt)

		if err != nil {
			errs[i] = fmt.Errorf("failed to insert account: %w", err)
			continue
		}

		// Initialize balance
		_, err = tx.Exec(ctx, `
			INSERT INTO balances (
				account_id, balance, available_balance, 
				pending_debit, pending_credit, version, updated_at
			)
			VALUES ($1, 0, 0, 0, 0, 0, $2)
			ON CONFLICT (account_id) DO NOTHING
		`, a.ID, now)

		if err != nil {
			errs[i] = fmt.Errorf("failed to insert balance: %w", err)
			continue
		}
	}

	return errs
}

// Update modifies an existing account within a transaction
// OPTIMIZED: Direct update by ID
func (r *accountRepo) Update(ctx context.Context, a *domain.Account, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	cmdTag, err := tx.Exec(ctx, `
		UPDATE accounts
		SET owner_type = $1,
		    owner_id = $2,
		    currency = $3,
		    purpose = $4,
		    account_type = $5,
		    is_active = $6,
		    is_locked = $7,
		    overdraft_limit = $8,
		    parent_agent_external_id = $9,
		    commission_rate = $10,
		    updated_at = $11
		WHERE id = $12
	`,
		a.OwnerType,
		a.OwnerID,
		a.Currency,
		a.Purpose,
		a.AccountType,
		a.IsActive,
		a.IsLocked,
		a.OverdraftLimit,
		a.ParentAgentExternalID,
		a.CommissionRate,
		time.Now(),
		a.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// UpdateMany updates multiple accounts within a transaction
// Returns error map keyed by account index
func (r *accountRepo) UpdateMany(ctx context.Context, accounts []*domain.Account, tx pgx.Tx) map[int]error {
	if tx == nil {
		return map[int]error{0: errors.New("transaction cannot be nil")}
	}

	errs := make(map[int]error)

	for i, a := range accounts {
		if err := r.Update(ctx, a, tx); err != nil {
			errs[i] = err
		}
	}

	return errs
}

// GetByFilter supports flexible filtering (avoid in hot path, use specific methods instead)
// OPTIMIZED: Returns early if no accounts found
func (r *accountRepo) GetByFilter(ctx context.Context, f *domain.AccountFilter) ([]*domain.Account, error) {
	query := baseSelectQuery + ` WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	if f.OwnerType != nil {
		query += fmt.Sprintf(` AND owner_type=$%d`, argPos)
		args = append(args, *f.OwnerType)
		argPos++
	}
	if f.OwnerID != nil {
		query += fmt.Sprintf(` AND owner_id=$%d`, argPos)
		args = append(args, *f.OwnerID)
		argPos++
	}
	if f.Currency != nil {
		query += fmt.Sprintf(` AND currency=$%d`, argPos)
		args = append(args, *f.Currency)
		argPos++
	}
	if f.Purpose != nil {
		query += fmt.Sprintf(` AND purpose=$%d`, argPos)
		args = append(args, *f.Purpose)
		argPos++
	}
	if f.AccountType != nil {
		query += fmt.Sprintf(` AND account_type=$%d`, argPos)
		args = append(args, *f.AccountType)
		argPos++
	}
	if f.IsActive != nil {
		query += fmt.Sprintf(` AND is_active=$%d`, argPos)
		args = append(args, *f.IsActive)
		argPos++
	}
	if f.IsLocked != nil {
		query += fmt.Sprintf(` AND is_locked=$%d`, argPos)
		args = append(args, *f.IsLocked)
		argPos++
	}
	if f.AccountNumber != nil {
		query += fmt.Sprintf(` AND account_number=$%d`, argPos)
		args = append(args, *f.AccountNumber)
		argPos++
	}
	if f.ParentAgentExternalID != nil {
		query += fmt.Sprintf(` AND parent_agent_external_id=$%d`, argPos)
		args = append(args, *f.ParentAgentExternalID)
		argPos++
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts by filter: %w", err)
	}

	accounts, err := scanAccountRows(rows)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return accounts, nil
}

// GetByParentAgent fetches all accounts under a specific agent (agent hierarchy)
// OPTIMIZED: Uses idx_accounts_agent_parent index
func (r *accountRepo) GetByParentAgent(ctx context.Context, agentExternalID string) ([]*domain.Account, error) {
	rows, err := r.db.Query(ctx,
		baseSelectQuery+` WHERE parent_agent_external_id=$1 AND account_type=$2`,
		agentExternalID, domain.AccountTypeReal)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts by parent agent: %w", err)
	}

	accounts, err := scanAccountRows(rows)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return accounts, nil
}

// GetByParentAgentTx same as GetByParentAgent but within a transaction
func (r *accountRepo) GetByParentAgentTx(ctx context.Context, agentExternalID string, tx pgx.Tx) ([]*domain.Account, error) {
	rows, err := tx.Query(ctx,
		baseSelectQuery+` WHERE parent_agent_external_id=$1 AND account_type=$2`,
		agentExternalID, domain.AccountTypeReal)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts by parent agent in tx: %w", err)
	}

	accounts, err := scanAccountRows(rows)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return accounts, nil
}

// Lock locks an account for maintenance or fraud prevention
// OPTIMIZED: Direct ID-based update
func (r *accountRepo) Lock(ctx context.Context, accountID int64, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	cmdTag, err := tx.Exec(ctx, `
		UPDATE accounts
		SET is_locked = true, updated_at = $1
		WHERE id = $2
	`, time.Now(), accountID)

	if err != nil {
		return fmt.Errorf("failed to lock account: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// Unlock unlocks an account
// OPTIMIZED: Direct ID-based update
func (r *accountRepo) Unlock(ctx context.Context, accountID int64, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	cmdTag, err := tx.Exec(ctx, `
		UPDATE accounts
		SET is_locked = false, updated_at = $1
		WHERE id = $2
	`, time.Now(), accountID)

	if err != nil {
		return fmt.Errorf("failed to unlock account: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

func (r *accountRepo) GetSystemAccount(
	ctx context.Context,
	currency string,
	accountType domain.AccountType,
) (*domain.Account, error) {
	// System accounts follow pattern: SYS-LIQ-{CURRENCY}
	accountNumber := fmt.Sprintf("SYS-LIQ-%s", currency)

	account, err := r.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, fmt.Errorf("system account not found for currency %s: %w", currency, err)
	}

	if account.OwnerType != domain.OwnerTypeSystem {
		return nil, errors.New("account is not a system account")
	}

	if account.Purpose != domain.PurposeLiquidity {
		return nil, fmt.Errorf("expected liquidity account, got %s", account.Purpose)
	}

	return account, nil
}

// GetSystemFeeAccount retrieves system fee collection account for a currency
// Pattern: SYS-FEE-{CURRENCY}
// OPTIMIZED: Uses unique account_number index
func (r *accountRepo) GetSystemFeeAccount(
	ctx context.Context,
	currency string,
) (*domain.Account, error) {
	// System fee accounts follow pattern: SYS-FEE-{CURRENCY}
	accountNumber := fmt.Sprintf("SYS-FEE-%s", currency)

	account, err := r.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, fmt.Errorf("system fee account not found for currency %s: %w", currency, err)
	}

	if account.OwnerType != domain.OwnerTypeSystem {
		return nil, errors.New("account is not a system account")
	}

	if account.Purpose != domain.PurposeFees {
		return nil, fmt.Errorf("expected fees account, got %s", account.Purpose)
	}

	return account, nil
}

// GetAgentAccount retrieves agent commission account
// Uses unique constraint: owner_type + owner_id + currency + purpose + account_type
// OPTIMIZED: Uses composite index idx_accounts_real
func (r *accountRepo) GetAgentAccount(
	ctx context.Context,
	agentExternalID string,
	currency string,
) (*domain.Account, error) {
	query := baseSelectQuery + `
		WHERE owner_type = $1
		  AND owner_id = $2
		  AND currency = $3
		  AND purpose = $4
		  AND account_type = 'real'
		  AND is_active = true
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query,
		domain.OwnerTypeAgent,
		agentExternalID,
		currency,
		domain.PurposeCommission,
	)

	account, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			return nil, fmt.Errorf("agent commission account not found for agent %s, currency %s: %w",
				agentExternalID, currency, xerrors.ErrAgentNotFound)
		}
		return nil, fmt.Errorf("failed to get agent account: %w", err)
	}

	return account, nil
}

// GetOrCreateAgentAccount ensures agent has a commission account
// Creates account if it doesn't exist
// CRITICAL: Must be called within a transaction
func (r *accountRepo) GetOrCreateAgentAccount(
	ctx context.Context,
	tx pgx.Tx,
	agentExternalID string,
	currency string,
	commissionRate *string,
) (*domain.Account, error) {
	if tx == nil {
		return nil, errors.New("transaction cannot be nil for GetOrCreateAgentAccount")
	}

	// Try to get existing account using transaction
	query := baseSelectQuery + `
		WHERE owner_type = $1
		  AND owner_id = $2
		  AND currency = $3
		  AND purpose = $4
		  AND account_type = 'real'
		LIMIT 1
	`

	row := tx.QueryRow(ctx, query,
		domain.OwnerTypeAgent,
		agentExternalID,
		currency,
		domain.PurposeCommission,
	)

	account, err := scanAccount(row)
	if err == nil {
		return account, nil // Already exists
	}

	if !errors.Is(err, xerrors.ErrNotFound) {
		return nil, err // Other error
	}

	// Create new agent commission account
	now := time.Now()
	newAccount := &domain.Account{
		OwnerType:      domain.OwnerTypeAgent,
		OwnerID:        agentExternalID,
		Currency:       currency,
		Purpose:        domain.PurposeCommission,
		AccountType:    domain.AccountTypeReal,
		IsActive:       true,
		IsLocked:       false,
		OverdraftLimit: 0,
		CommissionRate: commissionRate,
		AccountNumber:  fmt.Sprintf("AGT-COM-%s-%s-%d", agentExternalID, currency, now.UnixNano()),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Create account
	if err := r.Create(ctx, newAccount, tx); err != nil {
		return nil, fmt.Errorf("failed to create agent account: %w", err)
	}

	return newAccount, nil
}

// ========================================
// ADDITIONAL HELPER METHODS
// ========================================

// GetSystemAccountTx retrieves system account within a transaction
func (r *accountRepo) GetSystemAccountTx(
	ctx context.Context,
	tx pgx.Tx,
	currency string,
	accountType domain.AccountType,
) (*domain.Account, error) {
	accountNumber := fmt.Sprintf("SYS-LIQ-%s", currency)

	account, err := r.GetByAccountNumberTx(ctx, accountNumber, tx)
	if err != nil {
		return nil, fmt.Errorf("system account not found for currency %s: %w", currency, err)
	}

	if account.OwnerType != domain.OwnerTypeSystem {
		return nil, errors.New("account is not a system account")
	}

	return account, nil
}

// GetSystemFeeAccountTx retrieves system fee account within a transaction
func (r *accountRepo) GetSystemFeeAccountTx(
	ctx context.Context,
	tx pgx.Tx,
	currency string,
) (*domain.Account, error) {
	accountNumber := fmt.Sprintf("SYS-FEE-%s", currency)

	account, err := r.GetByAccountNumberTx(ctx, accountNumber, tx)
	if err != nil {
		return nil, fmt.Errorf("system fee account not found for currency %s: %w", currency, err)
	}

	if account.OwnerType != domain.OwnerTypeSystem {
		return nil, errors.New("account is not a system account")
	}

	return account, nil
}

// GetAgentAccountTx retrieves agent account within a transaction
func (r *accountRepo) GetAgentAccountTx(
	ctx context.Context,
	tx pgx.Tx,
	agentExternalID string,
	currency string,
) (*domain.Account, error) {
	query := baseSelectQuery + `
		WHERE owner_type = $1
		  AND owner_id = $2
		  AND currency = $3
		  AND purpose = $4
		  AND account_type = 'real'
		  AND is_active = true
		LIMIT 1
	`

	row := tx.QueryRow(ctx, query,
		domain.OwnerTypeAgent,
		agentExternalID,
		currency,
		domain.PurposeCommission,
	)

	account, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, xerrors.ErrNotFound) {
			return nil, fmt.Errorf("agent commission account not found for agent %s, currency %s: %w",
				agentExternalID, currency, xerrors.ErrAgentNotFound)
		}
		return nil, fmt.Errorf("failed to get agent account: %w", err)
	}

	return account, nil
}

// ========================================
// BATCH SYSTEM ACCOUNT OPERATIONS
// ========================================

// GetSystemAccountsForCurrencies retrieves system accounts for multiple currencies
// OPTIMIZED: Single query for batch lookup
func (r *accountRepo) GetSystemAccountsForCurrencies(
	ctx context.Context,
	currencies []string,
	accountType domain.AccountType,
) (map[string]*domain.Account, error) {
	if len(currencies) == 0 {
		return make(map[string]*domain.Account), nil
	}

	// Build account numbers
	accountNumbers := make([]string, len(currencies))
	for i, currency := range currencies {
		accountNumbers[i] = fmt.Sprintf("SYS-LIQ-%s", currency)
	}

	query := baseSelectQuery + ` WHERE account_number = ANY($1) AND owner_type = $2`

	rows, err := r.db.Query(ctx, query, accountNumbers, domain.OwnerTypeSystem)
	if err != nil {
		return nil, fmt.Errorf("failed to query system accounts: %w", err)
	}

	accounts, err := scanAccountRows(rows)
	if err != nil {
		return nil, err
	}

	// Map by currency
	result := make(map[string]*domain.Account, len(accounts))
	for _, account := range accounts {
		result[account.Currency] = account
	}

	return result, nil
}

// CreateSystemAccounts creates all required system accounts for a currency
// Creates: liquidity, fees, clearing, settlement accounts
// CRITICAL: Must be called within a transaction
func (r *accountRepo) CreateSystemAccounts(
	ctx context.Context,
	tx pgx.Tx,
	currency string,
	initialBalance float64,
) ([]*domain.Account, error) {
	if tx == nil {
		return nil, errors.New("transaction cannot be nil")
	}

	now := time.Now()

	systemAccounts := []*domain.Account{
		{
			OwnerType:     domain.OwnerTypeSystem,
			OwnerID:       "system",
			Currency:      currency,
			Purpose:       domain.PurposeLiquidity,
			AccountType:   domain.AccountTypeReal,
			IsActive:      true,
			AccountNumber: fmt.Sprintf("SYS-LIQ-%s", currency),
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			OwnerType:     domain.OwnerTypeSystem,
			OwnerID:       "system",
			Currency:      currency,
			Purpose:       domain.PurposeFees,
			AccountType:   domain.AccountTypeReal,
			IsActive:      true,
			AccountNumber: fmt.Sprintf("SYS-FEE-%s", currency),
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			OwnerType:     domain.OwnerTypeSystem,
			OwnerID:       "system",
			Currency:      currency,
			Purpose:       domain.PurposeClearing,
			AccountType:   domain.AccountTypeReal,
			IsActive:      true,
			AccountNumber: fmt.Sprintf("SYS-CLR-%s", currency),
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			OwnerType:     domain.OwnerTypeSystem,
			OwnerID:       "system",
			Currency:      currency,
			Purpose:       domain.PurposeSettlement,
			AccountType:   domain.AccountTypeReal,
			IsActive:      true,
			AccountNumber: fmt.Sprintf("SYS-SET-%s", currency),
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	errs := r.CreateMany(ctx, systemAccounts, tx)
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to create system accounts: %v", errs)
	}

	// Set initial balance for liquidity account if provided
	if initialBalance > 0 {
		_, err := tx.Exec(ctx, `
			UPDATE balances
			SET balance = $1, available_balance = $1, updated_at = $2
			WHERE account_id = $3
		`, initialBalance, now, systemAccounts[0].ID)

		if err != nil {
			return nil, fmt.Errorf("failed to set initial balance: %w", err)
		}
	}

	return systemAccounts, nil
}

package repository

import (
	"accounting-service/internal/domain"
	"context"
	"errors"
	//"log"
	"time"
	"fmt"

	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountRepository interface {
	GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error)
	GetByAccountNumberTx(ctx context.Context, accountNumber string, tx pgx.Tx) (*domain.Account, error)
	GetByOwner(ctx context.Context, ownerType, ownerID string) ([]*domain.Account, error)
	GetOrCreateUserAccounts(ctx context.Context, ownerType string, ownerID string,tx pgx.Tx) ([]*domain.Account, error)
	CreateMany(ctx context.Context, accounts []*domain.Account, tx pgx.Tx) map[int]error
	Update(ctx context.Context, a *domain.Account, tx pgx.Tx) error

	// Transaction helper
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type accountRepo struct {
	db *pgxpool.Pool
}

func NewAccountRepo(db *pgxpool.Pool) AccountRepository {
	return &accountRepo{db: db}
}

func (r *accountRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// GetByID fetches an account by its ID
func (r *accountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, owner_type, owner_id, currency, purpose, account_type, is_active, account_number, created_at, updated_at
		FROM accounts
		WHERE account_number=$1
	`, accountNumber)

	var a domain.Account
	err := row.Scan(
		&a.ID, &a.OwnerType, &a.OwnerID, &a.Currency, &a.Purpose, &a.AccountType,
		&a.IsActive, &a.AccountNumber, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *accountRepo) GetByAccountNumberTx(ctx context.Context, accountNumber string, tx pgx.Tx) (*domain.Account, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, owner_type, owner_id, currency, purpose, account_type, is_active, account_number, created_at, updated_at
		FROM accounts
		WHERE account_number=$1
	`, accountNumber)

	var a domain.Account
	err := row.Scan(
		&a.ID, &a.OwnerType, &a.OwnerID, &a.Currency, &a.Purpose, &a.AccountType,
		&a.IsActive, &a.AccountNumber, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}



// GetByOwner fetches all accounts for a given owner type and owner ID
func (r *accountRepo) GetByOwner(ctx context.Context, ownerType string, ownerID string) ([]*domain.Account, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_type, owner_id, currency, purpose, account_type, is_active, account_number, created_at, updated_at
		FROM accounts
		WHERE owner_type=$1 AND owner_id=$2
	`, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*domain.Account
	for rows.Next() {
		var a domain.Account
		if err := rows.Scan(
			&a.ID, &a.OwnerType, &a.OwnerID, &a.Currency, &a.Purpose, &a.AccountType,
			&a.IsActive, &a.AccountNumber, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		accounts = append(accounts, &a)
	}

	if len(accounts) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return accounts, nil
}


func (r *accountRepo) GetOrCreateUserAccounts(
	ctx context.Context,
	ownerType string,
	ownerID string,
	tx pgx.Tx, // required, must be part of a transaction
) ([]*domain.Account, error) {

	// Try to get existing accounts
	accounts, err := r.GetByOwner(ctx, ownerType, ownerID)
	if err == nil && len(accounts) > 0 {
		return accounts, nil // return all existing accounts
	}

	// If error is not "not found", return it
	if err != nil && !errors.Is(err, xerrors.ErrNotFound) {
		return nil, err
	}

	// No account exists → create a default USD account
	a := &domain.Account{
		OwnerType:  ownerType,
		OwnerID:    ownerID,
		Currency:   "USD",
		Purpose:    "wallet",
		AccountType:"real",
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Use CreateMany with single element slice
	errs := r.CreateMany(ctx, []*domain.Account{a}, tx)
	if err, exists := errs[0]; exists {
		return nil, err
	}

	// Return newly created account in a slice
	return []*domain.Account{a}, nil
}




// Create inserts a new account inside a transaction
// CreateMany inserts multiple accounts inside a transaction.
// Continues on error for individual accounts and returns a map of errors keyed by account index.
func (r *accountRepo) CreateMany(ctx context.Context, accounts []*domain.Account, tx pgx.Tx) map[int]error {
	if tx == nil {
		panic("transaction cannot be nil")
	}

	errs := make(map[int]error)
	now := time.Now()

	for i, a := range accounts {
		// Prepare account number only if not set
		if a.AccountNumber == "" {
			a.AccountNumber = fmt.Sprintf("WL-%d", time.Now().UnixNano())
		}

		// Insert or update
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (
				owner_type, owner_id, currency, purpose, account_type, is_active, account_number, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (owner_type, owner_id, currency, purpose, account_type) DO UPDATE
			SET updated_at = EXCLUDED.updated_at
			RETURNING id, account_number
		`,
			a.OwnerType,
			a.OwnerID,
			a.Currency,
			a.Purpose,
			a.AccountType,
			a.IsActive,
			a.AccountNumber,
			now,
			now,
		).Scan(&a.ID, &a.AccountNumber) // <-- always get the actual number from DB

		if err != nil {
			errs[i] = err
			continue
		}
	}

	return errs
}





// Update modifies an existing account inside a transaction
func (r *accountRepo) Update(ctx context.Context, a *domain.Account, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	cmdTag, err := tx.Exec(ctx, `
		UPDATE accounts
		SET owner_type=$1,
		    owner_id=$2,
		    currency=$3,
		    purpose=$4,
		    account_type=$5,
		    is_active=$6,
		    account_number=$7,
		    updated_at=$8
		WHERE id=$9
	`,
		a.OwnerType,
		a.OwnerID,
		a.Currency,
		a.Purpose,
		a.AccountType,
		a.IsActive,
		a.AccountNumber, // new field
		time.Now(),
		a.ID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}
	return nil
}


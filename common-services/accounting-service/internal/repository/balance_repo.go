package repository

import (
	"context"
	"accounting-service/internal/domain"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	xerrors "x/shared/utils/errors"
)

type BalanceRepository interface {
	GetByAccountID(ctx context.Context, accountID int64) (*domain.Balance, error)
	UpdateBalance(ctx context.Context, accountID int64, drCr string, amount float64, tx pgx.Tx) error
	GetCachedBalance(ctx context.Context, accountNumber string) (*domain.Balance, error)
}

type balanceRepo struct {
	db *pgxpool.Pool
}

func NewBalanceRepo(db *pgxpool.Pool) BalanceRepository {
	return &balanceRepo{db: db}
}

// GetByAccountID fetches the balance for a specific account
func (r *balanceRepo) GetByAccountID(ctx context.Context, accountID int64) (*domain.Balance, error) {
	row := r.db.QueryRow(ctx, `
		SELECT account_id, balance, updated_at
		FROM balances
		WHERE account_id=$1
	`, accountID)

	var b domain.Balance
	err := row.Scan(&b.AccountID, &b.Balance, &b.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

// UpdateBalance updates the balance based on DR/CR inside a transaction
func (r *balanceRepo) UpdateBalance(ctx context.Context, accountID int64, drCr string, amount float64, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	if drCr != "DR" && drCr != "CR" {
		return errors.New("invalid DR/CR value")
	}

	// Try to fetch current balance
	var current float64
	row := tx.QueryRow(ctx, `SELECT balance FROM balances WHERE account_id=$1`, accountID)
	err := row.Scan(&current)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Balance row doesn't exist yet → create it
			initialBalance := 0.0
			if drCr == "DR" {
				initialBalance -= amount
			} else {
				initialBalance += amount
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO balances (account_id, balance, updated_at)
				VALUES ($1, $2, $3)
			`, accountID, initialBalance, time.Now())
			return err
		}
		return err
	}

	// Update existing balance
	newBalance := current
	if drCr == "DR" {
		newBalance -= amount
	} else {
		newBalance += amount
	}

	cmdTag, err := tx.Exec(ctx, `
		UPDATE balances
		SET balance=$1, updated_at=$2
		WHERE account_id=$3
	`, newBalance, time.Now(), accountID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}
	return nil
}


func (r *balanceRepo) GetCurrentBalance(ctx context.Context, accountNumber string) (*domain.Balance, error) {
	var accountID int64
	err := r.db.QueryRow(ctx, `SELECT id FROM accounts WHERE account_number = $1`, accountNumber).Scan(&accountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	row := r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN dr_cr='CR' THEN amount ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN dr_cr='DR' THEN amount ELSE 0 END), 0) AS balance
		FROM ledgers
		WHERE account_id=$1
	`, accountID)

	var b domain.Balance
	b.AccountID = accountID

	var balance int64
	if err := row.Scan(&balance); err != nil {
		if err == pgx.ErrNoRows {
			b.Balance = 0
			b.UpdatedAt = time.Now()
			return &b, nil
		}
		return nil, err
	}

	b.Balance = float64(balance)
	b.UpdatedAt = time.Now()
	return &b, nil
}

func (r *balanceRepo) GetCachedBalance(ctx context.Context, accountNumber string) (*domain.Balance, error) {
	row := r.db.QueryRow(ctx, `
		SELECT b.account_id, b.balance, b.updated_at
		FROM balances b
		JOIN accounts a ON a.id = b.account_id
		WHERE a.account_number = $1
	`, accountNumber)

	var b domain.Balance
	if err := row.Scan(&b.AccountID, &b.Balance, &b.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	return &b, nil
}

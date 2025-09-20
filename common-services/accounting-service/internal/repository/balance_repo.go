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

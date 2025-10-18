package repository

import (
	"context"

	"wallet-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletRepository struct {
	db *pgxpool.Pool
}

func NewWalletRepository(db *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{db: db}
}

// CreateWallet inserts a new wallet record
func (r *WalletRepository) CreateWallet(ctx context.Context, wallet *domain.Wallet) error {
	query := `INSERT INTO wallets 
	(user_id, currency, balance, available, locked, type, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`
	_, err := r.db.Exec(ctx, query,
		wallet.UserID, wallet.Currency,
		wallet.Balance, wallet.Available, wallet.Locked, wallet.Type)
	return err
}

// GetWalletByUserIDAndCurrency retrieves a wallet by user ID and currency
func (r *WalletRepository) GetWalletByUserIDAndCurrency(ctx context.Context, userID, currency string) (*domain.Wallet, error) {
	var w domain.Wallet
	query := `SELECT id, user_id, currency, balance, available, locked, type, created_at, updated_at
			  FROM wallets WHERE user_id = $1 AND currency = $2`
	err := r.db.QueryRow(ctx, query, userID, currency).
		Scan(&w.ID, &w.UserID, &w.Currency, &w.Balance, &w.Available, &w.Locked,
			&w.Type, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// UpdateWalletBalance updates the balance, available, and locked amounts
func (r *WalletRepository) UpdateWalletBalance(ctx context.Context, wallet *domain.Wallet) error {
	query := `UPDATE wallets 
			  SET balance = $1, available = $2, locked = $3, updated_at = NOW() 
			  WHERE user_id = $4 AND currency = $5`
	_, err := r.db.Exec(ctx, query,
		wallet.Balance, wallet.Available, wallet.Locked, wallet.UserID, wallet.Currency)
	return err
}

// ListUserWallets retrieves all wallets for a given user
func (r *WalletRepository) ListUserWallets(ctx context.Context, userID string) ([]*domain.Wallet, error) {
	query := `SELECT id, user_id, currency, balance, available, locked, type, created_at, updated_at 
			  FROM wallets WHERE user_id = $1`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []*domain.Wallet
	for rows.Next() {
		var w domain.Wallet
		if err := rows.Scan(&w.ID, &w.UserID, &w.Currency, &w.Balance, &w.Available,
			&w.Locked, &w.Type, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		wallets = append(wallets, &w)
	}
	return wallets, nil
}

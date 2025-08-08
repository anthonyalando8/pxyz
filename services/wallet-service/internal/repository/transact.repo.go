package repository

import (
	"context"

	"wallet-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletTransactionRepository struct {
	db *pgxpool.Pool
}

func NewWalletTransactionRepository(db *pgxpool.Pool) *WalletTransactionRepository {
	return &WalletTransactionRepository{db: db}
}

// CreateTransaction inserts a new transaction record
func (r *WalletTransactionRepository) CreateTransaction(ctx context.Context, tx *domain.WalletTransaction) error {
	query := `
		INSERT INTO wallet_transactions (
			wallet_id, user_id, currency, amount, tx_status,
			tx_type, description, ref_id, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, NOW()
		)
	`

	_, err := r.db.Exec(ctx, query,
		tx.WalletID, tx.UserID, tx.Currency,
		tx.Amount, tx.TxStatus, tx.TxType,
		tx.Description, tx.RefID)
	return err
}

// ListTransactionsByUserID returns all transactions for a given user
func (r *WalletTransactionRepository) ListTransactionsByUserID(ctx context.Context, userID string) ([]*domain.WalletTransaction, error) {
	query := `
		SELECT id, wallet_id, user_id, currency, amount, tx_status,
			   tx_type, description, ref_id, created_at
		FROM wallet_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*domain.WalletTransaction
	for rows.Next() {
		var tx domain.WalletTransaction
		var refID *string
		err := rows.Scan(
			&tx.ID, &tx.WalletID, &tx.UserID, &tx.Currency, &tx.Amount,
			&tx.TxStatus, &tx.TxType, &tx.Description, &refID, &tx.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tx.RefID = refID
		transactions = append(transactions, &tx)
	}

	return transactions, nil
}

// Optional: Get single transaction by ID
func (r *WalletTransactionRepository) GetTransactionByID(ctx context.Context, id string) (*domain.WalletTransaction, error) {
	query := `
		SELECT id, wallet_id, user_id, currency, amount, tx_status,
			   tx_type, description, ref_id, created_at
		FROM wallet_transactions
		WHERE id = $1
	`

	var tx domain.WalletTransaction
	var refID *string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&tx.ID, &tx.WalletID, &tx.UserID, &tx.Currency, &tx.Amount,
		&tx.TxStatus, &tx.TxType, &tx.Description, &refID, &tx.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	tx.RefID = refID
	return &tx, nil
}

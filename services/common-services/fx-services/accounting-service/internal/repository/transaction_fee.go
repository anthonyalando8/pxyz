package repository

import (
	"accounting-service/internal/domain"
	"context"
	"time"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionFeeRepository interface {
	CreateBatch(ctx context.Context, fees []*domain.TransactionFee, tx pgx.Tx) map[int]error 
	GetByReceipt(ctx context.Context, receiptCode string) ([]*domain.TransactionFee, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type transactionFeeRepo struct {
	db *pgxpool.Pool
}

func NewTransactionFeeRepo(db *pgxpool.Pool) TransactionFeeRepository {
	return &transactionFeeRepo{db: db}
}

func (r *transactionFeeRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.BeginTx(ctx, pgx.TxOptions{})
}

func (r *transactionFeeRepo) CreateBatch(ctx context.Context, fees []*domain.TransactionFee, tx pgx.Tx) map[int]error {
	if tx == nil {
		return map[int]error{0: fmt.Errorf("transaction cannot be nil")}
	}

	errs := make(map[int]error)
	now := time.Now()

	batch := &pgx.Batch{}
	for _, fee := range fees {
		batch.Queue(`
			INSERT INTO transaction_fees
				(receipt_code, fee_rule_id, fee_type, amount, currency, created_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id, created_at
		`,
			fee.ReceiptCode,
			fee.FeeRuleID,
			fee.FeeType,
			fee.Amount,
			fee.Currency,
			now,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i, fee := range fees {
		row := br.QueryRow()
		if err := row.Scan(&fee.ID, &fee.CreatedAt); err != nil {
			errs[i] = err
		}
	}

	return errs
}


func (r *transactionFeeRepo) GetByReceipt(ctx context.Context, receiptCode string) ([]*domain.TransactionFee, error) {
	query := `
		SELECT id, receipt_code, fee_rule_id, fee_type, amount, currency, created_at
		FROM transaction_fees
		WHERE receipt_code=$1
	`
	rows, err := r.db.Query(ctx, query, receiptCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fees []*domain.TransactionFee
	for rows.Next() {
		var fee domain.TransactionFee
		if err := rows.Scan(&fee.ID, &fee.ReceiptCode, &fee.FeeRuleID, &fee.FeeType,
			&fee.Amount, &fee.Currency, &fee.CreatedAt); err != nil {
			return nil, err
		}
		fees = append(fees, &fee)
	}
	return fees, nil
}

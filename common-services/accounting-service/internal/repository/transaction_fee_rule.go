package repository

import (
	"accounting-service/internal/domain"
	"context"
	"time"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionFeeRuleRepository interface {
	CreateBatch(ctx context.Context, rules []*domain.TransactionFeeRule, tx pgx.Tx) map[int]error
	GetByTypeAndCurrencies(ctx context.Context, txType, source, target string) (*domain.TransactionFeeRule, error)
	ListAll(ctx context.Context) ([]*domain.TransactionFeeRule, error)
	Delete(ctx context.Context, id int64, tx pgx.Tx) error
}

type transactionFeeRuleRepo struct {
	db *pgxpool.Pool
}

func NewTransactionFeeRuleRepo(db *pgxpool.Pool) TransactionFeeRuleRepository {
	return &transactionFeeRuleRepo{db: db}
}

func (r *transactionFeeRuleRepo) CreateBatch(ctx context.Context, rules []*domain.TransactionFeeRule, tx pgx.Tx) map[int]error {
	if tx == nil {
		return map[int]error{0: fmt.Errorf("transaction cannot be nil")}
	}

	errs := make(map[int]error)
	now := time.Now()

	batch := &pgx.Batch{}
	for _, rule := range rules {
		batch.Queue(`
			INSERT INTO transaction_fee_rules (
				transaction_type, source_currency, target_currency,
				fee_type, fee_value, min_fee, max_fee, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (transaction_type, source_currency, target_currency)
			DO UPDATE SET 
				fee_type = EXCLUDED.fee_type,
				fee_value = EXCLUDED.fee_value,
				min_fee = EXCLUDED.min_fee,
				max_fee = EXCLUDED.max_fee,
				created_at = EXCLUDED.created_at
			RETURNING id, transaction_type, source_currency, target_currency, 
				fee_type, fee_value, min_fee, max_fee, created_at
		`,
			rule.TransactionType,
			rule.SourceCurrency,
			rule.TargetCurrency,
			rule.FeeType,
			rule.FeeValue,
			rule.MinFee,
			rule.MaxFee,
			now,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i, rule := range rules {
		row := br.QueryRow()
		if err := row.Scan(
			&rule.ID,
			&rule.TransactionType,
			&rule.SourceCurrency,
			&rule.TargetCurrency,
			&rule.FeeType,
			&rule.FeeValue,
			&rule.MinFee,
			&rule.MaxFee,
			&rule.CreatedAt,
		); err != nil {
			errs[i] = err
		}
	}

	return errs
}



func (r *transactionFeeRuleRepo) GetByTypeAndCurrencies(ctx context.Context, txType, source, target string) (*domain.TransactionFeeRule, error) {
	query := `
		SELECT id, transaction_type, source_currency, target_currency,
		       fee_type, fee_value, min_fee, max_fee, created_at
		FROM transaction_fee_rules
		WHERE transaction_type=$1 AND source_currency=$2 AND target_currency=$3
	`
	row := r.db.QueryRow(ctx, query, txType, source, target)

	var rule domain.TransactionFeeRule
	err := row.Scan(&rule.ID, &rule.TransactionType, &rule.SourceCurrency, &rule.TargetCurrency,
		&rule.FeeType, &rule.FeeValue, &rule.MinFee, &rule.MaxFee, &rule.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *transactionFeeRuleRepo) ListAll(ctx context.Context) ([]*domain.TransactionFeeRule, error) {
	query := `
		SELECT id, transaction_type, source_currency, target_currency,
		       fee_type, fee_value, min_fee, max_fee, created_at
		FROM transaction_fee_rules
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*domain.TransactionFeeRule
	for rows.Next() {
		var rule domain.TransactionFeeRule
		if err := rows.Scan(&rule.ID, &rule.TransactionType, &rule.SourceCurrency, &rule.TargetCurrency,
			&rule.FeeType, &rule.FeeValue, &rule.MinFee, &rule.MaxFee, &rule.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *transactionFeeRuleRepo) Delete(ctx context.Context, id int64, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DELETE FROM transaction_fee_rules WHERE id=$1`, id)
	return err
}

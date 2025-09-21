package repository

import (
	"context"
	"fmt"
	"receipt-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgconn"
)

type ReceiptRepository interface {
	Create(ctx context.Context, r *domain.Receipt, tx pgx.Tx) error
	GetByCode(ctx context.Context, code string) (*domain.Receipt, error)
	ListByJournal(ctx context.Context, journalID int64) ([]*domain.Receipt, error)
	ExistsByCode(ctx context.Context, code string) (bool, error)
}

type receiptRepo struct {
	db *pgxpool.Pool
}

func NewReceiptRepo(db *pgxpool.Pool) ReceiptRepository {
	return &receiptRepo{db: db}
}

var ErrReceiptCodeExists = fmt.Errorf("receipt code already exists")

// Create inserts a new receipt. If tx is nil, it uses the pool directly.
func (r *receiptRepo) Create(ctx context.Context, rec *domain.Receipt, tx pgx.Tx) error {
	query := `
		INSERT INTO receipts
			(code, journal_id,
			 creditor_account_id, creditor_account_type,
			 debitor_account_id, debitor_account_type,
			 type, amount, currency, status,
			 coded_type, external_ref)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at
	`

	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query,
			rec.Code, rec.JournalID,
			rec.Creditor.ID, rec.Creditor.Type,
			rec.Debitor.ID, rec.Debitor.Type,
			rec.Type, rec.Amount, rec.Currency, rec.Status,
			rec.CodedType, rec.ExternalRef,
		)
	} else {
		row = r.db.QueryRow(ctx, query,
			rec.Code, rec.JournalID,
			rec.Creditor.ID, rec.Creditor.Type,
			rec.Debitor.ID, rec.Debitor.Type,
			rec.Type, rec.Amount, rec.Currency, rec.Status,
			rec.CodedType, rec.ExternalRef,
		)
	}

	if err := row.Scan(&rec.ID, &rec.CreatedAt); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return ErrReceiptCodeExists
		}
		return fmt.Errorf("failed to insert receipt: %w", err)
	}

	return nil
}

// ExistsByCode checks if a receipt code already exists.
func (r *receiptRepo) ExistsByCode(ctx context.Context, code string) (bool, error) {
	query := `SELECT 1 FROM receipts WHERE code = $1 LIMIT 1`
	var dummy int
	err := r.db.QueryRow(ctx, query, code).Scan(&dummy)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check receipt code existence: %w", err)
	}
	return true, nil
}

// GetByCode retrieves a receipt by its unique code
func (r *receiptRepo) GetByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	query := `
		SELECT id, code, journal_id,
		       creditor_account_id, creditor_account_type,
		       debitor_account_id, debitor_account_type,
		       type, amount, currency, status,
		       created_at, coded_type, external_ref
		FROM receipts
		WHERE code = $1
	`
	rec := &domain.Receipt{}
	err := r.db.QueryRow(ctx, query, code).Scan(
		&rec.ID,
		&rec.Code,
		&rec.JournalID,
		&rec.Creditor.ID,
		&rec.Creditor.Type,
		&rec.Debitor.ID,
		&rec.Debitor.Type,
		&rec.Type,
		&rec.Amount,
		&rec.Currency,
		&rec.Status,
		&rec.CreatedAt,
		&rec.CodedType,
		&rec.ExternalRef,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch receipt: %w", err)
	}
	return rec, nil
}

// ListByJournal returns all receipts for a given journal
func (r *receiptRepo) ListByJournal(ctx context.Context, journalID int64) ([]*domain.Receipt, error) {
	query := `
		SELECT id, code, journal_id,
		       creditor_account_id, creditor_account_type,
		       debitor_account_id, debitor_account_type,
		       type, amount, currency, status,
		       created_at, coded_type, external_ref
		FROM receipts
		WHERE journal_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, journalID)
	if err != nil {
		return nil, fmt.Errorf("failed to query receipts: %w", err)
	}
	defer rows.Close()

	var receipts []*domain.Receipt
	for rows.Next() {
		rec := &domain.Receipt{}
		if err := rows.Scan(
			&rec.ID,
			&rec.Code,
			&rec.JournalID,
			&rec.Creditor.ID,
			&rec.Creditor.Type,
			&rec.Debitor.ID,
			&rec.Debitor.Type,
			&rec.Type,
			&rec.Amount,
			&rec.Currency,
			&rec.Status,
			&rec.CreatedAt,
			&rec.CodedType,
			&rec.ExternalRef,
		); err != nil {
			return nil, fmt.Errorf("failed to scan receipt: %w", err)
		}
		receipts = append(receipts, rec)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("row iteration error: %w", rows.Err())
	}

	return receipts, nil
}

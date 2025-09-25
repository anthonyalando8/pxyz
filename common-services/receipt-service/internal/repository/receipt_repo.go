package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"receipt-service/internal/domain"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReceiptRepository interface {
	CreateBatch(ctx context.Context, receipts []*domain.Receipt, tx pgx.Tx) error
	GetByCode(ctx context.Context, code string) (*domain.Receipt, error)
	ExistsByCode(ctx context.Context, code string) (bool, error)
}

type receiptRepo struct {
	db *pgxpool.Pool
}

func NewReceiptRepo(db *pgxpool.Pool) ReceiptRepository {
	return &receiptRepo{db: db}
}

var (
	ErrReceiptCodeExists = errors.New("receipt code already exists")
	ErrReceiptNotFound   = errors.New("receipt not found")
)

// --- Create ---
func (r *receiptRepo) CreateBatch(ctx context.Context, receipts []*domain.Receipt, tx pgx.Tx) error {
	query := `
		INSERT INTO fx_receipts (
			code, type, coded_type, amount, currency, external_ref, status,
			creditor_account_id, creditor_ledger_id, creditor_account_type, creditor_status,
			debitor_account_id, debitor_ledger_id, debitor_account_type, debitor_status,
			created_at, updated_at, created_by, reversed_at, reversed_by, metadata
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21
		)
		RETURNING id, created_at
	`

	batch := &pgx.Batch{}
	for _, rc := range receipts {
		metadataJSON, err := json.Marshal(rc.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}

		batch.Queue(query,
			rc.Code, rc.Type, rc.CodedType, rc.Amount, rc.Currency, rc.ExternalRef, rc.Status,
			rc.Creditor.AccountID, rc.Creditor.LedgerID, rc.Creditor.AccountType, rc.Creditor.Status,
			rc.Debitor.AccountID, rc.Debitor.LedgerID, rc.Debitor.AccountType, rc.Debitor.Status,
			rc.CreatedAt, rc.UpdatedAt, rc.CreatedBy, rc.ReversedAt, rc.ReversedBy, metadataJSON,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for _, rc := range receipts {
		var createdAt time.Time
		if err := br.QueryRow().Scan(&rc.ID, &createdAt); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.ConstraintName == "fx_receipts_code_key" {
				return ErrReceiptCodeExists
			}
			return fmt.Errorf("insert receipt: %w", err)
		}
		rc.CreatedAt = createdAt
	}

	return nil
}


// --- GetByCode ---
func (r *receiptRepo) GetByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	query := `
		SELECT 
			id, code, type, coded_type, amount, currency, external_ref, status,
			creditor_account_id, creditor_ledger_id, creditor_account_type, creditor_status,
			debitor_account_id, debitor_ledger_id, debitor_account_type, debitor_status,
			created_at, updated_at, created_by, reversed_at, reversed_by, metadata
		FROM fx_receipts
		WHERE code = $1
	`

	row := r.db.QueryRow(ctx, query, code)

	var rc domain.Receipt
	var metadataJSON []byte
	err := row.Scan(
		&rc.ID, &rc.Code, &rc.Type, &rc.CodedType, &rc.Amount, &rc.Currency, &rc.ExternalRef, &rc.Status,
		&rc.Creditor.AccountID, &rc.Creditor.LedgerID, &rc.Creditor.AccountType, &rc.Creditor.Status,
		&rc.Debitor.AccountID, &rc.Debitor.LedgerID, &rc.Debitor.AccountType, &rc.Debitor.Status,
		&rc.CreatedAt, &rc.UpdatedAt, &rc.CreatedBy, &rc.ReversedAt, &rc.ReversedBy, &metadataJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReceiptNotFound
		}
		return nil, fmt.Errorf("get receipt by code: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &rc.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	// mark roles
	rc.Creditor.IsCreditor = true
	rc.Debitor.IsCreditor = false

	return &rc, nil
}

// --- ExistsByCode ---
func (r *receiptRepo) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM fx_receipts WHERE code = $1)`
	if err := r.db.QueryRow(ctx, query, code).Scan(&exists); err != nil {
		return false, fmt.Errorf("exists by code: %w", err)
	}
	return exists, nil
}
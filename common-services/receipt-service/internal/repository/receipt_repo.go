package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"receipt-service/internal/domain"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReceiptRepository interface {
	CreateBatch(ctx context.Context, receipts []*domain.Receipt, tx pgx.Tx) error
	GetByCode(ctx context.Context, code string) (*domain.Receipt, error)
	ExistsByCode(ctx context.Context, code string) (bool, error)
	CreateBatchTx(ctx context.Context, tx pgx.Tx, receipts []*domain.Receipt) error
	UpdateBatch(ctx context.Context, updates []*domain.ReceiptUpdate) ([]*domain.Receipt, error)
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

// CreateBatch automatically manages transaction if tx is nil.
func (r *receiptRepo) CreateBatch(ctx context.Context, receipts []*domain.Receipt, tx pgx.Tx) error {
    if tx != nil {
        return r.CreateBatchTx(ctx, tx, receipts)
    }

    dbTx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer func() {
        if err != nil {
            _ = dbTx.Rollback(ctx)
        } else {
            _ = dbTx.Commit(ctx)
        }
    }()

    return r.CreateBatchTx(ctx, dbTx, receipts)
}

// CreateBatchTx executes insert inside an existing tx
func (r *receiptRepo) CreateBatchTx(ctx context.Context, tx pgx.Tx, receipts []*domain.Receipt) error {
	query := `
        INSERT INTO fx_receipts (
            code, type, coded_type, amount, transaction_cost, currency, external_ref, status,
            creditor_account_id, creditor_ledger_id, creditor_account_type, creditor_status,
            debitor_account_id, debitor_ledger_id, debitor_account_type, debitor_status,
            created_at, updated_at, created_by, reversed_at, reversed_by, metadata
        )
        VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8,
            $9, $10, $11, $12,
            $13, $14, $15, $16,
            $17, $18, $19, $20, $21, $22
        )
        ON CONFLICT (code) DO NOTHING
        RETURNING id, created_at
    `

	batch := &pgx.Batch{}
	for _, rc := range receipts {
		metadataJSON, err := json.Marshal(rc.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}

		batch.Queue(query,
			rc.Code, rc.Type, rc.CodedType, rc.Amount, rc.TransactionCost, rc.Currency, rc.ExternalRef, rc.Status,
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
			if errors.Is(err, pgx.ErrNoRows) {
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
			id, code, type, coded_type, amount, transaction_cost, currency, external_ref, status,
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
		&rc.ID, &rc.Code, &rc.Type, &rc.CodedType, &rc.Amount, &rc.TransactionCost, &rc.Currency, &rc.ExternalRef, &rc.Status,
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

// UpdateBatch updates multiple receipts and returns their new state
func (r *receiptRepo) UpdateBatch(ctx context.Context, updates []*domain.ReceiptUpdate) ([]*domain.Receipt, error) {
	if len(updates) == 0 {
		return nil, nil
	}

	// Build VALUES for batch update
	// We'll use a CTE with jsonb metadata merge
	query := `
	WITH u (code, status, creditor_status, debitor_status, reversed_by, reversed_at, metadata) AS (
		VALUES %s
	)
	UPDATE fx_receipts r
	SET 
		status = COALESCE(NULLIF(u.status, ''), r.status),
		creditor_status = COALESCE(NULLIF(u.creditor_status, ''), r.creditor_status),
		debitor_status = COALESCE(NULLIF(u.debitor_status, ''), r.debitor_status),
		reversed_by = COALESCE(NULLIF(u.reversed_by, ''), r.reversed_by),
		reversed_at = COALESCE(u.reversed_at, r.reversed_at),
		metadata = r.metadata || u.metadata,
		updated_at = now()
	FROM u
	WHERE r.code = u.code
	RETURNING 
		r.id, r.code, r.type, r.coded_type, r.amount, r.currency, r.external_ref, r.status,
		r.creditor_account_id, r.creditor_ledger_id, r.creditor_account_type, r.creditor_status,
		r.debitor_account_id, r.debitor_ledger_id, r.debitor_account_type, r.debitor_status,
		r.created_at, r.updated_at, r.created_by, r.reversed_at, r.reversed_by, r.metadata
	`

	// Prepare args for VALUES
	valueStrings := make([]string, 0, len(updates))
	valueArgs := make([]any, 0, len(updates)*7)

	for i, upd := range updates {
		metadataJSON, err := json.Marshal(upd.MetadataPatch)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata patch: %w", err)
		}

		// 7 cols
		valueStrings = append(valueStrings,
			fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7))

		valueArgs = append(valueArgs,
			upd.Code,
			upd.Status,
			upd.CreditorStatus,
			upd.DebitorStatus,
			upd.ReversedBy,
			upd.ReversedAt,
			metadataJSON,
		)
	}

	sql := fmt.Sprintf(query, strings.Join(valueStrings, ","))

	rows, err := r.db.Query(ctx, sql, valueArgs...)
	if err != nil {
		return nil, fmt.Errorf("update batch: %w", err)
	}
	defer rows.Close()

	var results []*domain.Receipt
	for rows.Next() {
		var rc domain.Receipt
		var metadataJSON []byte
		if err := rows.Scan(
			&rc.ID, &rc.Code, &rc.Type, &rc.CodedType, &rc.Amount, &rc.Currency, &rc.ExternalRef, &rc.Status,
			&rc.Creditor.AccountID, &rc.Creditor.LedgerID, &rc.Creditor.AccountType, &rc.Creditor.Status,
			&rc.Debitor.AccountID, &rc.Debitor.LedgerID, &rc.Debitor.AccountType, &rc.Debitor.Status,
			&rc.CreatedAt, &rc.UpdatedAt, &rc.CreatedBy, &rc.ReversedAt, &rc.ReversedBy, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("scan updated receipt: %w", err)
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &rc.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}
		rc.Creditor.IsCreditor = true
		rc.Debitor.IsCreditor = false

		results = append(results, &rc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration: %w", err)
	}

	return results, nil
}


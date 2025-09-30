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
    // Build VALUES placeholders
    lookupValues := make([]string, len(receipts))
    receiptValues := make([]string, len(receipts))
    args := []interface{}{}
    argPos := 1

    for i, rc := range receipts {
        // Add lookup value (code only)
        lookupValues[i] = fmt.Sprintf("($%d)", argPos)
        args = append(args, rc.Code)
        argPos++

        // Marshal metadata
        metadataJSON, err := json.Marshal(rc.Metadata)
        if err != nil {
            return fmt.Errorf("marshal metadata: %w", err)
        }

        // Add full receipt row
        receiptValues[i] = fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
            argPos, argPos+1, argPos+2, argPos+3, argPos+4, argPos+5, argPos+6, argPos+7,
            argPos+8, argPos+9, argPos+10, argPos+11,
            argPos+12, argPos+13, argPos+14, argPos+15,
            argPos+16, argPos+17, argPos+18, argPos+19, argPos+20, argPos+21,
        )
        args = append(args,
            rc.Code, rc.Type, rc.CodedType, rc.Amount, rc.TransactionCost, rc.Currency, rc.ExternalRef, rc.Status,
            rc.Creditor.AccountID, rc.Creditor.LedgerID, rc.Creditor.AccountType, rc.Creditor.Status,
            rc.Debitor.AccountID, rc.Debitor.LedgerID, rc.Debitor.AccountType, rc.Debitor.Status,
            rc.CreatedAt, rc.UpdatedAt, rc.CreatedBy, rc.ReversedAt, rc.ReversedBy, metadataJSON,
        )
        argPos += 22
    }

    query := fmt.Sprintf(`
        WITH lookup_ins AS (
            INSERT INTO receipt_lookup (code)
            VALUES %s
            ON CONFLICT (code) DO UPDATE SET code = EXCLUDED.code
            RETURNING id, code
        )
        INSERT INTO fx_receipts (
            lookup_id, type, coded_type, amount, transaction_cost, currency, external_ref, status,
            creditor_account_id, creditor_ledger_id, creditor_account_type, creditor_status,
            debitor_account_id, debitor_ledger_id, debitor_account_type, debitor_status,
            created_at, updated_at, created_by, reversed_at, reversed_by, metadata
        )
        SELECT l.id, r.type, r.coded_type, r.amount, r.transaction_cost, r.currency, r.external_ref, r.status,
               r.creditor_account_id, r.creditor_ledger_id, r.creditor_account_type, r.creditor_status,
               r.debitor_account_id, r.debitor_ledger_id, r.debitor_account_type, r.debitor_status,
               r.created_at, r.updated_at, r.created_by, r.reversed_at, r.reversed_by, r.metadata
        FROM (VALUES %s) AS r (
            code, type, coded_type, amount, transaction_cost, currency, external_ref, status,
            creditor_account_id, creditor_ledger_id, creditor_account_type, creditor_status,
            debitor_account_id, debitor_ledger_id, debitor_account_type, debitor_status,
            created_at, updated_at, created_by, reversed_at, reversed_by, metadata
        )
        JOIN lookup_ins l ON l.code = r.code
        RETURNING lookup_id, created_at;
    `, strings.Join(lookupValues, ","), strings.Join(receiptValues, ","))

    rows, err := tx.Query(ctx, query, args...)
    if err != nil {
        return fmt.Errorf("batch insert: %w", err)
    }
    defer rows.Close()

    i := 0
    for rows.Next() {
        var createdAt time.Time
        if err := rows.Scan(&receipts[i].ID, &createdAt); err != nil {
            return fmt.Errorf("scan: %w", err)
        }
        receipts[i].CreatedAt = createdAt
        i++
    }

    return rows.Err()
}


// --- GetByCode ---
func (r *receiptRepo) GetByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	query := `
		SELECT 
			rl.id, rl.code,
			fr.type, fr.coded_type, fr.amount, fr.transaction_cost, fr.currency, fr.external_ref, fr.status,
			fr.creditor_account_id, fr.creditor_ledger_id, fr.creditor_account_type, fr.creditor_status,
			fr.debitor_account_id, fr.debitor_ledger_id, fr.debitor_account_type, fr.debitor_status,
			fr.created_at, fr.updated_at, fr.created_by, fr.reversed_at, fr.reversed_by, fr.metadata
		FROM fx_receipts fr
		JOIN receipt_lookup rl ON rl.id = fr.lookup_id
		WHERE rl.code = $1
		ORDER BY fr.created_at DESC
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query, code)

	var rc domain.Receipt
	var metadataJSON []byte
	err := row.Scan(
		&rc.ID, &rc.Code,
		&rc.Type, &rc.CodedType, &rc.Amount, &rc.TransactionCost, &rc.Currency, &rc.ExternalRef, &rc.Status,
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

	// Mark roles
	rc.Creditor.IsCreditor = true
	rc.Debitor.IsCreditor = false

	return &rc, nil
}


// --- ExistsByCode ---
func (r *receiptRepo) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM fx_receipts fr
			JOIN receipt_lookup rl ON rl.id = fr.lookup_id
			WHERE rl.code = $1
		)
	`
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

	// SQL template with join against receipt_lookup
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
		metadata = CASE 
			WHEN u.metadata = '{}'::jsonb THEN r.metadata 
			ELSE r.metadata || u.metadata 
		END,
		updated_at = now()
	FROM receipt_lookup l
	JOIN u ON u.code = l.code
	WHERE r.lookup_id = l.id
	RETURNING 
		r.lookup_id, l.code, r.type, r.coded_type, r.amount, r.transaction_cost, r.currency,
		r.external_ref, r.status,
		r.creditor_account_id, r.creditor_ledger_id, r.creditor_account_type, r.creditor_status,
		r.debitor_account_id, r.debitor_ledger_id, r.debitor_account_type, r.debitor_status,
		r.created_at, r.updated_at, r.created_by, r.reversed_at, r.reversed_by, r.metadata
	`

	// Prepare VALUES
	valueStrings := make([]string, 0, len(updates))
	valueArgs := make([]any, 0, len(updates)*7)

	for i, upd := range updates {
		metadataJSON, err := json.Marshal(upd.MetadataPatch)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata patch: %w", err)
		}
		if string(metadataJSON) == "null" {
			metadataJSON = []byte("{}") // fallback to empty object
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
			&rc.ID, &rc.Code, &rc.Type, &rc.CodedType, &rc.Amount, &rc.TransactionCost, &rc.Currency,
			&rc.ExternalRef, &rc.Status,
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



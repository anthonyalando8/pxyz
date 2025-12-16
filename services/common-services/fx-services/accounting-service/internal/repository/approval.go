// repository/transaction_approval_repository.go
package repository

import (
    "context"
    "encoding/json"
    "fmt"

    "accounting-service/internal/domain"
    
    "github.com/jackc/pgx/v5/pgxpool"
)

type TransactionApprovalRepository interface {
    Create(ctx context.Context, approval *domain.TransactionApproval) error
    GetByID(ctx context.Context, id int64) (*domain.TransactionApproval, error)
    List(ctx context.Context, filter *domain.ApprovalFilter) ([]*domain.TransactionApproval, int64, error)
    UpdateStatus(ctx context.Context, id int64, status domain.ApprovalStatus, approvedBy *int64, reason *string) error
    MarkExecuted(ctx context.Context, id int64, receiptCode string) error
    MarkFailed(ctx context.Context, id int64, errorMsg string) error
}

type transactionApprovalRepo struct {
    db *pgxpool.Pool
}

func NewTransactionApprovalRepository(db *pgxpool.Pool) TransactionApprovalRepository {
    return &transactionApprovalRepo{db: db}
}

func (r *transactionApprovalRepo) Create(ctx context. Context, approval *domain.TransactionApproval) error {
    query := `
        INSERT INTO transaction_approvals (
            requested_by, transaction_type, account_number, amount, currency,
            description, to_account_number, status, request_metadata,
            created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `

    metadataJSON, _ := json.Marshal(approval.RequestMetadata)

    return r.db.QueryRow(ctx, query,
        approval.RequestedBy,
        approval.TransactionType,
        approval.AccountNumber,
        approval.Amount,
        approval.Currency,
        approval.Description,
        approval.ToAccountNumber,
        approval.Status,
        metadataJSON,
    ).Scan(&approval.ID, &approval.CreatedAt, &approval.UpdatedAt)
}

func (r *transactionApprovalRepo) GetByID(ctx context.Context, id int64) (*domain.TransactionApproval, error) {
    query := `
        SELECT 
            id, requested_by, transaction_type, account_number, amount, currency,
            description, to_account_number, status, approved_by, rejection_reason,
            receipt_code, error_message, request_metadata,
            created_at, updated_at, approved_at, executed_at
        FROM transaction_approvals
        WHERE id = $1
    `

    var approval domain.TransactionApproval
    err := r.db. QueryRow(ctx, query, id).Scan(
        &approval.ID,
        &approval. RequestedBy,
        &approval.TransactionType,
        &approval.AccountNumber,
        &approval.Amount,
        &approval.Currency,
        &approval. Description,
        &approval.ToAccountNumber,
        &approval. Status,
        &approval.ApprovedBy,
        &approval.RejectionReason,
        &approval.ReceiptCode,
        &approval.ErrorMessage,
        &approval.RequestMetadata,
        &approval.CreatedAt,
        &approval. UpdatedAt,
        &approval.ApprovedAt,
        &approval.ExecutedAt,
    )

    if err != nil {
        return nil, err
    }

    return &approval, nil
}

func (r *transactionApprovalRepo) List(ctx context.Context, filter *domain.ApprovalFilter) ([]*domain.TransactionApproval, int64, error) {
    // Build query
    baseQuery := `
        SELECT 
            id, requested_by, transaction_type, account_number, amount, currency,
            description, to_account_number, status, approved_by, rejection_reason,
            receipt_code, error_message, request_metadata,
            created_at, updated_at, approved_at, executed_at
        FROM transaction_approvals
        WHERE 1=1
    `
    countQuery := `SELECT COUNT(*) FROM transaction_approvals WHERE 1=1`

    args := []interface{}{}
    argIndex := 1

    // Add filters
    if filter.Status != nil {
        baseQuery += fmt.Sprintf(" AND status = $%d", argIndex)
        countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
        args = append(args, *filter. Status)
        argIndex++
    }

    if filter.RequestedBy != nil {
        baseQuery += fmt.Sprintf(" AND requested_by = $%d", argIndex)
        countQuery += fmt.Sprintf(" AND requested_by = $%d", argIndex)
        args = append(args, *filter.RequestedBy)
        argIndex++
    }

    if filter.FromDate != nil {
        baseQuery += fmt.Sprintf(" AND created_at >= $%d", argIndex)
        countQuery += fmt.Sprintf(" AND created_at >= $%d", argIndex)
        args = append(args, *filter.FromDate)
        argIndex++
    }

    if filter.ToDate != nil {
        baseQuery += fmt. Sprintf(" AND created_at <= $%d", argIndex)
        countQuery += fmt.Sprintf(" AND created_at <= $%d", argIndex)
        args = append(args, *filter.ToDate)
        argIndex++
    }

    // Get total count
    var total int64
    if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
        return nil, 0, err
    }

    // Add pagination
    baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
    args = append(args, filter. Limit, filter.Offset)

    // Execute query
    rows, err := r.db.Query(ctx, baseQuery, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var approvals []*domain.TransactionApproval
    for rows.Next() {
        var approval domain.TransactionApproval
        if err := rows. Scan(
            &approval.ID,
            &approval.RequestedBy,
            &approval.TransactionType,
            &approval. AccountNumber,
            &approval. Amount,
            &approval.Currency,
            &approval.Description,
            &approval.ToAccountNumber,
            &approval.Status,
            &approval.ApprovedBy,
            &approval.RejectionReason,
            &approval.ReceiptCode,
            &approval.ErrorMessage,
            &approval.RequestMetadata,
            &approval.CreatedAt,
            &approval.UpdatedAt,
            &approval.ApprovedAt,
            &approval.ExecutedAt,
        ); err != nil {
            return nil, 0, err
        }
        approvals = append(approvals, &approval)
    }

    return approvals, total, nil
}

func (r *transactionApprovalRepo) UpdateStatus(ctx context.Context, id int64, status domain.ApprovalStatus, approvedBy *int64, reason *string) error {
    query := `
        UPDATE transaction_approvals
        SET 
            status = $1,
            approved_by = $2,
            rejection_reason = $3,
            approved_at = CASE WHEN $1 IN ('approved', 'rejected') THEN NOW() ELSE approved_at END,
            updated_at = NOW()
        WHERE id = $4
    `

    _, err := r.db.Exec(ctx, query, status, approvedBy, reason, id)
    return err
}

func (r *transactionApprovalRepo) MarkExecuted(ctx context.Context, id int64, receiptCode string) error {
    query := `
        UPDATE transaction_approvals
        SET 
            status = 'executed',
            receipt_code = $1,
            executed_at = NOW(),
            updated_at = NOW()
        WHERE id = $2
    `

    _, err := r.db.Exec(ctx, query, receiptCode, id)
    return err
}

func (r *transactionApprovalRepo) MarkFailed(ctx context.Context, id int64, errorMsg string) error {
    query := `
        UPDATE transaction_approvals
        SET 
            status = 'failed',
            error_message = $1,
            executed_at = NOW(),
            updated_at = NOW()
        WHERE id = $2
    `

    _, err := r.db. Exec(ctx, query, errorMsg, id)
    return err
}
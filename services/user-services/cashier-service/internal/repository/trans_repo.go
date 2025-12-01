// repository/deposit. go
package repository

import (
    "context"
    //"fmt"
    "cashier-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"

)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}


func (r *UserRepo) CreateDepositRequest(ctx context.Context, req *domain.DepositRequest) error {
    query := `
        INSERT INTO deposit_requests 
        (user_id, partner_id, request_ref, amount, currency, service, payment_method, status, metadata, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        RETURNING id, created_at, updated_at
    `
    return r.db. QueryRow(ctx, query,
        req.UserID, req.PartnerID, req. RequestRef, req.Amount, req.Currency,
        req.Service, req. PaymentMethod, req.Status, req.Metadata, req.ExpiresAt,
    ).Scan(&req. ID, &req.CreatedAt, &req.UpdatedAt)
}

func (r *UserRepo) UpdateDepositStatus(ctx context.Context, id int64, status string, errorMsg *string) error {
    query := `
        UPDATE deposit_requests 
        SET status = $1, error_message = $2, updated_at = NOW()
        WHERE id = $3
    `
    _, err := r.db. Exec(ctx, query, status, errorMsg, id)
    return err
}

func (r *UserRepo) UpdateDepositWithPartnerRef(ctx context.Context, id int64, partnerRef string) error {
    query := `
        UPDATE deposit_requests 
        SET partner_transaction_ref = $1, status = 'processing', updated_at = NOW()
        WHERE id = $2
    `
    _, err := r. db.Exec(ctx, query, partnerRef, id)
    return err
}

func (r *UserRepo) UpdateDepositWithReceipt(ctx context.Context, id int64, receiptCode string, journalID int64) error {
    query := `
        UPDATE deposit_requests 
        SET 
            receipt_code = $1,
            journal_id = $2,
            status = 'completed',
            completed_at = NOW(),
            updated_at = NOW()
        WHERE id = $3
    `
    _, err := r.db. Exec(ctx, query, receiptCode, journalID, id)
    return err
}

func (r *UserRepo) GetDepositByRef(ctx context. Context, requestRef string) (*domain.DepositRequest, error) {
    query := `
        SELECT 
            id, user_id, partner_id, request_ref, amount, currency, service, payment_method,
            status, partner_transaction_ref, receipt_code, journal_id, metadata, error_message,
            expires_at, created_at, updated_at, completed_at
        FROM deposit_requests
        WHERE request_ref = $1
    `
    var req domain.DepositRequest
    err := r.db.QueryRow(ctx, query, requestRef).Scan(
        &req.ID, &req.UserID, &req.PartnerID, &req.RequestRef, &req.Amount, &req.Currency,
        &req.Service, &req.PaymentMethod, &req.Status, &req. PartnerTransactionRef,
        &req.ReceiptCode, &req.JournalID, &req. Metadata, &req.ErrorMessage,
        &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt, &req.CompletedAt,
    )
    if err != nil {
        return nil, err
    }
    return &req, nil
}

func (r *UserRepo) ListDeposits(ctx context.Context, userID int64, limit, offset int) ([]domain.DepositRequest, int64, error) {
    // Implementation similar to partner transactions
    var deposits []domain.DepositRequest
    var total int64
    
    // Count query
    countQuery := `SELECT COUNT(*) FROM deposit_requests WHERE user_id = $1`
    r.db.QueryRow(ctx, countQuery, userID).Scan(&total)
    
    // Data query
    query := `
        SELECT 
            id, user_id, partner_id, request_ref, amount, currency, service, payment_method,
            status, partner_transaction_ref, receipt_code, journal_id, metadata, error_message,
            expires_at, created_at, updated_at, completed_at
        FROM deposit_requests
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
    
    rows, err := r.db.Query(ctx, query, userID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var req domain. DepositRequest
        if err := rows.Scan(
            &req.ID, &req.UserID, &req. PartnerID, &req.RequestRef, &req.Amount, &req.Currency,
            &req.Service, &req.PaymentMethod, &req.Status, &req.PartnerTransactionRef,
            &req.ReceiptCode, &req.JournalID, &req.Metadata, &req.ErrorMessage,
            &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt, &req.CompletedAt,
        ); err != nil {
            return nil, 0, err
        }
        deposits = append(deposits, req)
    }
    
    return deposits, total, nil
}

// Similar methods for withdrawal_requests... 
func (r *UserRepo) CreateWithdrawalRequest(ctx context.Context, req *domain.WithdrawalRequest) error {
    query := `
        INSERT INTO withdrawal_requests 
        (user_id, request_ref, amount, currency, destination, service, status, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, created_at, updated_at
    `
    return r. db.QueryRow(ctx, query,
        req.UserID, req.RequestRef, req.Amount, req.Currency,
        req.Destination, req.Service, req.Status, req.Metadata,
    ).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
}

func (r *UserRepo) UpdateWithdrawalStatus(ctx context.Context, id int64, status string, errorMsg *string) error {
    query := `
        UPDATE withdrawal_requests 
        SET status = $1, error_message = $2, updated_at = NOW()
        WHERE id = $3
    `
    _, err := r.db.Exec(ctx, query, status, errorMsg, id)
    return err
}

func (r *UserRepo) UpdateWithdrawalWithReceipt(ctx context.Context, id int64, receiptCode string, journalID int64) error {
    query := `
        UPDATE withdrawal_requests 
        SET 
            receipt_code = $1,
            journal_id = $2,
            status = 'completed',
            completed_at = NOW(),
            updated_at = NOW()
        WHERE id = $3
    `
    _, err := r. db.Exec(ctx, query, receiptCode, journalID, id)
    return err
}

func (r *UserRepo) ListWithdrawals(ctx context. Context, userID int64, limit, offset int) ([]domain. WithdrawalRequest, int64, error) {
    // Similar implementation to ListDeposits
    var withdrawals []domain.WithdrawalRequest
    var total int64
    
    countQuery := `SELECT COUNT(*) FROM withdrawal_requests WHERE user_id = $1`
    r.db.QueryRow(ctx, countQuery, userID).Scan(&total)
    
    query := `
        SELECT 
            id, user_id, request_ref, amount, currency, destination, service,
            status, receipt_code, journal_id, metadata, error_message,
            created_at, updated_at, completed_at
        FROM withdrawal_requests
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
    
    rows, err := r.db.Query(ctx, query, userID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows. Close()
    
    for rows.Next() {
        var req domain.WithdrawalRequest
        if err := rows.Scan(
            &req.ID, &req.UserID, &req.RequestRef, &req.Amount, &req.Currency,
            &req.Destination, &req.Service, &req.Status, &req.ReceiptCode,
            &req.JournalID, &req.Metadata, &req.ErrorMessage,
            &req.CreatedAt, &req.UpdatedAt, &req.CompletedAt,
        ); err != nil {
            return nil, 0, err
        }
        withdrawals = append(withdrawals, req)
    }
    
    return withdrawals, total, nil
}
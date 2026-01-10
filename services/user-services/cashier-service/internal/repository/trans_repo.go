package repository

import (
    "context"
    "encoding/json"
    "errors"
    
    "cashier-service/internal/domain"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
    db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepo {
    return &UserRepo{db: db}
}

// ============================================================================
// DEPOSIT METHODS
// ============================================================================

func (r *UserRepo) CreateDepositRequest(ctx context.Context, req *domain.DepositRequest) error {
    metaJSON, _ := json.Marshal(req.Metadata)
    
    query := `
        INSERT INTO deposit_requests 
        (user_id, partner_id, request_ref, amount, currency, service, agent_external_id, payment_method, 
         status, metadata, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id, created_at, updated_at
    `
    return r.db.QueryRow(ctx, query,
        req.UserID, req.PartnerID, req.RequestRef, req.Amount, req.Currency,
        req. Service, req.AgentExternalID, req.PaymentMethod, req.Status, metaJSON, req.ExpiresAt,
    ).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
}

func (r *UserRepo) GetDepositByID(ctx context. Context, id int64) (*domain.DepositRequest, error) {
    query := `
        SELECT 
            id, user_id, partner_id, request_ref, amount, currency, service, agent_external_id, payment_method,
            status, partner_transaction_ref, receipt_code, journal_id, metadata, error_message,
            expires_at, created_at, updated_at, completed_at
        FROM deposit_requests
        WHERE id = $1
    `
    return r.scanDepositRequest(r. db.QueryRow(ctx, query, id))
}

func (r *UserRepo) GetDepositByRef(ctx context.Context, requestRef string) (*domain.DepositRequest, error) {
    query := `
        SELECT 
            id, user_id, partner_id, request_ref, amount, currency, service, agent_external_id, payment_method,
            status, partner_transaction_ref, receipt_code, journal_id, metadata, error_message,
            expires_at, created_at, updated_at, completed_at
        FROM deposit_requests
        WHERE request_ref = $1
    `
    return r.scanDepositRequest(r.db.QueryRow(ctx, query, requestRef))
}

func (r *UserRepo) GetDepositByPartnerRef(ctx context.Context, partnerRef string) (*domain.DepositRequest, error) {
    query := `
        SELECT 
            id, user_id, partner_id, request_ref, amount, currency, service, agent_external_id, payment_method,
            status, partner_transaction_ref, receipt_code, journal_id, metadata, error_message,
            expires_at, created_at, updated_at, completed_at
        FROM deposit_requests
        WHERE partner_transaction_ref = $1
    `
    return r.scanDepositRequest(r. db.QueryRow(ctx, query, partnerRef))
}

func (r *UserRepo) UpdateDepositStatus(ctx context.Context, id int64, status string, errorMsg *string) error {
    query := `
        UPDATE deposit_requests 
        SET status = $1, error_message = $2, updated_at = NOW()
        WHERE id = $3
    `
    _, err := r.db.Exec(ctx, query, status, errorMsg, id)
    return err
}

func (r *UserRepo) UpdateDepositWithPartnerRef(ctx context.Context, id int64, partnerRef string, status string) error {
    query := `
        UPDATE deposit_requests 
        SET partner_transaction_ref = $1, status = $2, updated_at = NOW()
        WHERE id = $3
    `
    _, err := r.db.Exec(ctx, query, partnerRef, status, id)
    return err
}

func (r *UserRepo) UpdateDepositWithReceipt(ctx context.Context, id int64, receiptCode string, journalID int64) error {
    query := `
        UPDATE deposit_requests 
        SET 
            receipt_code = $1,
            journal_id = $2,
            status = $3,
            -- completed_at = NOW(),
            updated_at = NOW()
        WHERE id = $4
    `
    _, err := r.db.Exec(ctx, query, receiptCode, journalID, domain. DepositStatusCompleted, id)
    return err
}

// Mark deposit as failed (called from accounting service webhook/callback)
func (r *UserRepo) MarkDepositFailed(ctx context.Context, requestRef string, errorMsg string) error {
    query := `
        UPDATE deposit_requests 
        SET 
            status = $1,
            error_message = $2,
            updated_at = NOW()
        WHERE request_ref = $3
    `
    _, err := r.db.Exec(ctx, query, domain.DepositStatusFailed, errorMsg, requestRef)
    return err
}

// Mark deposit as completed (called from accounting service after successful credit)
func (r *UserRepo) MarkDepositCompleted(ctx context.Context, requestRef string, receiptCode string, journalID int64) error {
    query := `
        UPDATE deposit_requests 
        SET 
            receipt_code = $1,
            journal_id = $2,
            status = $3,
            completed_at = NOW(),
            updated_at = NOW()
        WHERE request_ref = $4
    `
    _, err := r.db.Exec(ctx, query, receiptCode, journalID, domain.DepositStatusCompleted, requestRef)
    return err
}

func (r *UserRepo) ListDeposits(ctx context.Context, userID int64, limit, offset int) ([]domain.DepositRequest, int64, error) {
    var total int64
    
    // Count query
    countQuery := `SELECT COUNT(*) FROM deposit_requests WHERE user_id = $1`
    if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
        return nil, 0, err
    }
    
    // Data query
    query := `
        SELECT 
            id, user_id, partner_id, request_ref, amount, currency, service, agent_external_id, payment_method,
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
    
    var deposits []domain.DepositRequest
    for rows.Next() {
        req, err := r.scanDepositRequestFromRows(rows)
        if err != nil {
            return nil, 0, err
        }
        deposits = append(deposits, *req)
    }
    
    return deposits, total, rows. Err()
}

// ✅ NEW:  ListAllDeposits - Admin list all deposits with optional status filter
func (r *UserRepo) ListAllDeposits(ctx context.Context, limit, offset int, status *string) ([]domain.DepositRequest, int64, error) {
    var total int64
    var args []interface{}
    argPos := 1
    
    // Build count query
    countQuery := `SELECT COUNT(*) FROM deposit_requests WHERE 1=1`
    if status != nil {
        countQuery += ` AND status = $` + string(rune(argPos+'0'))
        args = append(args, *status)
        argPos++
    }
    
    if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
        return nil, 0, err
    }
    
    // Build data query
    query := `
        SELECT 
            id, user_id, partner_id, request_ref, amount, currency, service, agent_external_id, payment_method,
            status, partner_transaction_ref, receipt_code, journal_id, metadata, error_message,
            expires_at, created_at, updated_at, completed_at
        FROM deposit_requests
        WHERE 1=1
    `
    
    if status != nil {
        query += ` AND status = $1`
    }
    
    query += ` ORDER BY created_at DESC LIMIT $` + string(rune(argPos+'0')) + ` OFFSET $` + string(rune(argPos+'1'))
    args = append(args, limit, offset)
    
    rows, err := r.db.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var deposits []domain.DepositRequest
    for rows.Next() {
        req, err := r. scanDepositRequestFromRows(rows)
        if err != nil {
            return nil, 0, err
        }
        deposits = append(deposits, *req)
    }
    
    return deposits, total, rows.Err()
}

// ============================================================================
// WITHDRAWAL METHODS
// ============================================================================

func (r *UserRepo) CreateWithdrawalRequest(ctx context.Context, req *domain.WithdrawalRequest) error {
    metaJSON, _ := json.Marshal(req.Metadata)
    
    query := `
        INSERT INTO withdrawal_requests 
        (user_id, request_ref, amount, currency, destination, service, agent_external_id, partner_id, status, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        RETURNING id, created_at, updated_at
    `
    return r.db.QueryRow(ctx, query,
        req. UserID, req.RequestRef, req.Amount, req.Currency,
        req.Destination, req.Service, req.AgentExternalID, req.PartnerID, req.Status, metaJSON,
    ).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
}

func (r *UserRepo) GetWithdrawalByID(ctx context.Context, id int64) (*domain.WithdrawalRequest, error) {
    query := `
        SELECT 
            id, user_id, request_ref, amount, currency, destination, service, agent_external_id, partner_id,
            status, receipt_code, journal_id, metadata, error_message,
            created_at, updated_at, completed_at
        FROM withdrawal_requests
        WHERE id = $1
    `
    return r.scanWithdrawalRequest(r. db.QueryRow(ctx, query, id))
}

func (r *UserRepo) GetWithdrawalByRef(ctx context. Context, requestRef string) (*domain.WithdrawalRequest, error) {
    query := `
        SELECT 
            id, user_id, request_ref, amount, currency, destination, service, agent_external_id, partner_id,
            status, receipt_code, journal_id, metadata, error_message,
            created_at, updated_at, completed_at
        FROM withdrawal_requests
        WHERE request_ref = $1
    `
    return r.scanWithdrawalRequest(r. db.QueryRow(ctx, query, requestRef))
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

func (r *UserRepo) UpdateWithdrawalWithReceipt(
    ctx context.Context,
    id int64,
    receiptCode string,
    journalID int64,
    completed bool,
) error {
    query := `
        UPDATE withdrawal_requests 
        SET 
            receipt_code = $1,
            journal_id = $2,
            status = $3,
            completed_at = CASE 
                WHEN $4 THEN COALESCE(completed_at, NOW())
                ELSE completed_at
            END,
            updated_at = NOW()
        WHERE id = $5
    `
    _, err := r.db.Exec(
        ctx,
        query,
        receiptCode,
        journalID,
        domain.WithdrawalStatusCompleted,
        completed,
        id,
    )
    return err
}


// Mark withdrawal as failed (called from accounting service)
func (r *UserRepo) MarkWithdrawalFailed(ctx context.Context, requestRef string, errorMsg string) error {
    query := `
        UPDATE withdrawal_requests 
        SET 
            status = $1,
            error_message = $2,
            updated_at = NOW()
        WHERE request_ref = $3
    `
    _, err := r.db.Exec(ctx, query, domain.WithdrawalStatusFailed, errorMsg, requestRef)
    return err
}

// Mark withdrawal as completed (called from accounting service after successful debit)
func (r *UserRepo) MarkWithdrawalCompleted(ctx context.Context, requestRef string, partnerTransRef string) error {
    query := `
        UPDATE withdrawal_requests 
        SET 
            status = $1,
            partner_transaction_ref = $2,
            completed_at = NOW(),
            updated_at = NOW()
        WHERE request_ref = $3
    `
    _, err := r.db. Exec(ctx, query, domain. WithdrawalStatusCompleted, partnerTransRef, requestRef)
    return err
}

func (r *UserRepo) ListWithdrawals(ctx context.Context, userID int64, limit, offset int) ([]domain.WithdrawalRequest, int64, error) {
    var total int64
    
    countQuery := `SELECT COUNT(*) FROM withdrawal_requests WHERE user_id = $1`
    if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
        return nil, 0, err
    }
    
    query := `
        SELECT 
            id, user_id, request_ref, amount, currency, destination, service, agent_external_id, partner_id,
            status, receipt_code, journal_id, metadata, error_message,
            created_at, updated_at, completed_at
        FROM withdrawal_requests
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
    
    rows, err := r.db. Query(ctx, query, userID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var withdrawals []domain.WithdrawalRequest
    for rows.Next() {
        req, err := r.scanWithdrawalRequestFromRows(rows)
        if err != nil {
            return nil, 0, err
        }
        withdrawals = append(withdrawals, *req)
    }
    
    return withdrawals, total, rows.Err()
}

// ✅ NEW: ListAllWithdrawals - Admin list all withdrawals with optional status filter
func (r *UserRepo) ListAllWithdrawals(ctx context.Context, limit, offset int, status *string) ([]domain.WithdrawalRequest, int64, error) {
    var total int64
    var args []interface{}
    argPos := 1
    
    // Build count query
    countQuery := `SELECT COUNT(*) FROM withdrawal_requests WHERE 1=1`
    if status != nil {
        countQuery += ` AND status = $` + string(rune(argPos+'0'))
        args = append(args, *status)
        argPos++
    }
    
    if err := r. db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
        return nil, 0, err
    }
    
    // Build data query
    query := `
        SELECT 
            id, user_id, request_ref, amount, currency, destination, service, agent_external_id, partner_id,
            status, receipt_code, journal_id, metadata, error_message,
            created_at, updated_at, completed_at
        FROM withdrawal_requests
        WHERE 1=1
    `
    
    if status != nil {
        query += ` AND status = $1`
    }
    
    query += ` ORDER BY created_at DESC LIMIT $` + string(rune(argPos+'0')) + ` OFFSET $` + string(rune(argPos+'1'))
    args = append(args, limit, offset)
    
    rows, err := r.db.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var withdrawals []domain.WithdrawalRequest
    for rows.Next() {
        req, err := r. scanWithdrawalRequestFromRows(rows)
        if err != nil {
            return nil, 0, err
        }
        withdrawals = append(withdrawals, *req)
    }
    
    return withdrawals, total, rows.Err()
}

// ============================================================================
// HELPER SCAN METHODS
// ============================================================================

func (r *UserRepo) scanDepositRequest(row pgx.Row) (*domain.DepositRequest, error) {
    var req domain.DepositRequest
    var metaJSON []byte
    
    err := row.Scan(
        &req.ID, &req.UserID, &req.PartnerID, &req.RequestRef, &req.Amount, &req.Currency,
        &req.Service, &req.AgentExternalID, &req.PaymentMethod, &req.Status, &req.PartnerTransactionRef,
        &req. ReceiptCode, &req. JournalID, &metaJSON, &req.ErrorMessage,
        &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt, &req.CompletedAt,
    )
    
    if err != nil {
        if errors.Is(err, pgx. ErrNoRows) {
            return nil, nil
        }
        return nil, err
    }
    
    if len(metaJSON) > 0 {
        json.Unmarshal(metaJSON, &req.Metadata)
    }
    
    return &req, nil
}

func (r *UserRepo) scanDepositRequestFromRows(rows pgx.Rows) (*domain.DepositRequest, error) {
    var req domain.DepositRequest
    var metaJSON []byte
    
    err := rows.Scan(
        &req.ID, &req. UserID, &req.PartnerID, &req.RequestRef, &req.Amount, &req.Currency,
        &req. Service, &req.AgentExternalID, &req.PaymentMethod, &req.Status, &req.PartnerTransactionRef,
        &req. ReceiptCode, &req.JournalID, &metaJSON, &req.ErrorMessage,
        &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt, &req.CompletedAt,
    )
    
    if err != nil {
        return nil, err
    }
    
    if len(metaJSON) > 0 {
        json.Unmarshal(metaJSON, &req.Metadata)
    }
    
    return &req, nil
}

func (r *UserRepo) scanWithdrawalRequest(row pgx.Row) (*domain.WithdrawalRequest, error) {
    var req domain.WithdrawalRequest
    var metaJSON []byte
    
    err := row.Scan(
        &req.ID, &req. UserID, &req.RequestRef, &req.Amount, &req.Currency,
        &req. Destination, &req.Service, &req.AgentExternalID, &req.PartnerID, &req.Status, &req. ReceiptCode,
        &req.JournalID, &metaJSON, &req.ErrorMessage,
        &req.CreatedAt, &req.UpdatedAt, &req.CompletedAt,
    )
    
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, nil
        }
        return nil, err
    }
    
    if len(metaJSON) > 0 {
        json.Unmarshal(metaJSON, &req.Metadata)
    }
    
    return &req, nil
}

func (r *UserRepo) scanWithdrawalRequestFromRows(rows pgx. Rows) (*domain.WithdrawalRequest, error) {
    var req domain.WithdrawalRequest
    var metaJSON []byte
    
    err := rows.Scan(
        &req.ID, &req.UserID, &req.RequestRef, &req.Amount, &req.Currency,
        &req.Destination, &req.Service, &req.AgentExternalID, &req.PartnerID, &req.Status, &req. ReceiptCode,
        &req.JournalID, &metaJSON, &req.ErrorMessage,
        &req.CreatedAt, &req.UpdatedAt, &req.CompletedAt,
    )
    
    if err != nil {
        return nil, err
    }
    
    if len(metaJSON) > 0 {
        json.Unmarshal(metaJSON, &req.Metadata)
    }
    
    return &req, nil
}
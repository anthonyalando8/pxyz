// internal/repository/payment_repository.go
package repository

import (
    "context"
    "encoding/json"
    //"fmt"

    "payment-service/internal/domain"
    
    "github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRepository interface {
    Create(ctx context.Context, payment *domain.Payment) error
    GetByPaymentRef(ctx context.Context, paymentRef string) (*domain.Payment, error)
    GetByPartnerTxRef(ctx context.Context, partnerID, partnerTxRef string) (*domain.Payment, error)
    UpdateStatus(ctx context.Context, id int64, status domain.PaymentStatus) error
    UpdateCallback(ctx context.Context, id int64, callbackData map[string]interface{}, providerRef *string) error
    MarkPartnerNotified(ctx context.Context, id int64) error
    IncrementRetry(ctx context.Context, id int64) error
    SetError(ctx context.Context, id int64, errorMsg string) error
}

type paymentRepo struct {
    db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) PaymentRepository {
    return &paymentRepo{db: db}
}

func (r *paymentRepo) Create(ctx context.Context, payment *domain.Payment) error {
    query := `
        INSERT INTO payments (
            payment_ref, partner_id, partner_tx_ref, provider, payment_type,
            amount, currency, user_id, account_number, phone_number,
            bank_account, status, description, metadata
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
        RETURNING id, created_at, updated_at
    `

    metadataJSON, _ := json.Marshal(payment. Metadata)

    return r.db.QueryRow(ctx, query,
        payment.PaymentRef,
        payment. PartnerID,
        payment. PartnerTxRef,
        payment.Provider,
        payment.PaymentType,
        payment.Amount,
        payment.Currency,
        payment.UserID,
        payment.AccountNumber,
        payment.PhoneNumber,
        payment.BankAccount,
        payment.Status,
        payment.Description,
        metadataJSON,
    ).Scan(&payment.ID, &payment.CreatedAt, &payment. UpdatedAt)
}

func (r *paymentRepo) GetByPaymentRef(ctx context. Context, paymentRef string) (*domain.Payment, error) {
    query := `
        SELECT 
            id, payment_ref, partner_id, partner_tx_ref, provider, payment_type,
            amount, currency, user_id, account_number, phone_number, bank_account,
            status, provider_reference, description, metadata,
            callback_received, callback_data, callback_at,
            partner_notified, partner_notification_attempts, partner_notified_at,
            error_message, retry_count, created_at, updated_at, completed_at
        FROM payments
        WHERE payment_ref = $1
    `

    var payment domain.Payment
    err := r.db.QueryRow(ctx, query, paymentRef).Scan(
        &payment.ID,
        &payment.PaymentRef,
        &payment. PartnerID,
        &payment.PartnerTxRef,
        &payment.Provider,
        &payment.PaymentType,
        &payment.Amount,
        &payment.Currency,
        &payment.UserID,
        &payment.AccountNumber,
        &payment.PhoneNumber,
        &payment.BankAccount,
        &payment.Status,
        &payment.ProviderReference,
        &payment.Description,
        &payment.Metadata,
        &payment.CallbackReceived,
        &payment.CallbackData,
        &payment. CallbackAt,
        &payment.PartnerNotified,
        &payment.PartnerNotificationAttempts,
        &payment.PartnerNotifiedAt,
        &payment.ErrorMessage,
        &payment.RetryCount,
        &payment.CreatedAt,
        &payment. UpdatedAt,
        &payment.CompletedAt,
    )

    if err != nil {
        return nil, err
    }

    return &payment, nil
}

func (r *paymentRepo) GetByPartnerTxRef(ctx context. Context, partnerID, partnerTxRef string) (*domain.Payment, error) {
    query := `
        SELECT 
            id, payment_ref, partner_id, partner_tx_ref, provider, payment_type,
            amount, currency, user_id, account_number, phone_number, bank_account,
            status, provider_reference, description, metadata,
            callback_received, callback_data, callback_at,
            partner_notified, partner_notification_attempts, partner_notified_at,
            error_message, retry_count, created_at, updated_at, completed_at
        FROM payments
        WHERE partner_id = $1 AND partner_tx_ref = $2
    `

    var payment domain.Payment
    err := r.db.QueryRow(ctx, query, partnerID, partnerTxRef).Scan(
        &payment. ID,
        &payment.PaymentRef,
        &payment. PartnerID,
        &payment.PartnerTxRef,
        &payment.Provider,
        &payment.PaymentType,
        &payment.Amount,
        &payment.Currency,
        &payment.UserID,
        &payment.AccountNumber,
        &payment.PhoneNumber,
        &payment.BankAccount,
        &payment.Status,
        &payment.ProviderReference,
        &payment.Description,
        &payment.Metadata,
        &payment.CallbackReceived,
        &payment. CallbackData,
        &payment.CallbackAt,
        &payment.PartnerNotified,
        &payment.PartnerNotificationAttempts,
        &payment.PartnerNotifiedAt,
        &payment.ErrorMessage,
        &payment.RetryCount,
        &payment.CreatedAt,
        &payment.UpdatedAt,
        &payment. CompletedAt,
    )

    if err != nil {
        return nil, err
    }

    return &payment, nil
}

func (r *paymentRepo) UpdateStatus(ctx context. Context, id int64, status domain.PaymentStatus) error {
    query := `
        UPDATE payments
        SET 
            status = $1,
            completed_at = CASE WHEN $1:: text IN ('completed', 'failed', 'cancelled') THEN NOW() ELSE completed_at END,
            updated_at = NOW()
        WHERE id = $2
    `

    _, err := r.db.Exec(ctx, query, status, id)
    return err
}

func (r *paymentRepo) UpdateCallback(ctx context.Context, id int64, callbackData map[string]interface{}, providerRef *string) error {
    query := `
        UPDATE payments
        SET 
            callback_received = TRUE,
            callback_data = $1,
            callback_at = NOW(),
            provider_reference = COALESCE($2, provider_reference),
            updated_at = NOW()
        WHERE id = $3
    `

    callbackJSON, _ := json.Marshal(callbackData)
    _, err := r.db.Exec(ctx, query, callbackJSON, providerRef, id)
    return err
}

func (r *paymentRepo) MarkPartnerNotified(ctx context. Context, id int64) error {
    query := `
        UPDATE payments
        SET 
            partner_notified = TRUE,
            partner_notification_attempts = partner_notification_attempts + 1,
            partner_notified_at = NOW(),
            updated_at = NOW()
        WHERE id = $1
    `

    _, err := r.db.Exec(ctx, query, id)
    return err
}

func (r *paymentRepo) IncrementRetry(ctx context.Context, id int64) error {
    query := `
        UPDATE payments
        SET 
            retry_count = retry_count + 1,
            updated_at = NOW()
        WHERE id = $1
    `

    _, err := r.db.Exec(ctx, query, id)
    return err
}

func (r *paymentRepo) SetError(ctx context.Context, id int64, errorMsg string) error {
    query := `
        UPDATE payments
        SET 
            error_message = $1,
            status = 'failed',
            updated_at = NOW()
        WHERE id = $2
    `

    _, err := r.db. Exec(ctx, query, errorMsg, id)
    return err
}
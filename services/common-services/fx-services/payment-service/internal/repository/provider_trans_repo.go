// internal/repository/provider_transaction_repository.go
package repository

import (
    "context"
    "encoding/json"

    "payment-service/internal/domain"
    
    "github.com/jackc/pgx/v5/pgxpool"
)

type ProviderTransactionRepository interface {
    Create(ctx context.Context, tx *domain.ProviderTransaction) error
    GetByCheckoutRequestID(ctx context.Context, checkoutRequestID string) (*domain.ProviderTransaction, error)
    GetByProviderTxID(ctx context.Context, providerTxID string) (*domain.ProviderTransaction, error)
    UpdateStatus(ctx context.Context, id int64, status domain.TransactionStatus, resultCode, resultDesc string) error
    UpdateResponse(ctx context.Context, id int64, responsePayload map[string]interface{}) error
}

type providerTransactionRepo struct {
    db *pgxpool.Pool
}

func NewProviderTransactionRepository(db *pgxpool.Pool) ProviderTransactionRepository {
    return &providerTransactionRepo{db:  db}
}

func (r *providerTransactionRepo) Create(ctx context.Context, tx *domain.ProviderTransaction) error {
    query := `
        INSERT INTO provider_transactions (
            payment_id, provider, transaction_type, request_payload,
            response_payload, provider_tx_id, checkout_request_id, status
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, created_at, updated_at
    `

    requestJSON, _ := json.Marshal(tx.RequestPayload)
    responseJSON, _ := json.Marshal(tx.ResponsePayload)

    return r.db.QueryRow(ctx, query,
        tx. PaymentID,
        tx. Provider,
        tx.TransactionType,
        requestJSON,
        responseJSON,
        tx.ProviderTxID,
        tx.CheckoutRequestID,
        tx.Status,
    ).Scan(&tx.ID, &tx.CreatedAt, &tx.UpdatedAt)
}

func (r *providerTransactionRepo) GetByCheckoutRequestID(ctx context. Context, checkoutRequestID string) (*domain.ProviderTransaction, error) {
    query := `
        SELECT 
            id, payment_id, provider, transaction_type, request_payload,
            response_payload, provider_tx_id, checkout_request_id, status,
            result_code, result_description, created_at, updated_at, completed_at
        FROM provider_transactions
        WHERE checkout_request_id = $1
    `

    var tx domain.ProviderTransaction
    err := r.db.QueryRow(ctx, query, checkoutRequestID).Scan(
        &tx.ID,
        &tx.PaymentID,
        &tx.Provider,
        &tx.TransactionType,
        &tx.RequestPayload,
        &tx.ResponsePayload,
        &tx. ProviderTxID,
        &tx.CheckoutRequestID,
        &tx.Status,
        &tx.ResultCode,
        &tx.ResultDescription,
        &tx.CreatedAt,
        &tx.UpdatedAt,
        &tx.CompletedAt,
    )

    if err != nil {
        return nil, err
    }

    return &tx, nil
}

func (r *providerTransactionRepo) GetByProviderTxID(ctx context.Context, providerTxID string) (*domain.ProviderTransaction, error) {
    query := `
        SELECT 
            id, payment_id, provider, transaction_type, request_payload,
            response_payload, provider_tx_id, checkout_request_id, status,
            result_code, result_description, created_at, updated_at, completed_at
        FROM provider_transactions
        WHERE provider_tx_id = $1
    `

    var tx domain.ProviderTransaction
    err := r.db.QueryRow(ctx, query, providerTxID).Scan(
        &tx. ID,
        &tx.PaymentID,
        &tx. Provider,
        &tx.TransactionType,
        &tx. RequestPayload,
        &tx.ResponsePayload,
        &tx.ProviderTxID,
        &tx.CheckoutRequestID,
        &tx. Status,
        &tx.ResultCode,
        &tx.ResultDescription,
        &tx.CreatedAt,
        &tx. UpdatedAt,
        &tx.CompletedAt,
    )

    if err != nil {
        return nil, err
    }

    return &tx, nil
}

func (r *providerTransactionRepo) UpdateStatus(ctx context.Context, id int64, status domain.TransactionStatus, resultCode, resultDesc string) error {
    query := `
        UPDATE provider_transactions
        SET 
            status = $1,
            result_code = $2,
            result_description = $3,
            completed_at = CASE WHEN $1 IN ('completed', 'failed') THEN NOW() ELSE completed_at END,
            updated_at = NOW()
        WHERE id = $4
    `

    _, err := r.db.Exec(ctx, query, status, resultCode, resultDesc, id)
    return err
}

func (r *providerTransactionRepo) UpdateResponse(ctx context.Context, id int64, responsePayload map[string]interface{}) error {
    query := `
        UPDATE provider_transactions
        SET 
            response_payload = $1,
            updated_at = NOW()
        WHERE id = $2
    `

    responseJSON, _ := json.Marshal(responsePayload)
    _, err := r.db.Exec(ctx, query, responseJSON, id)
    return err
}
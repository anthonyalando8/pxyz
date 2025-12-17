// internal/domain/payment.go
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type PaymentProvider string
type PaymentType string
type PaymentStatus string

const (
    ProviderMpesa  PaymentProvider = "mpesa"
    ProviderBank   PaymentProvider = "bank"
    ProviderCard   PaymentProvider = "card"
    ProviderPayPal PaymentProvider = "paypal"
)

const (
    PaymentTypeDeposit    PaymentType = "deposit"
    PaymentTypeWithdrawal PaymentType = "withdrawal"
)

const (
    PaymentStatusPending    PaymentStatus = "pending"
    PaymentStatusProcessing PaymentStatus = "processing"
    PaymentStatusCompleted  PaymentStatus = "completed"
    PaymentStatusFailed     PaymentStatus = "failed"
    PaymentStatusCancelled  PaymentStatus = "cancelled"
)

type TransactionStatus string

const (
    TxStatusInitiated        TransactionStatus = "initiated"
    TxStatusSent             TransactionStatus = "sent"
    TxStatusCallbackReceived TransactionStatus = "callback_received"
    TxStatusVerified         TransactionStatus = "verified"
    TxStatusCompleted        TransactionStatus = "completed"
    TxStatusFailed           TransactionStatus = "failed"
)

// Payment represents a payment transaction
type Payment struct {
    ID                          int64           `json:"id" db:"id"`
    PaymentRef                  string          `json:"payment_ref" db:"payment_ref"`
    PartnerID                   string          `json:"partner_id" db:"partner_id"`
    PartnerTxRef                string          `json:"partner_tx_ref" db:"partner_tx_ref"`
    
    Provider                    PaymentProvider `json:"provider" db:"provider"`
    PaymentType                 PaymentType     `json:"payment_type" db:"payment_type"`
    Amount                      float64         `json:"amount" db:"amount"`
    Currency                    string          `json:"currency" db:"currency"`
    
    UserID                      string          `json:"user_id" db:"user_id"`
    AccountNumber               *string         `json:"account_number,omitempty" db:"account_number"`
    PhoneNumber                 *string         `json:"phone_number,omitempty" db:"phone_number"`
    BankAccount                 *string         `json:"bank_account,omitempty" db:"bank_account"`
    
    Status                      PaymentStatus   `json:"status" db:"status"`
    ProviderReference           *string         `json:"provider_reference,omitempty" db:"provider_reference"`
    
    Description                 *string         `json:"description,omitempty" db:"description"`
    Metadata                    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
    
    CallbackReceived            bool            `json:"callback_received" db:"callback_received"`
    CallbackData                json.RawMessage `json:"callback_data,omitempty" db:"callback_data"`
    CallbackAt                  *time.Time      `json:"callback_at,omitempty" db:"callback_at"`
    
    PartnerNotified             bool            `json:"partner_notified" db:"partner_notified"`
    PartnerNotificationAttempts int             `json:"partner_notification_attempts" db:"partner_notification_attempts"`
    PartnerNotifiedAt           *time.Time      `json:"partner_notified_at,omitempty" db:"partner_notified_at"`
    
    ErrorMessage                *string         `json:"error_message,omitempty" db:"error_message"`
    RetryCount                  int             `json:"retry_count" db:"retry_count"`
    
    CreatedAt                   time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt                   time.Time       `json:"updated_at" db:"updated_at"`
    CompletedAt                 *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
}

// ProviderTransaction represents interaction with payment provider
type ProviderTransaction struct {
    ID                int64             `json:"id" db:"id"`
    PaymentID         int64             `json:"payment_id" db:"payment_id"`
    Provider          PaymentProvider   `json:"provider" db:"provider"`
    TransactionType   string            `json:"transaction_type" db:"transaction_type"`
    
    RequestPayload    json.RawMessage   `json:"request_payload" db:"request_payload"`
    ResponsePayload   json.RawMessage   `json:"response_payload,omitempty" db:"response_payload"`
    ProviderTxID      *string           `json:"provider_tx_id,omitempty" db:"provider_tx_id"`
    CheckoutRequestID *string           `json:"checkout_request_id,omitempty" db:"checkout_request_id"`
    
    Status            TransactionStatus `json:"status" db:"status"`
    ResultCode        *string           `json:"result_code,omitempty" db:"result_code"`
    ResultDescription *string           `json:"result_description,omitempty" db:"result_description"`
    
    CreatedAt         time.Time         `json:"created_at" db:"created_at"`
    UpdatedAt         time.Time         `json:"updated_at" db:"updated_at"`
    CompletedAt       *time.Time        `json:"completed_at,omitempty" db:"completed_at"`
}

// DepositRequest represents incoming deposit webhook from partner
// internal/domain/payment.go

// Add webhook metadata structure
type DepositWebhookMetadata struct {
    RequestRef        string  `json:"request_ref"`
    OriginalAmount    float64 `json:"original_amount"`
    ConvertedAmount   float64 `json:"converted_amount"`
    OriginalCurrency  string  `json:"original_currency"`
    TargetCurrency    string  `json:"target_currency"`
    ExchangeRate      float64 `json:"exchange_rate"`
    AccountNumber     string  `json:"account_number,omitempty"`
}

// DepositRequest updated to include parsed metadata
type DepositRequest struct {
    TransactionRef string                 `json:"transaction_ref"`
    PartnerID      string                 `json:"partner_id"`
    Provider       PaymentProvider        `json:"provider"`
    UserID         string                 `json:"user_id"`
    AccountNumber  string                 `json:"account_number"`
    PhoneNumber    string                 `json:"phone_number"`
    Amount         float64                `json:"amount"`          // USD amount (converted)
    Currency       string                 `json:"currency"`        // Target currency (USD)
    PaymentMethod  string                 `json:"payment_method"`
    Description    string                 `json:"description"`
    Metadata       map[string]interface{} `json:"metadata"`
    
    // Parsed metadata fields
    ParsedMetadata *DepositWebhookMetadata `json:"-"`
}

func (r *DepositRequest) Validate() error {
    if r.TransactionRef == "" {
        return errors.New("transaction_ref is required")
    }
    if r. PartnerID == "" {
        return errors.New("partner_id is required")
    }
    if r.UserID == "" {
        return errors.New("user_id is required")
    }
    if r.Amount <= 0 {
        return errors.New("amount must be greater than 0")
    }
    if r.Provider == ProviderMpesa && r. PhoneNumber == "" {
        return errors.New("phone_number is required for M-Pesa")
    }
    if r.Currency == "" {
        r.Currency = "USD"
    }
    
    // Parse and validate metadata
    if err := r.parseMetadata(); err != nil {
        return fmt.Errorf("invalid metadata: %w", err)
    }
    
    return nil
}

func (r *DepositRequest) parseMetadata() error {
    if r.Metadata == nil {
        return errors.New("metadata is required")
    }

    r.ParsedMetadata = &DepositWebhookMetadata{}

    // Extract required fields
    if val, ok := r.Metadata["original_amount"].(string); ok {
        if amt, err := strconv.ParseFloat(val, 64); err == nil {
            r.ParsedMetadata.OriginalAmount = amt
        }
    } else if val, ok := r.Metadata["original_amount"].(float64); ok {
        r.ParsedMetadata. OriginalAmount = val
    }

    if val, ok := r.Metadata["converted_amount"].(string); ok {
        if amt, err := strconv. ParseFloat(val, 64); err == nil {
            r. ParsedMetadata.ConvertedAmount = amt
        }
    } else if val, ok := r.Metadata["converted_amount"].(float64); ok {
        r.ParsedMetadata.ConvertedAmount = val
    }

    if val, ok := r.Metadata["original_currency"].(string); ok {
        r.ParsedMetadata.OriginalCurrency = val
    }

    if val, ok := r.Metadata["target_currency"].(string); ok {
        r.ParsedMetadata.TargetCurrency = val
    }

    if val, ok := r.Metadata["exchange_rate"].(string); ok {
        if rate, err := strconv.ParseFloat(val, 64); err == nil {
            r.ParsedMetadata.ExchangeRate = rate
        }
    } else if val, ok := r. Metadata["exchange_rate"].(float64); ok {
        r.ParsedMetadata.ExchangeRate = val
    }

    if val, ok := r. Metadata["request_ref"].(string); ok {
        r.ParsedMetadata.RequestRef = val
    }

    if val, ok := r.Metadata["account_number"].(string); ok {
        r.ParsedMetadata.AccountNumber = val
    }

    // Validate parsed metadata
    if r.ParsedMetadata.OriginalAmount <= 0 {
        return errors.New("original_amount is required in metadata")
    }
    if r.ParsedMetadata. OriginalCurrency == "" {
        return errors.New("original_currency is required in metadata")
    }

    return nil
}
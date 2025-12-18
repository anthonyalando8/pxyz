package domain
import (
	"errors"
	"fmt"
	"strconv"
)

// internal/domain/payment. go

// Add withdrawal request
type WithdrawalRequest struct {
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
    ParsedMetadata *WithdrawalWebhookMetadata `json:"-"`
}

// WithdrawalWebhookMetadata represents parsed metadata from webhook
type WithdrawalWebhookMetadata struct {
    RequestRef        string  `json:"request_ref"`
    OriginalAmount    float64 `json:"original_amount"`
    ConvertedAmount   float64 `json:"converted_amount"`
    OriginalCurrency  string  `json:"original_currency"`
    TargetCurrency    string  `json:"target_currency"`
    ExchangeRate      float64 `json:"exchange_rate"`
    AccountNumber     string  `json:"account_number,omitempty"`
}

func (r *WithdrawalRequest) Validate() error {
    if r.TransactionRef == "" {
        return errors.New("transaction_ref is required")
    }
    if r.PartnerID == "" {
        return errors. New("partner_id is required")
    }
    if r. UserID == "" {
        return errors.New("user_id is required")
    }
    if r.Amount <= 0 {
        return errors.New("amount must be greater than 0")
    }
    if r.Provider == ProviderMpesa && r.PhoneNumber == "" {
        return errors.New("phone_number is required for M-Pesa")
    }
    if r. Currency == "" {
        r.Currency = "USD"
    }
    
    // Parse and validate metadata
    if err := r.parseMetadata(); err != nil {
        return fmt.Errorf("invalid metadata: %w", err)
    }
    
    return nil
}

func (r *WithdrawalRequest) parseMetadata() error {
    if r.Metadata == nil {
        return errors.New("metadata is required")
    }

    r.ParsedMetadata = &WithdrawalWebhookMetadata{}

    // Extract required fields (same logic as deposit)
    if val, ok := r.Metadata["original_amount"].(string); ok {
        if amt, err := strconv.ParseFloat(val, 64); err == nil {
            r.ParsedMetadata.OriginalAmount = amt
        }
    } else if val, ok := r. Metadata["original_amount"].(float64); ok {
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
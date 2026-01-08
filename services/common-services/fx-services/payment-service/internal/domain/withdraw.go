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
    OriginalAmount    string `json:"original_amount"`
    ConvertedAmount   string `json:"converted_amount"`
    OriginalCurrency  string  `json:"original_currency"`
    TargetCurrency    string  `json:"target_currency"`
    ExchangeRate      string `json:"exchange_rate"`
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

    // ✅ Extract original_amount (as string)
    if val, ok := r.Metadata["original_amount"].(string); ok {
        r.ParsedMetadata.OriginalAmount = val
    } else if val, ok := r. Metadata["original_amount"].(float64); ok {
        // Handle if sent as number
        r.ParsedMetadata. OriginalAmount = fmt. Sprintf("%.2f", val)
    }

    // ✅ Extract converted_amount (as string)
    if val, ok := r.Metadata["converted_amount"].(string); ok {
        r.ParsedMetadata. ConvertedAmount = val
    } else if val, ok := r.Metadata["converted_amount"].(float64); ok {
        r.ParsedMetadata.ConvertedAmount = fmt.Sprintf("%.2f", val)
    }

    // ✅ Extract original_currency (as string)
    if val, ok := r.Metadata["original_currency"].(string); ok {
        r.ParsedMetadata.OriginalCurrency = val
    }

    // ✅ Extract target_currency (as string)
    if val, ok := r. Metadata["target_currency"].(string); ok {
        r.ParsedMetadata.TargetCurrency = val
    }

    // ✅ Extract exchange_rate (as string)
    if val, ok := r. Metadata["exchange_rate"].(string); ok {
        r.ParsedMetadata.ExchangeRate = val
    } else if val, ok := r. Metadata["exchange_rate"].(float64); ok {
        r.ParsedMetadata.ExchangeRate = fmt.Sprintf("%.4f", val)
    }

    // ✅ Extract request_ref (as string)
    if val, ok := r. Metadata["request_ref"].(string); ok {
        r.ParsedMetadata.RequestRef = val
    }

    // ✅ Extract account_number (as string, optional)
    if val, ok := r.Metadata["account_number"].(string); ok {
        r.ParsedMetadata.AccountNumber = val
    }

    // ✅ Extract phone_number (as string, optional) - ADD THIS
    if val, ok := r.Metadata["phone_number"].(string); ok {
        r.PhoneNumber = val // Set it on the main request
    }

    // ✅ Validate parsed metadata (parse to float for validation)
    if r.ParsedMetadata.OriginalAmount == "" {
        return errors.New("original_amount is required in metadata")
    }

    // Validate it's a valid number
    if amount, err := strconv.ParseFloat(r.ParsedMetadata. OriginalAmount, 64); err != nil || amount <= 0 {
        return fmt.Errorf("invalid original_amount in metadata: %s", r.ParsedMetadata. OriginalAmount)
    }

    if r.ParsedMetadata.OriginalCurrency == "" {
        return errors.New("original_currency is required in metadata")
    }

    return nil
}
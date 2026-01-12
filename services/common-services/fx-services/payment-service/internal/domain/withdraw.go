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

    BankAccount      string    `json:"bank_account,omitempty"`      // Format: "bank_name,account_number"
	BankName         string    `json:"bank_name,omitempty"`         // Extracted bank name
	BankAccountNum   string    `json:"bank_account_num,omitempty"`  // Extracted account number
	BankInfo         *BankInfo `json:"bank_info,omitempty"`         // Full bank info
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

// Update parseMetadata to extract bank info
func (r *WithdrawalRequest) parseMetadata() error {
	if r.Metadata == nil {
		return errors.New("metadata is required")
	}

	r.ParsedMetadata = &WithdrawalWebhookMetadata{}

	// Extract original_amount
	if val, ok := r.Metadata["original_amount"].(string); ok {
		r.ParsedMetadata.OriginalAmount = val
	} else if val, ok := r.Metadata["original_amount"].(float64); ok {
		r.ParsedMetadata. OriginalAmount = fmt. Sprintf("%.2f", val)
	}

	// Extract converted_amount
	if val, ok := r.Metadata["converted_amount"].(string); ok {
		r.ParsedMetadata. ConvertedAmount = val
	} else if val, ok := r.Metadata["converted_amount"].(float64); ok {
		r.ParsedMetadata.ConvertedAmount = fmt.Sprintf("%.2f", val)
	}

	// Extract original_currency
	if val, ok := r.Metadata["original_currency"].(string); ok {
		r.ParsedMetadata. OriginalCurrency = val
	}

	// Extract target_currency
	if val, ok := r.Metadata["target_currency"].(string); ok {
		r.ParsedMetadata.TargetCurrency = val
	}

	// Extract exchange_rate
	if val, ok := r. Metadata["exchange_rate"].(string); ok {
		r.ParsedMetadata.ExchangeRate = val
	} else if val, ok := r.Metadata["exchange_rate"].(float64); ok {
		r.ParsedMetadata.ExchangeRate = fmt.Sprintf("%.4f", val)
	}

	// Extract request_ref
	if val, ok := r. Metadata["request_ref"].(string); ok {
		r.ParsedMetadata.RequestRef = val
	}

	// Extract account_number (legacy)
	if val, ok := r.Metadata["account_number"].(string); ok {
		r.ParsedMetadata.AccountNumber = val
	}

	// ✅ Extract bank_account (NEW)
	if val, ok := r.Metadata["bank_account"].(string); ok {
		r.ParsedMetadata.BankAccount = val
		
		// Parse bank_account format:  "bank_name,account_number"
		bankName, accountNum, err := ValidateBankAccount(val)
		if err != nil {
			return fmt. Errorf("invalid bank_account:  %w", err)
		}
		
		r.ParsedMetadata. BankName = bankName
		r.ParsedMetadata.BankAccountNum = accountNum
		
		// Get full bank info
		bankInfo, err := GetBankByName(bankName)
		if err != nil {
			return fmt.Errorf("bank not found: %w", err)
		}
		
		r.ParsedMetadata.BankInfo = bankInfo
	}

	// Extract phone_number
	if val, ok := r. Metadata["phone_number"].(string); ok {
		r.PhoneNumber = val
	}

	// Validate parsed metadata
	if r.ParsedMetadata.OriginalAmount == "" {
		return errors.New("original_amount is required in metadata")
	}

	// Validate it's a valid number
	if amount, err := strconv.ParseFloat(r.ParsedMetadata. OriginalAmount, 64); err != nil || amount <= 0 {
		return fmt. Errorf("invalid original_amount in metadata: %s", r.ParsedMetadata.OriginalAmount)
	}

	if r.ParsedMetadata.OriginalCurrency == "" {
		return errors.New("original_currency is required in metadata")
	}

	return nil
}

// ✅ Helper to check if withdrawal is bank transfer
func (r *WithdrawalRequest) IsBankTransfer() bool {
	return r.ParsedMetadata != nil && r.ParsedMetadata.BankInfo != nil
}

// ✅ Helper to get bank paybill
func (r *WithdrawalRequest) GetBankPaybill() string {
	if r.ParsedMetadata != nil && r.ParsedMetadata.BankInfo != nil {
		return r.ParsedMetadata.BankInfo.PaybillNumber
	}
	return ""
}

// ✅ Helper to get bank account number
func (r *WithdrawalRequest) GetBankAccountNumber() string {
	if r.ParsedMetadata != nil {
		return r.ParsedMetadata.BankAccountNum
	}
	return ""
}
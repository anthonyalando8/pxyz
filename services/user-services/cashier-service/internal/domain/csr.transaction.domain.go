// domain/transaction.go
package domain

import "time"

type DepositRequest struct {
	ID                    int64                  `json:"id"`
	UserID                int64                  `json:"user_id"`
	PartnerID             string                 `json:"partner_id"`
	RequestRef            string                 `json:"request_ref"`
	Amount                float64                `json:"amount"`   //  Converted amount (USD)
	Currency              string                 `json:"currency"` //  Target currency (USD)
	Service               string                 `json:"service"`
	AgentExternalID       *string                `json:"agent_external_id,omitempty"` //  NEW
	PaymentMethod         *string                `json:"payment_method,omitempty"`
	Status                string                 `json:"status"`
	PartnerTransactionRef *string                `json:"partner_transaction_ref,omitempty"`
	ReceiptCode           *string                `json:"receipt_code,omitempty"`
	JournalID             *int64                 `json:"journal_id,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"` //  Store original amount/currency here
	ErrorMessage          *string                `json:"error_message,omitempty"`
	ExpiresAt             time.Time              `json:"expires_at"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	CompletedAt           *time.Time             `json:"completed_at,omitempty"`

	// Deprecated fields (keep for backward compatibility)
	Provider   string `json:"provider,omitempty"`
	Phone      string `json:"phone,omitempty"`
	AccountRef string `json:"account_ref,omitempty"`
}

type WithdrawalRequest struct {
	ID                    int64                  `json:"id"`
	UserID                int64                  `json:"user_id"`
	RequestRef            string                 `json:"request_ref"`
	Amount                float64                `json:"amount"`   //  Converted amount (USD)
	Currency              string                 `json:"currency"` //  Account currency (USD)
	Destination           string                 `json:"destination"`
	Service               *string                `json:"service,omitempty"`
	AgentExternalID       *string                `json:"agent_external_id,omitempty"`
	PartnerID             *string                `json:"partner_id,omitempty"`              //  NEW
	PartnerTransactionRef *string                `json:"partner_transaction_ref,omitempty"` //  NEW
	Status                string                 `json:"status"`
	ReceiptCode           *string                `json:"receipt_code,omitempty"`
	JournalID             *int64                 `json:"journal_id,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"` //  Store original amount/currency here
	ErrorMessage          *string                `json:"error_message,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	CompletedAt           *time.Time             `json:"completed_at,omitempty"`
}

// Helper functions to work with metadata
func (d *DepositRequest) SetOriginalAmount(amount float64, currency string, rate float64) {
	if d.Metadata == nil {
		d.Metadata = make(map[string]interface{})
	}
	d.Metadata["original_amount"] = amount
	d.Metadata["original_currency"] = currency
	d.Metadata["exchange_rate"] = rate
}

func (d *DepositRequest) GetOriginalAmount() (amount float64, currency string, rate float64, ok bool) {
	if d.Metadata == nil {
		return 0, "", 0, false
	}
	amount, ok1 := d.Metadata["original_amount"].(float64)
	currency, ok2 := d.Metadata["original_currency"].(string)
	rate, ok3 := d.Metadata["exchange_rate"].(float64)
	return amount, currency, rate, ok1 && ok2 && ok3
}

func (w *WithdrawalRequest) SetOriginalAmount(amount float64, currency string, rate float64) {
	if w.Metadata == nil {
		w.Metadata = make(map[string]interface{})
	}
	w.Metadata["original_amount"] = amount
	w.Metadata["original_currency"] = currency
	w.Metadata["exchange_rate"] = rate
}

func (w *WithdrawalRequest) GetOriginalAmount() (amount float64, currency string, rate float64, ok bool) {
	if w.Metadata == nil {
		return 0, "", 0, false
	}
	amount, ok1 := w.Metadata["original_amount"].(float64)
	currency, ok2 := w.Metadata["original_currency"].(string)
	rate, ok3 := w.Metadata["exchange_rate"].(float64)
	return amount, currency, rate, ok1 && ok2 && ok3
}

// Status constants (existing)
const (
	// Deposit statuses
	DepositStatusPending       = "pending"
	DepositStatusSentToPartner = "sent_to_partner"
	DepositStatusSentToAgent   = "sent_to_agent" //  NEW
	DepositStatusProcessing    = "processing"
	DepositStatusCompleted     = "completed"
	DepositStatusFailed        = "failed"
	DepositStatusCancelled     = "cancelled"

	// Withdrawal statuses
	WithdrawalStatusPending       = "pending"
	WithdrawalStatusSentToPartner = "sent_to_partner"
	WithdrawalStatusProcessing    = "processing"
	WithdrawalStatusCompleted     = "completed"
	WithdrawalStatusFailed        = "failed"
	WithdrawalStatusCancelled     = "cancelled"
)

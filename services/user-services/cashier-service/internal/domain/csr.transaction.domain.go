package domain
// domain/deposit.go
import "time"

type DepositRequest struct {
    ID                   int64                  `json:"id"`
    UserID               int64                  `json:"user_id"`
    PartnerID            string                 `json:"partner_id"`
    RequestRef           string                 `json:"request_ref"`
    Amount               float64                `json:"amount"`
    Currency             string                 `json:"currency"`
	Provider   string `json:"provider"` // e.g., "mpesa"
	Phone      string `json:"phone"`
    AccountRef string `json:"account_ref"`
    Service              string                 `json:"service"`
    PaymentMethod        string                 `json:"payment_method,omitempty"`
    Status               string                 `json:"status"`
    PartnerTransactionRef *string               `json:"partner_transaction_ref,omitempty"`
    ReceiptCode          *string                `json:"receipt_code,omitempty"`
    JournalID            *int64                 `json:"journal_id,omitempty"`
    Metadata             map[string]interface{} `json:"metadata,omitempty"`
    ErrorMessage         *string                `json:"error_message,omitempty"`
    ExpiresAt            time.Time              `json:"expires_at"`
    CreatedAt            time.Time              `json:"created_at"`
    UpdatedAt            time. Time              `json:"updated_at"`
    CompletedAt          *time.Time             `json:"completed_at,omitempty"`
}

type WithdrawalRequest struct {
    ID           int64                  `json:"id"`
    UserID       int64                  `json:"user_id"`
    RequestRef   string                 `json:"request_ref"`
    Amount       float64                `json:"amount"`
    Currency     string                 `json:"currency"`
    Destination  string                 `json:"destination"`
    Service      string                 `json:"service"`
    Status       string                 `json:"status"`
    ReceiptCode  *string                `json:"receipt_code,omitempty"`
    JournalID    *int64                 `json:"journal_id,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
    ErrorMessage *string                `json:"error_message,omitempty"`
    CreatedAt    time.Time              `json:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at"`
    CompletedAt  *time.Time             `json:"completed_at,omitempty"`
}
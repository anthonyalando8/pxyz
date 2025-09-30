package domain

import "time"

// Posting represents a single ledger line item (DR/CR)
type Posting struct {
    ID        int64     `json:"id"`
    JournalID int64     `json:"journal_id"`
    AccountID int64     `json:"account_id"`
    Amount    float64   `json:"amount"`
    DrCr      string    `json:"dr_cr"`    // DR or CR
    Currency  string    `json:"currency"`
    ReceiptCode *string    `json:"receipt_id,omitempty"` // optional link to receipt
    CreatedAt time.Time `json:"created_at"`

	AccountData *Account `json:"account,omitempty"`
}

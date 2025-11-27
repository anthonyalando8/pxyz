package domain

import (
	"encoding/json"
	"time"
)

// DrCr represents debit or credit
type DrCr string

const (
	DrCrDebit  DrCr = "DR"
	DrCrCredit DrCr = "CR"
)

// Ledger represents a single ledger line item (DR/CR)
// This is the atomic unit of double-entry bookkeeping
type Ledger struct {
	ID           int64           `json:"id" db:"id"`
	JournalID    int64           `json:"journal_id" db:"journal_id"`
	AccountID    int64           `json:"account_id" db:"account_id"`
	AccountType  AccountType     `json:"account_type" db:"account_type"`        // real or demo
	Amount       int64           `json:"amount" db:"amount"`                    // In smallest currency unit
	DrCr         DrCr            `json:"dr_cr" db:"dr_cr"`                      // DR or CR
	Currency     string          `json:"currency" db:"currency"`                // Max 8 chars
	ReceiptCode  *string         `json:"receipt_code,omitempty" db:"receipt_code"` // Link to receipt
	BalanceAfter *int64          `json:"balance_after,omitempty" db:"balance_after"` // Account balance after this entry
	Description  *string         `json:"description,omitempty" db:"description"`
	Metadata     json.RawMessage `json:"metadata,omitempty" db:"metadata"`      // JSONB field
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`

	// Populated via JOIN (not in DB)
	AccountData *Account `json:"account,omitempty" db:"-"`
	JournalData *Journal `json:"journal,omitempty" db:"-"`
}

// LedgerCreate represents data needed to create a new ledger entry
type LedgerCreate struct {
	JournalID    int64
	AccountID    int64
	AccountType  AccountType
	Amount       int64
	DrCr         DrCr
	Currency     string
	ReceiptCode  *string
	BalanceAfter *int64
	Description  *string
	Metadata     json.RawMessage
}

// LedgerEntry represents a paired debit/credit entry
type LedgerEntry struct {
	Debit  *LedgerCreate
	Credit *LedgerCreate
}

// LedgerFilter represents filter criteria for ledger queries
type LedgerFilter struct {
	AccountID   *int64
	JournalID   *int64
	AccountType *AccountType
	Currency    *string
	DrCr        *DrCr
	ReceiptCode *string
	StartDate   *time.Time
	EndDate     *time.Time
	Limit       int
	Offset      int
}

// LedgerBalance represents account balance calculation
type LedgerBalance struct {
	AccountID     int64     `json:"account_id"`
	Currency      string    `json:"currency"`
	TotalDebits   int64     `json:"total_debits"`
	TotalCredits  int64     `json:"total_credits"`
	Balance       int64     `json:"balance"` // Credits - Debits
	LastLedgerID  *int64    `json:"last_ledger_id,omitempty"`
	LastUpdated   time.Time `json:"last_updated"`
}

// IsValid checks if the ledger entry has valid required fields
func (l *Ledger) IsValid() bool {
	if l.JournalID <= 0 || l.AccountID <= 0 || l.Amount <= 0 {
		return false
	}
	if l.DrCr != DrCrDebit && l.DrCr != DrCrCredit {
		return false
	}
	if len(l.Currency) == 0 || len(l.Currency) > 8 {
		return false
	}
	return true
}

// IsDebit returns true if this is a debit entry
func (l *Ledger) IsDebit() bool {
	return l.DrCr == DrCrDebit
}

// IsCredit returns true if this is a credit entry
func (l *Ledger) IsCredit() bool {
	return l.DrCr == DrCrCredit
}

// IsRealAccount returns true if this is a real account ledger
func (l *Ledger) IsRealAccount() bool {
	return l.AccountType == AccountTypeReal
}

// IsDemoAccount returns true if this is a demo account ledger
func (l *Ledger) IsDemoAccount() bool {
	return l.AccountType == AccountTypeDemo
}

// SetMetadata sets the metadata field from a struct
func (l *Ledger) SetMetadata(data interface{}) error {
	if data == nil {
		l.Metadata = nil
		return nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	l.Metadata = bytes
	return nil
}

// GetMetadata unmarshals the metadata field into a struct
func (l *Ledger) GetMetadata(target interface{}) error {
	if l.Metadata == nil {
		return nil
	}
	return json.Unmarshal(l.Metadata, target)
}

// ValidatePairedEntry validates that debit and credit entries balance
func ValidatePairedEntry(debit, credit *LedgerCreate) error {
	if debit.Amount != credit.Amount {
		return ErrUnbalancedEntry
	}
	if debit.DrCr != DrCrDebit {
		return ErrInvalidDrCr
	}
	if credit.DrCr != DrCrCredit {
		return ErrInvalidDrCr
	}
	if debit.AccountType != credit.AccountType {
		return ErrMixedAccountTypes
	}
	return nil
}

// Common errors
var (
	ErrUnbalancedEntry    = NewDomainError("unbalanced ledger entry")
	ErrInvalidDrCr        = NewDomainError("invalid DR/CR value")
	ErrMixedAccountTypes  = NewDomainError("cannot mix real and demo accounts in same entry")
)

// DomainError represents a domain-level error
type DomainError struct {
	message string
}

func NewDomainError(message string) *DomainError {
	return &DomainError{message: message}
}

func (e *DomainError) Error() string {
	return e.message
}
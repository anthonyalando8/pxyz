package domain

import (
	"time"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeDeposit      TransactionType = "deposit"
	TransactionTypeWithdrawal   TransactionType = "withdrawal"
	TransactionTypeConversion   TransactionType = "conversion"
	TransactionTypeTrade        TransactionType = "trade"
	TransactionTypeTransfer     TransactionType = "transfer"
	TransactionTypeFee          TransactionType = "fee"
	TransactionTypeCommission   TransactionType = "commission"
	TransactionTypeReversal     TransactionType = "reversal"
	TransactionTypeAdjustment   TransactionType = "adjustment"
	TransactionTypeDemoFunding  TransactionType = "demo_funding"
    TransactionTypeRefund      TransactionType = "refund"
)

// AccountType represents real or demo account
type AccountType string

const (
	AccountTypeReal AccountType = "real"
	AccountTypeDemo AccountType = "demo"
	AccountTypeSystem AccountType = "system"
)

// // OwnerType represents the type of entity creating the transaction
type OwnerType string

const (
	OwnerTypeSystem  OwnerType = "system"
	OwnerTypeUser    OwnerType = "user"
	OwnerTypeAgent   OwnerType = "agent"
	OwnerTypePartner OwnerType = "partner"
    OwnerTypeAdmin OwnerType = "admin"
)

// Journal represents a transaction header (container for ledger entries)
type Journal struct {
	ID                    int64           `json:"id" db:"id"`
	IdempotencyKey        *string         `json:"idempotency_key,omitempty" db:"idempotency_key"`
	TransactionType       TransactionType `json:"transaction_type" db:"transaction_type"`
	AccountType           AccountType     `json:"account_type" db:"account_type"`
	ExternalRef           *string         `json:"external_ref,omitempty" db:"external_ref"`
	Description           *string         `json:"description,omitempty" db:"description"`
	CreatedByExternalID   *string         `json:"created_by_external_id,omitempty" db:"created_by_external_id"` // External ID from auth service
	CreatedByType         *OwnerType      `json:"created_by_type,omitempty" db:"created_by_type"`
	IPAddress             *string         `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent             *string         `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt             time.Time       `json:"created_at" db:"created_at"`
}

// JournalCreate represents data needed to create a new journal
type JournalCreate struct {
	IdempotencyKey      *string
	TransactionType     TransactionType
	AccountType         AccountType
	ExternalRef         *string
	Description         *string
	CreatedByExternalID *string
	CreatedByType       *OwnerType
	IPAddress           *string
	UserAgent           *string
}

// IsValid checks if the journal has valid required fields
func (j *Journal) IsValid() bool {
	return j.TransactionType != "" && j.AccountType != ""
}

// IsRealTransaction returns true if this is a real money transaction
func (j *Journal) IsRealTransaction() bool {
	return j.AccountType == AccountTypeReal
}

// IsDemoTransaction returns true if this is a demo account transaction
func (j *Journal) IsDemoTransaction() bool {
	return j.AccountType == AccountTypeDemo
}

// CanHaveType checks if transaction type is valid for account type
func (j *Journal) CanHaveType() bool {
	// Demo accounts can only have certain transaction types
	if j.AccountType == AccountTypeDemo {
		restrictedTypes := []TransactionType{
			TransactionTypeDeposit,
			TransactionTypeWithdrawal,
			TransactionTypeTransfer,
			TransactionTypeFee,
			TransactionTypeCommission,
		}
		
		for _, restricted := range restrictedTypes {
			if j.TransactionType == restricted {
				return false
			}
		}
	}
	return true
}

// JournalFilter represents filter criteria for journal queries
type JournalFilter struct {
	AccountID       *int64
	TransactionType *TransactionType
	AccountType     *AccountType
	ExternalRef     *string
	CreatedByID     *string
	StartDate       *time.Time
	EndDate         *time.Time
	Limit           int
	Offset          int
}
package domain

import (
	"errors"
	"time"
)

// LedgerAggregate represents a complete transaction with all its parts
type LedgerAggregate struct {
	Journal  *Journal
	Ledgers  []*Ledger  // Double-entry ledger entries
	Receipt  *Receipt   // Optional receipt reference
	Fees     []*TransactionFee // Applied fees
}

// Receipt represents a transaction receipt (optional)
type Receipt struct {
	Code         string
	ReceiptType  TransactionType
	AccountType  AccountType
	Amount       int64
	Currency     string
	Status       string
	ExternalRef  *string
	CreatedAt    string
}

// TransactionRequest represents a request to create a transaction
type TransactionRequest struct {
	// Transaction metadata
	IdempotencyKey      *string
	TransactionType     TransactionType
	AccountType         AccountType
	ExternalRef         *string
	Description         *string
	CreatedByExternalID *string
	CreatedByType       *OwnerType
	IPAddress           *string
	UserAgent           *string
	
	// Ledger entries (must balance)
	Entries []*LedgerEntryRequest
	
	// Optional receipt
	GenerateReceipt bool
}

// LedgerEntryRequest represents a single ledger entry
type LedgerEntryRequest struct {
	AccountNumber string      // Account to debit/credit
	Amount        int64       // Amount in smallest unit
	DrCr          DrCr        // DR or CR
	Currency      string      // Currency code
	Description   *string     // Optional description
	ReceiptCode   *string     // Optional receipt code
}


// ===============================
// TRANSACTION RESULT TYPES
// ===============================

// TransactionResult represents the result of a transaction execution
type TransactionResult struct {
	ReceiptCode    string
	TransactionID  int64
	Status         string
	Amount         int64
	Currency       string
	Fee            int64
	ProcessingTime time.Duration
	CreatedAt      time.Time
}

// TransactionStatusDetails represents detailed status information
type TransactionStatus struct {
	ReceiptCode  string
	Status       string
	ErrorMessage string
	StartedAt    time.Time
	UpdatedAt    time.Time
}


// Validate checks if the transaction request is valid
func (r *TransactionRequest) Validate() error {
	if r.TransactionType == "" {
		return errors.New("transaction_type is required")
	}
	if r.AccountType == "" {
		return errors.New("account_type is required")
	}
	if len(r.Entries) < 2 {
		return errors.New("at least 2 ledger entries required for double-entry")
	}
	
	// Validate entries balance (debits = credits)
	var totalDebits, totalCredits int64
	currencyMap := make(map[string]bool)
	
	for _, entry := range r.Entries {
		if entry.AccountNumber == "" {
			return errors.New("account_number required for all entries")
		}
		if entry.Amount <= 0 {
			return errors.New("amount must be positive")
		}
		if entry.DrCr != DrCrDebit && entry.DrCr != DrCrCredit {
			return errors.New("dr_cr must be DR or CR")
		}
		if entry.Currency == "" {
			return errors.New("currency required for all entries")
		}
		
		currencyMap[entry.Currency] = true
		
		if entry.DrCr == DrCrDebit {
			totalDebits += entry.Amount
		} else {
			totalCredits += entry.Amount
		}
	}
	
	// Check balance
	if totalDebits != totalCredits {
		return ErrUnbalancedEntry
	}
	
	// All entries must be same currency for now (multi-currency later)
	if len(currencyMap) > 1 {
		return errors.New("all entries must be in same currency")
	}
	
	return nil
}

// IsBalanced checks if debits equal credits
func (r *TransactionRequest) IsBalanced() bool {
	var totalDebits, totalCredits int64
	
	for _, entry := range r.Entries {
		if entry.DrCr == DrCrDebit {
			totalDebits += entry.Amount
		} else {
			totalCredits += entry.Amount
		}
	}
	
	return totalDebits == totalCredits
}

// GetCurrency returns the currency of the transaction (assumes single currency)
func (r *TransactionRequest) GetCurrency() string {
	if len(r.Entries) > 0 {
		return r.Entries[0].Currency
	}
	return ""
}

// SimpleTransfer creates a simple A->B transfer request
func SimpleTransfer(fromAccount, toAccount string, amount int64, currency string, accountType AccountType) *TransactionRequest {
	return &TransactionRequest{
		TransactionType: TransactionTypeTransfer,
		AccountType:     accountType,
		Entries: []*LedgerEntryRequest{
			{
				AccountNumber: fromAccount,
				Amount:        amount,
				DrCr:          DrCrDebit,
				Currency:      currency,
			},
			{
				AccountNumber: toAccount,
				Amount:        amount,
				DrCr:          DrCrCredit,
				Currency:      currency,
			},
		},
		GenerateReceipt: true,
	}
}
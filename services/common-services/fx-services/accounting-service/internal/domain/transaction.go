package domain

import (
	"errors"
	"fmt"
	"math"
	"time"
	xerrors "x/shared/utils/errors"
)

// LedgerAggregate represents a complete transaction with all its parts
type LedgerAggregate struct {
	Journal *Journal
	Ledgers []*Ledger         // Double-entry ledger entries
	Receipt *Receipt          // Optional receipt reference
	Fees    []*TransactionFee // Applied fees
}

// Receipt represents a transaction receipt (optional)
type Receipt struct {
	Code        string
	ReceiptType TransactionType
	AccountType AccountType
	Amount      float64
	Currency    string
	Status      string
	ExternalRef *string
	CreatedAt   string
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

	TransactionFee *TransactionFee

	// Optional receipt
	GenerateReceipt bool
	ReceiptCode     *string

	AgentExternalID     *string                `json:"agent_external_id,omitempty"` // Agent who facilitated transaction
	IsSystemTransaction bool                   `json:"is_system_transaction"`       // If true, no fees applied
	Metadata            map[string]interface{} `json:"metadata,omitempty"`          // Additional metadata
}

// LedgerEntryRequest represents a single ledger entry
type LedgerEntryRequest struct {
	AccountNumber string                 // Account to debit/credit
	Amount        float64                // Amount in smallest unit
	DrCr          DrCr                   // DR or CR
	Currency      string                 // Currency code
	Description   *string                // Optional description
	ReceiptCode   *string                // Optional receipt code
	Metadata      map[string]interface{} `json:"metadata,omitempty"` // Additional metadata
}

// ===============================
// TRANSACTION RESULT TYPES
// ===============================

// TransactionResult represents the result of a transaction execution
type TransactionResult struct {
	ReceiptCode    string
	TransactionID  int64
	Status         string
	Amount         float64
	Currency       string
	Fee            float64
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

// CreditRequest represents a simple credit operation (add money to account)
type CreditRequest struct {
	AccountNumber       string                 `json:"account_number"`
	Amount              float64                `json:"amount"`
	Currency            string                 `json:"currency"`
	AccountType         AccountType            `json:"account_type"`
	Description         string                 `json:"description"`
	IdempotencyKey      *string                `json:"idempotency_key,omitempty"`
	ExternalRef         *string                `json:"external_ref,omitempty"`
	CreatedByExternalID string                 `json:"created_by_external_id"`
	CreatedByType       OwnerType              `json:"created_by_type"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ReceiptCode         *string
	TransactionType     TransactionType `json:"transaction_type"`
}

// DebitRequest represents a simple debit operation (remove money from account)
type DebitRequest struct {
	AccountNumber       string                 `json:"account_number"`
	Amount              float64                `json:"amount"`
	Currency            string                 `json:"currency"`
	AccountType         AccountType            `json:"account_type"`
	Description         string                 `json:"description"`
	IdempotencyKey      *string                `json:"idempotency_key,omitempty"`
	ExternalRef         *string                `json:"external_ref,omitempty"`
	CreatedByExternalID string                 `json:"created_by_external_id"`
	CreatedByType       OwnerType              `json:"created_by_type"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ReceiptCode         *string
	TransactionType     TransactionType `json:"transaction_type"`
}

// TransferRequest represents a transfer between two accounts (same currency)
type TransferRequest struct {
	FromAccountNumber   string                 `json:"from_account_number"`
	ToAccountNumber     string                 `json:"to_account_number"`
	Amount              float64                `json:"amount"`
	AccountType         AccountType            `json:"account_type"`
	Description         string                 `json:"description"`
	IdempotencyKey      *string                `json:"idempotency_key,omitempty"`
	ExternalRef         *string                `json:"external_ref,omitempty"`
	CreatedByExternalID string                 `json:"created_by_external_id"`
	CreatedByType       OwnerType              `json:"created_by_type"`
	AgentExternalID     *string                `json:"agent_external_id,omitempty"` // Agent who facilitated
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ReceiptCode         *string
	TransactionType     TransactionType `json:"transaction_type"`
	TransactionFee *TransactionFee
}

// ConversionRequest represents a currency conversion transfer
type ConversionRequest struct {
	FromAccountNumber   string                 `json:"from_account_number"` // USD account
	ToAccountNumber     string                 `json:"to_account_number"`   // EUR account
	Amount              float64                `json:"amount"`              // Amount in source currency
	AccountType         AccountType            `json:"account_type"`
	IdempotencyKey      *string                `json:"idempotency_key,omitempty"`
	ExternalRef         *string                `json:"external_ref,omitempty"`
	CreatedByExternalID string                 `json:"created_by_external_id"`
	CreatedByType       OwnerType              `json:"created_by_type"`
	AgentExternalID     *string                `json:"agent_external_id,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ReceiptCode         *string
	TransactionFee *TransactionFee
}

type AgentCommissionRequest struct {
	AgentExternalID   string  `json:"agent_external_id"`
	TransactionRef    string  `json:"transaction_ref"` // Reference to original transaction
	Currency          string  `json:"currency"`
	TransactionAmount float64 `json:"transaction_amount"` // Original transaction amount
	CommissionAmount  float64 `json:"commission_amount"`  // Calculated commission
	CommissionRate    *string `json:"commission_rate"`    // Rate used for calculation
	IdempotencyKey    *string `json:"idempotency_key,omitempty"`
	ReceiptCode       *string
}
type TradeRequest struct {
	AccountNumber       string                 `json:"account_number"`
	Amount              float64                `json:"amount"`
	Currency            string                 `json:"currency"`
	AccountType         AccountType            `json:"account_type"`
	TradeID             string                 `json:"trade_id"`
	TradeType           string                 `json:"trade_type"` // e.g., "forex", "crypto", "sports"
	IdempotencyKey      *string                `json:"idempotency_key,omitempty"`
	CreatedByExternalID string                 `json:"created_by_external_id"`
	CreatedByType       OwnerType              `json:"created_by_type"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ReceiptCode         *string
}

// ========================================
// VALIDATION METHODS
// ========================================

func (r *CreditRequest) Validate() error {
	if r.AccountNumber == "" {
		return xerrors.ErrInvalidAccountNumber
	}
	if r.Amount <= 0 {
		return xerrors.ErrInvalidAmount
	}
	// if r.Currency == "" {
	// 	return xerrors.ErrInvalidCurrency
	// }
	if r.CreatedByExternalID == "" {
		return xerrors.ErrRequiredFieldMissing
	}
	return nil
}

func (r *DebitRequest) Validate() error {
	if r.AccountNumber == "" {
		return xerrors.ErrInvalidAccountNumber
	}
	if r.Amount <= 0 {
		return xerrors.ErrInvalidAmount
	}
	// if r.Currency == "" {
	// 	return xerrors.ErrInvalidCurrency
	// }
	if r.CreatedByExternalID == "" {
		return xerrors.ErrRequiredFieldMissing
	}
	return nil
}

func (r *TransferRequest) Validate() error {
	if r.FromAccountNumber == "" || r.ToAccountNumber == "" {
		return xerrors.ErrInvalidAccountNumber
	}
	if r.FromAccountNumber == r.ToAccountNumber {
		return xerrors.ErrInvalidTransaction
	}
	if r.Amount <= 0 {
		return xerrors.ErrInvalidAmount
	}
	if r.CreatedByExternalID == "" {
		return xerrors.ErrRequiredFieldMissing
	}
	return nil
}

func (r *ConversionRequest) Validate() error {
	if r.FromAccountNumber == "" || r.ToAccountNumber == "" {
		return xerrors.ErrInvalidAccountNumber
	}
	if r.FromAccountNumber == r.ToAccountNumber {
		return xerrors.ErrInvalidTransaction
	}
	if r.Amount <= 0 {
		return xerrors.ErrInvalidAmount
	}
	if r.CreatedByExternalID == "" {
		return xerrors.ErrRequiredFieldMissing
	}
	return nil
}

func (r *TradeRequest) Validate() error {
	if r.AccountNumber == "" {
		return xerrors.ErrInvalidAccountNumber
	}
	if r.Amount <= 0 {
		return xerrors.ErrInvalidAmount
	}
	// if r.Currency == "" {
	// 	return xerrors.ErrInvalidCurrency
	// }
	if r.TradeID == "" {
		return xerrors.ErrRequiredFieldMissing
	}
	if r.CreatedByExternalID == "" {
		return xerrors.ErrRequiredFieldMissing
	}
	return nil
}

func (r *AgentCommissionRequest) Validate() error {
	if r.AgentExternalID == "" {
		return xerrors.ErrAgentNotFound
	}
	if r.Currency == "" {
		return xerrors.ErrInvalidCurrency
	}
	if r.CommissionAmount <= 0 {
		return xerrors.ErrInvalidFeeAmount
	}
	if r.TransactionRef == "" {
		return xerrors.ErrRequiredFieldMissing
	}
	return nil
}

// ========================================
// HELPER METHODS FOR TransactionRequest
// ========================================

// GetTotalAmount returns the debit amount (transaction amount)
func (r *TransactionRequest) GetTotalAmount() float64 {
	for _, entry := range r.Entries {
		if entry.DrCr == DrCrDebit {
			return entry.Amount
		}
	}
	return 0
}

// IsConversion checks if transaction involves multiple currencies
func (r *TransactionRequest) IsConversion() bool {
	if len(r.Entries) < 2 {
		return false
	}

	currency := r.Entries[0].Currency
	for _, entry := range r.Entries[1:] {
		if entry.Currency != currency {
			return true
		}
	}

	return false
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
		return errors.New("at least 2 ledger entries required")
	}

	// ✅ Route to appropriate validator based on transaction type
	switch r.TransactionType {
	case TransactionTypeConversion:
		return r.validateConversion()
	case TransactionTypeTransfer, TransactionTypeDeposit, TransactionTypeWithdrawal:
		return r.validateStandardTransaction()
	default:
		return r.validateStandardTransaction()
	}
}

// ✅ Standard transaction validation (single currency, balanced)
func (r *TransactionRequest) validateStandardTransaction() error {
	var totalDebits, totalCredits float64
	currencyMap := make(map[string]bool)

	for _, entry := range r. Entries {
		if err := entry.Validate(); err != nil {
			return err
		}

		currencyMap[entry.Currency] = true

		if entry.DrCr == DrCrDebit {
			totalDebits += entry. Amount
		} else {
			totalCredits += entry.Amount
		}
	}

	// Must be single currency
	if len(currencyMap) > 1 {
		return errors.New("standard transactions must use single currency")
	}

	// Must balance
	if math.Abs(totalDebits-totalCredits) > 0.0001 { // Use epsilon for float comparison
		return fmt.Errorf("unbalanced transaction: debits=%.2f, credits=%.2f", totalDebits, totalCredits)
	}

	return nil
}

// ✅ Conversion validation (multi-currency allowed, FX-based balance)
func (r *TransactionRequest) validateConversion() error {
	if len(r.Entries) < 2 || len(r.Entries) > 4 {
		return errors.New("conversion must have 2-4 entries (source, dest, optional fee, optional agent)")
	}

	currencyBalances := make(map[string]struct {
		debits  float64
		credits float64
		count   int
	})

	for _, entry := range r.Entries {
		if err := entry.Validate(); err != nil {
			return err
		}

		balance := currencyBalances[entry.Currency]
		balance.count++
		if entry.DrCr == DrCrDebit {
			balance. debits += entry.Amount
		} else {
			balance.credits += entry.Amount
		}
		currencyBalances[entry. Currency] = balance
	}

	// Must involve at least 2 currencies
	if len(currencyBalances) < 2 {
		return errors.New("conversion must involve at least 2 currencies")
	}

	// ✅ Each currency should have either pure debit or pure credit (with possible fee in dest currency)
	sourceCount := 0
	destCount := 0

	for _, balance := range currencyBalances {
		// Source currency: should be all debits
		if balance.debits > 0 && balance.credits == 0 {
			sourceCount++
		}
		// Dest currency: should have credits (and maybe fee debit)
		if balance.credits > 0 {
			destCount++
		}
	}

	if sourceCount == 0 {
		return errors.New("conversion must have source currency debit")
	}
	if destCount == 0 {
		return errors.New("conversion must have destination currency credit")
	}

	return nil
}

// Entry validation helper
func (e *LedgerEntryRequest) Validate() error {
	if e.AccountNumber == "" {
		return errors.New("account_number required")
	}
	if e.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if e.DrCr != DrCrDebit && e.DrCr != DrCrCredit {
		return errors.New("dr_cr must be DR or CR")
	}
	if e.Currency == "" {
		return errors.New("currency required")
	}
	return nil
}

// IsBalanced checks if debits equal credits
func (r *TransactionRequest) IsBalanced() bool {
	var totalDebits, totalCredits float64

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
func SimpleTransfer(fromAccount, toAccount string, amount float64, currency string, accountType AccountType) *TransactionRequest {
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

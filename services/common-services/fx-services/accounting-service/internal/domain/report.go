package domain

import "time"

// DailyReport represents aggregated financial data for reporting
type DailyReport struct {
	// Owner info (system, partner, user, agent)
	OwnerType   OwnerType   `json:"owner_type" db:"owner_type"`
	OwnerID     string      `json:"owner_id" db:"owner_id"` // External ID from auth service
	
	// Account / currency info
	AccountID     int64       `json:"account_id,omitempty" db:"account_id"`
	AccountNumber string      `json:"account_number" db:"account_number"`
	AccountType   AccountType `json:"account_type" db:"account_type"`
	Currency      string      `json:"currency" db:"currency"`

	// Aggregated values (in smallest currency unit)
	TotalDebit  int64 `json:"total_debit" db:"total_debit"`
	TotalCredit int64 `json:"total_credit" db:"total_credit"`
	Balance     int64 `json:"balance" db:"balance"`
	NetChange   int64 `json:"net_change" db:"net_change"` // TotalCredit - TotalDebit

	// Report date
	Date time.Time `json:"date" db:"date"`
}

// AccountStatement represents a detailed account statement
type AccountStatement struct {
	AccountID      int64       `json:"account_id"`
	AccountNumber  string      `json:"account_number"`
	AccountType    AccountType `json:"account_type"`
	OwnerType      OwnerType   `json:"owner_type"`
	OwnerID        string      `json:"owner_id"`
	Currency       string      `json:"currency"`
	
	OpeningBalance int64       `json:"opening_balance"`
	ClosingBalance int64       `json:"closing_balance"`
	TotalDebits    int64       `json:"total_debits"`
	TotalCredits   int64       `json:"total_credits"`
	
	PeriodStart    time.Time   `json:"period_start"`
	PeriodEnd      time.Time   `json:"period_end"`
	
	Ledgers        []*Ledger   `json:"ledgers"`
}

// OwnerSummary represents aggregated data for an owner across all accounts
type OwnerSummary struct {
	OwnerType                OwnerType          `json:"owner_type"`
	OwnerID                  string             `json:"owner_id"`
	AccountType              AccountType        `json:"account_type"`
	Balances                 []*AccountBalanceSummary `json:"balances"` // Changed from AccountSummary
	TotalBalance             int64              `json:"total_balance_usd_equivalent"` // Changed from map
}

// AccountBalanceSummary represents a single account's balance in summary
// type AccountBalanceSummary struct {
// 	AccountID        int64  `json:"account_id"`
// 	AccountNumber    string `json:"account_number"`
// 	Currency         string `json:"currency"`
// 	Balance          int64  `json:"balance"`
// 	AvailableBalance int64  `json:"available_balance"`
// }

// TransactionSummary represents transaction statistics
type TransactionSummary struct {
	AccountType       AccountType       `json:"account_type"`
	TransactionType   TransactionType   `json:"transaction_type"`
	Currency          string            `json:"currency"`
	Count             int64             `json:"count"`
	TotalVolume       int64             `json:"total_volume"`
	AverageAmount     int64             `json:"average_amount"`
	MinAmount         int64             `json:"min_amount"`
	MaxAmount         int64             `json:"max_amount"`
	PeriodStart       time.Time         `json:"period_start"`
	PeriodEnd         time.Time         `json:"period_end"`
}

// StatementFilter represents filter criteria for statement queries
type StatementFilter struct {
	AccountNumber  *string
	OwnerID        *string
	OwnerType      *OwnerType
	AccountType    *AccountType
	Currency       *string
	StartDate      *time.Time
	EndDate        *time.Time
	MinAmount      *int64
	MaxAmount      *int64
	Limit          int
	Offset         int
}
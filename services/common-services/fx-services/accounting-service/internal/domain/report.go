package domain

import "time"

// DailyReport represents aggregated financial data for reporting
type DailyReport struct {
	// Owner info (can be system, partner, or user)
	OwnerType string `json:"owner_type"`
	OwnerID   string `json:"owner_id"`

	// Account / currency info
	AccountID int64  `json:"account_id,omitempty"`
	AccountNumber string `json:"account_number"`
	Currency  string `json:"currency"`

	// Aggregated values
	TotalDebit  float64 `json:"total_debit"`
	TotalCredit float64 `json:"total_credit"`
	Balance     float64 `json:"balance"`
	NetChange float64 `json:"net_change"`

	// Report date
	Date time.Time `json:"date"`
}

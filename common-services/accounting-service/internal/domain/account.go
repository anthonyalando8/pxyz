package domain

import "time"

// Account represents a ledger account for user, partner, or system
type Account struct {
	ID            int64  `json:"id"`
	OwnerType     string `json:"owner_type"`   // system | partner | user
	OwnerID       string `json:"owner_id"`
	Currency      string `json:"currency"`
	Purpose       string `json:"purpose"`      // liquidity, wallet, fees, etc.
	AccountType   string `json:"account_type"` // real | demo
	IsActive      bool   `json:"is_active"`
	AccountNumber string `json:"account_number"` // new field
	CreatedAt     time.Time `json:"-"`           // omit in JSON
	UpdatedAt     time.Time `json:"-"`           // omit in JSON
}
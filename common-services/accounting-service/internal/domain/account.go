package domain

import (
	"fmt"
	"math/rand"
	"time"
)

func generateAccountNumber(prefix, currency string) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return fmt.Sprintf("%s-%s-%s", prefix, currency, string(b))
}

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
	Balance Balance `json:"balance"`
}

type AccountFilter struct {
	OwnerType   *string
	OwnerID     *string
	Currency    *string
	Purpose     *string
	AccountType *string
	IsActive    *bool
	AccountNumber *string
}

var DefaultSystemAccounts = []*Account{
	// --- System Wallets ---
	{
		OwnerType:   "system",
		OwnerID:     "SYSTEM",
		Currency:    "USD",
		Purpose:     "wallet",
		AccountType: "real",
		AccountNumber: generateAccountNumber("WL", "USD"),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   10_000_000,
			UpdatedAt: time.Now(),
		},
	},
	{
		OwnerType:   "system",
		OwnerID:     "SYSTEM",
		Currency:    "BTC",
		Purpose:     "wallet",
		AccountType: "real",
		AccountNumber: generateAccountNumber("WL", "BTC"),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   10_000_000,
			UpdatedAt: time.Now(),
		},
	},
	{
		OwnerType:   "system",
		OwnerID:     "SYSTEM",
		Currency:    "USDT",
		Purpose:     "wallet",
		AccountNumber: generateAccountNumber("WL", "USDT"),
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   10_000_000,
			UpdatedAt: time.Now(),
		},
	},

	// --- System Profits ---
	{
		OwnerType:   "system",
		OwnerID:     "SYSTEM",
		Currency:    "USD",
		AccountNumber: generateAccountNumber("WL", "USD"),
		Purpose:     "profits",
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   0, // starts empty
			UpdatedAt: time.Now(),
		},
	},
	{
		OwnerType:   "system",
		OwnerID:     "SYSTEM",
		Currency:    "BTC",
		AccountNumber: generateAccountNumber("WL", "BTC"),
		Purpose:     "profits",
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   0, // starts empty
			UpdatedAt: time.Now(),
		},
	},
	{
		OwnerType:   "system",
		OwnerID:     "SYSTEM",
		Currency:    "USDT",
		Purpose:     "profits",
		AccountNumber: generateAccountNumber("WL", "USDT"),
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   0, // starts empty
			UpdatedAt: time.Now(),
		},
	},

	// --- Partner Float ---
	{
		OwnerType:   "system",
		OwnerID:     "PARTNER_POOL",
		Currency:    "USD",
		AccountNumber: generateAccountNumber("WL", "USD"),
		Purpose:     "partner_float",
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   1_000_000_000,
			UpdatedAt: time.Now(),
		},
	},
	{
		OwnerType:   "system",
		OwnerID:     "PARTNER_POOL",
		Currency:    "BTC",
		AccountNumber: generateAccountNumber("WL", "BTC"),
		Purpose:     "partner_float",
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   1_000_000_000,
			UpdatedAt: time.Now(),
		},
	},
	{
		OwnerType:   "system",
		OwnerID:     "PARTNER_POOL",
		Currency:    "USDT",
		Purpose:     "partner_float",
		AccountNumber: generateAccountNumber("WL", "USDT"),
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Balance: Balance{
			Balance:   1_000_000_000,
			UpdatedAt: time.Now(),
		},
	},
}

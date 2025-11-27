package domain

import (
	"time"
)

// Account represents a ledger account for user, agent, partner, or system
// Supports both real and demo accounts with multi-tenancy
type Account struct {
	// Primary fields
	ID            int64       `json:"id" db:"id"`
	AccountNumber string      `json:"account_number" db:"account_number"` // Unique identifier
	OwnerType     OwnerType   `json:"owner_type" db:"owner_type"`         // system | user | agent | partner
	OwnerID       string      `json:"owner_id" db:"owner_id"`             // External ID from auth service (UUID/string)
	Currency      string      `json:"currency" db:"currency"`             // VARCHAR(8) - e.g., USD, BTC, USDT
	Purpose       AccountPurpose `json:"purpose" db:"purpose"`            // liquidity | clearing | fees | wallet | escrow | settlement | revenue | contra | commission
	AccountType   AccountType `json:"account_type" db:"account_type"`     // real | demo

	// Status and control fields
	IsActive       bool  `json:"is_active" db:"is_active"`
	IsLocked       bool  `json:"is_locked" db:"is_locked"`
	OverdraftLimit int64 `json:"overdraft_limit" db:"overdraft_limit"` // In smallest currency unit (cents/satoshis)

	// Agent-specific fields (only for accounts owned by agents or user accounts with agent parents)
	ParentAgentExternalID *string `json:"parent_agent_external_id,omitempty" db:"parent_agent_external_id"` // Agent external ID from auth service
	CommissionRate        *string `json:"commission_rate,omitempty" db:"commission_rate"`                   // NUMERIC(5,4) as string - e.g., "0.0025" for 0.25%

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Related objects (not persisted, loaded separately)
	Balance *Balance `json:"balance,omitempty" db:"-"`
}

// AccountPurpose represents the purpose of an account
type AccountPurpose string

const (
	PurposeLiquidity  AccountPurpose = "liquidity"
	PurposeClearing   AccountPurpose = "clearing"
	PurposeFees       AccountPurpose = "fees"
	PurposeWallet     AccountPurpose = "wallet"
	PurposeEscrow     AccountPurpose = "escrow"
	PurposeSettlement AccountPurpose = "settlement"
	PurposeRevenue    AccountPurpose = "revenue"
	PurposeContra     AccountPurpose = "contra"
	PurposeCommission AccountPurpose = "commission"
	PurposeInvestment AccountPurpose = "investment"
	PurposeSavings   AccountPurpose = "savings"
)

// AccountFilter supports efficient filtering for high-throughput queries
type AccountFilter struct {
	OwnerType             *OwnerType
	OwnerID               *string
	Currency              *string
	Purpose               *AccountPurpose
	AccountType           *AccountType
	IsActive              *bool
	IsLocked              *bool
	AccountNumber         *string
	ParentAgentExternalID *string
}

// CreateAccountRequest is used for account creation
type CreateAccountRequest struct {
	OwnerType             OwnerType
	OwnerID               string
	Currency              string
	Purpose               AccountPurpose
	AccountType           AccountType
	ParentAgentExternalID *string
	CommissionRate        *string // NUMERIC(5,4) as string
	OverdraftLimit        int64
	InitialBalance        int64
}


// AccountTotals represents calculated totals for an account
type AccountTotals struct {
	AccountNumber    string
	AccountType      AccountType
	TotalDebits      int64
	TotalCredits     int64
	NetChange        int64
	TransactionCount int64
	PeriodStart      time.Time
	PeriodEnd        time.Time
}



// AccountBalanceSummary represents a single account's balance in summary
type AccountBalanceSummary struct {
	AccountID        int64
	AccountNumber    string
	Currency         string
	Balance          int64
	AvailableBalance int64
}

// IsValid checks if the account has valid required fields
func (a *Account) IsValid() bool {
	if a.AccountNumber == "" || a.OwnerType == "" || a.OwnerID == "" {
		return false
	}
	if len(a.Currency) == 0 || len(a.Currency) > 8 {
		return false
	}
	if a.Purpose == "" || a.AccountType == "" {
		return false
	}
	return true
}

// IsSystemAccount returns true if this is a system-owned account
func (a *Account) IsSystemAccount() bool {
	return a.OwnerType == OwnerTypeSystem
}

// IsUserAccount returns true if this is a user-owned account
func (a *Account) IsUserAccount() bool {
	return a.OwnerType == OwnerTypeUser
}

// IsAgentAccount returns true if this is an agent-owned account
func (a *Account) IsAgentAccount() bool {
	return a.OwnerType == OwnerTypeAgent
}

// IsRealAccount returns true if this is a real money account
func (a *Account) IsRealAccount() bool {
	return a.AccountType == AccountTypeReal
}

// IsDemoAccount returns true if this is a demo account
func (a *Account) IsDemoAccount() bool {
	return a.AccountType == AccountTypeDemo
}

// HasAgentParent returns true if this account has a parent agent
func (a *Account) HasAgentParent() bool {
	return a.ParentAgentExternalID != nil && *a.ParentAgentExternalID != ""
}

// CanOverdraft returns true if this account allows overdraft
func (a *Account) CanOverdraft() bool {
	return a.OverdraftLimit > 0
}

// DefaultSystemAccounts returns system accounts for initialization
// These match the schema's initial data exactly
func DefaultSystemAccounts() []*Account {
	now := time.Now()
	
	return []*Account{
		// System liquidity accounts
		{
			AccountNumber:  "SYS-LIQ-USD",
			OwnerType:      OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       "USD",
			Purpose:        PurposeLiquidity,
			AccountType:    AccountTypeReal,
			IsActive:       true,
			IsLocked:       false,
			OverdraftLimit: 0,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			AccountNumber:  "SYS-LIQ-USDT",
			OwnerType:      OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       "USDT",
			Purpose:        PurposeLiquidity,
			AccountType:    AccountTypeReal,
			IsActive:       true,
			IsLocked:       false,
			OverdraftLimit: 0,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			AccountNumber:  "SYS-LIQ-BTC",
			OwnerType:      OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       "BTC",
			Purpose:        PurposeLiquidity,
			AccountType:    AccountTypeReal,
			IsActive:       true,
			IsLocked:       false,
			OverdraftLimit: 0,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		// System fee accounts
		{
			AccountNumber:  "SYS-FEE-USD",
			OwnerType:      OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       "USD",
			Purpose:        PurposeFees,
			AccountType:    AccountTypeReal,
			IsActive:       true,
			IsLocked:       false,
			OverdraftLimit: 0,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			AccountNumber:  "SYS-FEE-USDT",
			OwnerType:      OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       "USDT",
			Purpose:        PurposeFees,
			AccountType:    AccountTypeReal,
			IsActive:       true,
			IsLocked:       false,
			OverdraftLimit: 0,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			AccountNumber:  "SYS-FEE-BTC",
			OwnerType:      OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       "BTC",
			Purpose:        PurposeFees,
			AccountType:    AccountTypeReal,
			IsActive:       true,
			IsLocked:       false,
			OverdraftLimit: 0,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
}
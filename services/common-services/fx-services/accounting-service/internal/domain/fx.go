package domain

import (
	"time"
)

// Currency represents a supported currency (fiat or crypto)
type Currency struct {
	Code               string    `json:"code" db:"code"`
	Name               string    `json:"name" db:"name"`
	Symbol             *string   `json:"symbol,omitempty" db:"symbol"`
	Decimals           int16     `json:"decimals" db:"decimals"`
	IsFiat             bool      `json:"is_fiat" db:"is_fiat"`
	IsActive           bool      `json:"is_active" db:"is_active"`
	DemoEnabled        bool      `json:"demo_enabled" db:"demo_enabled"`
	DemoInitialBalance float64   `json:"demo_initial_balance" db:"demo_initial_balance"` // ✅ In decimal units
	MinAmount          float64   `json:"min_amount" db:"min_amount"`                     // ✅ In decimal units
	MaxAmount          *float64  `json:"max_amount,omitempty" db:"max_amount"`           // ✅ In decimal units (NULL = unlimited)
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

// FXRate represents an exchange rate between two currencies
type FXRate struct {
	ID            int64      `json:"id" db:"id"`
	BaseCurrency  string     `json:"base_currency" db:"base_currency"`   // Max 8 chars
	QuoteCurrency string     `json:"quote_currency" db:"quote_currency"` // Max 8 chars
	Rate          string     `json:"rate" db:"rate"`                     // NUMERIC(30,18) stored as string for precision
	BidRate       *string    `json:"bid_rate,omitempty" db:"bid_rate"`   // Optional bid rate
	AskRate       *string    `json:"ask_rate,omitempty" db:"ask_rate"`   // Optional ask rate
	Spread        *string    `json:"spread,omitempty" db:"spread"`       // Bid-Ask spread
	Source        *string    `json:"source,omitempty" db:"source"`       // Rate source/provider
	ValidFrom     time.Time  `json:"valid_from" db:"valid_from"`
	ValidTo       *time. Time `json:"valid_to,omitempty" db:"valid_to"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// FXRateQuery represents query parameters for fetching FX rates
type FXRateQuery struct {
	BaseCurrency   string
	QuoteCurrency  string
	ValidAt        time.Time // Get rate valid at this time
	IncludeExpired bool      // Include rates where valid_to is not null
}

// DefaultCurrencies returns the static list of supported currencies with demo support
func DefaultCurrencies() []*Currency {
	now := time.Now()
	usdSymbol := "$"
	btcSymbol := "₿"
	usdtSymbol := "₮"

	return []*Currency{
		{
			Code:               "USD",
			Name:               "United States Dollar",
			Symbol:             &usdSymbol,
			Decimals:           2,
			IsFiat:             true,
			IsActive:           true,
			DemoEnabled:        true,
			DemoInitialBalance: 10000.00,  // ✅ $10,000.00 (decimal)
			MinAmount:          0.01,      // ✅ $0. 01 (decimal)
			MaxAmount:          nil,       // Unlimited
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			Code:               "USDT",
			Name:               "Tether USD",
			Symbol:             &usdtSymbol,
			Decimals:           6,         // ✅ Fixed: USDT has 6 decimals
			IsFiat:             false,
			IsActive:           true,
			DemoEnabled:        true,
			DemoInitialBalance: 10000.000000,  // ✅ 10,000 USDT (decimal with 6 places)
			MinAmount:          0.000001,      // ✅ 0. 000001 USDT (1 micro-USDT)
			MaxAmount:          nil,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			Code:               "BTC",
			Name:               "Bitcoin",
			Symbol:             &btcSymbol,
			Decimals:           8,              // Satoshis
			IsFiat:             false,
			IsActive:           true,
			DemoEnabled:        true,
			DemoInitialBalance: 0.10000000,    // ✅ 0.1 BTC (decimal with 8 places)
			MinAmount:          0.00000001,    // ✅ 1 satoshi (decimal)
			MaxAmount:          nil,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
	}
}

// IsValid checks if the currency code length is valid
func (c *Currency) IsValid() bool {
	return len(c.Code) > 0 && len(c.Code) <= 8
}

// SupportsDemo returns whether this currency supports demo accounts
func (c *Currency) SupportsDemo() bool {
	return c.DemoEnabled && c.IsActive
}

// WithinLimits checks if an amount is within min/max limits
func (c *Currency) WithinLimits(amount float64) bool {
	if amount < c. MinAmount {
		return false
	}
	if c.MaxAmount != nil && amount > *c.MaxAmount {
		return false
	}
	return true
}
package domain

import "time"

type Currency struct {
	Code      string
	Name      string
	Decimals  int16
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FXRate struct {
	ID            int64
	BaseCurrency  string
	QuoteCurrency string
	Rate          float64
	AsOf          time.Time
	CreatedAt     time.Time
}

// DefaultCurrencies returns the static list of supported currencies
func DefaultCurrencies() []*Currency {
	now := time.Now()
	return []*Currency{
		{
			Code:      "USD",
			Name:      "US Dollar",
			Decimals:  2,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			Code:      "BTC",
			Name:      "Bitcoin",
			Decimals:  8, // BTC supports up to 8 decimals
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			Code:      "USDT",
			Name:      "Tether USD",
			Decimals:  6, // many exchanges use 6 for USDT
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

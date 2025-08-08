// internal/usecase/wallet/interface.go
package wallet

import "context"

// CurrencyConverter defines how to convert any currency to USD
// Used to compute user net worth

// --- interface.go ---
type CurrencyConverter interface {
	ConvertToUSD(ctx context.Context, amount float64, fromCurrency string) (float64, error)
}

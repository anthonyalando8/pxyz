package usecase

import "fmt"

var fxRates = map[string]map[string]float64{
	"USD": {
		"USD":  1,
		"USDT": 1,
		"BTC":  1.0 / 30_000, // 1 USD = 0.00003333 BTC
	},
	"USDT": {
		"USD":  1,
		"USDT": 1,
		"BTC":  1.0 / 30_000,
	},
	"BTC": {
		"USD":  30_000,
		"USDT": 30_000,
		"BTC":  1,
	},
}

// ConvertCurrency converts an amount from src to target currency
func ConvertCurrency(amount float64, srcCurrency, targetCurrency string) (float64, error) {
	srcRates, ok := fxRates[srcCurrency]
	if !ok {
		return 0, fmt.Errorf("unsupported source currency: %s", srcCurrency)
	}

	rate, ok := srcRates[targetCurrency]
	if !ok {
		return 0, fmt.Errorf("unsupported target currency: %s", targetCurrency)
	}

	return amount * rate, nil
}

// ConvertToUSD converts a given amount in any supported currency to USD.
// If the currency is not supported, it just returns the original amount unchanged.
func ConvertToUSD(currency string, amount float64) float64 {
	// Dummy FX rates — replace later with real provider
	fxRates := map[string]float64{
		"USD": 1.0,     // base
		"KES": 0.0078,  // 1 KES = 0.0078 USD (dummy rate)
		"EUR": 1.1,     // 1 EUR = 1.1 USD (dummy rate)
		"GBP": 1.25,    // 1 GBP = 1.25 USD (dummy rate)
		"NGN": 0.00065, // 1 NGN = 0.00065 USD (dummy rate)
	}

	if rate, ok := fxRates[currency]; ok {
		return amount * rate
	}

	// fallback: unsupported currency, return unchanged
	return amount
}

func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func strOrDefault(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}
func ptrStrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
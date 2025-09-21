package usecase

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

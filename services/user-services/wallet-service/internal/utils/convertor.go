package utils

import (
	"context"
	"fmt"
)

type DummyConverter struct{}

func (d *DummyConverter) ConvertToUSD(ctx context.Context, amount float64, fromCurrency string) (float64, error) {
	rates := map[string]float64{
	"USD":  1.0,        // Base
	"EUR":  1.1,        // Euro
	"KES":  0.0065,     // Kenyan Shilling
	"XAF":  0.0018,     // Central African CFA Franc
	"XOF":  0.0018,     // West African CFA Franc
	"NGN":  0.00065,    // Nigerian Naira
	"ZAR":  0.055,      // South African Rand
	"BTC":  29000.0,    // Bitcoin
	"ETH":  1800.0,     // Ethereum
	"USDT": 1.0,        // Tether (USD-pegged)
	"USDC": 1.0,        // USD Coin (USD-pegged)
	"BNB":  240.0,      // Binance Coin
	"SOL":  24.0,       // Solana
	}


	rate, ok := rates[fromCurrency]
	if !ok {
		return 0, fmt.Errorf("unsupported currency: %s", fromCurrency)
	}
	return amount * rate, nil
}

// internal/usecase/helpers.go
package usecase

import (
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"strings"
)

// assetFromCode converts asset code to domain.Asset
func assetFromCode(code string) *domain.Asset {
	switch code {
	case "TRX":
		return &domain.Asset{
			Chain:    "TRON",
			Symbol:   "TRX",
			Type:     domain.AssetTypeNative,
			Decimals: 6,
		}
	case "USDT": 
		return &domain.Asset{
			Chain:        "TRON",
			Symbol:       "USDT",
			Type:         domain.AssetTypeToken,
			ContractAddr: stringPtr("TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs"), // TRON USDT mainnet
			Decimals:     6,
		}
	case "BTC":
		return &domain.Asset{
			Chain:    "BITCOIN",
			Symbol:   "BTC",
			Type:     domain.AssetTypeNative,
			Decimals: 8,
		}
	default: 
		return nil
	}
}

// getAssetDecimals returns decimals for asset
func getAssetDecimals(assetCode string) int {
	asset := assetFromCode(assetCode)
	if asset != nil {
		return asset. Decimals
	}
	return 6 // default
}

// getAssetChain returns chain for asset
func getAssetChain(assetCode string) string {
	asset := assetFromCode(assetCode)
	if asset != nil {
		return asset.Chain
	}
	return ""
}

// formatBalance formats big.Int balance to human-readable string
func formatBalance(balance *big.Int, decimals int, asset string) string {
	if balance == nil || balance. Cmp(big.NewInt(0)) == 0 {
		return fmt.Sprintf("0 %s", asset)
	}
	
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	wholePart := new(big.Int).Div(balance, divisor)
	remainder := new(big.Int).Mod(balance, divisor)
	
	if remainder.Cmp(big. NewInt(0)) == 0 {
		return fmt.Sprintf("%s %s", wholePart.String(), asset)
	}
	
	// Format with decimals
	decimalPart := new(big.Float).Quo(
		new(big.Float).SetInt(remainder),
		new(big.Float).SetInt(divisor),
	)
	
	formatted := fmt.Sprintf("%s%s", wholePart.String(), decimalPart.Text('f', decimals)[1:])
	
	// Trim trailing zeros
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	
	return fmt.Sprintf("%s %s", formatted, asset)
}

// formatAmount is a shorthand for formatBalance
func formatAmount(amount *big.Int, asset string) string {
	decimals := getAssetDecimals(asset)
	return formatBalance(amount, decimals, asset)
}

// parseAmount converts string amount to big.Int (smallest unit)
func parseAmount(amountStr string, decimals int) (*big.Int, error) {
	// Remove spaces
	amountStr = strings. TrimSpace(amountStr)
	
	// If already in smallest unit (no decimal point), parse directly
	if ! strings.Contains(amountStr, ".") {
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok {
			return nil, fmt.Errorf("invalid amount format")
		}
		return amount, nil
	}
	
	// Split by decimal point
	parts := strings.Split(amountStr, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid decimal format")
	}
	
	wholePart := parts[0]
	decimalPart := parts[1]
	
	// Pad or truncate decimal part to match decimals
	if len(decimalPart) > decimals {
		decimalPart = decimalPart[:decimals] // Truncate
	} else {
		decimalPart = decimalPart + strings.Repeat("0", decimals-len(decimalPart)) // Pad
	}
	
	// Combine whole and decimal parts
	combined := wholePart + decimalPart
	
	amount, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse amount")
	}
	
	return amount, nil
}

// getRequiredConfirmations returns required confirmations per chain
func getRequiredConfirmations(chain string) int {
	confirmations := map[string]int{
		"TRON":     19, // TRON
		"BITCOIN": 3,  // Bitcoin
		"ETHEREUM": 12, // Ethereum
	}
	
	if conf, ok := confirmations[chain]; ok {
		return conf
	}
	return 1
}

// stringPtr returns pointer to string
func stringPtr(s string) *string {
	return &s
}

// int64Ptr returns pointer to int64
func int64Ptr(i int64) *int64 {
	return &i
}
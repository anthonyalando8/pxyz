// pkg/utils/assets.go
package utils

import (
	"fmt"
	"math/big"
	"strings"
	"crypto-service/internal/domain"

)

//  AssetFromChainAndCode - Explicitly specify chain
func AssetFromChainAndCode(chain, code string) *domain.Asset {
	chain = strings.ToUpper(chain)
	code = strings.ToUpper(code)

	switch chain {
	case "TRON":
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
				ContractAddr: StringPtr("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"),
				Decimals:     6,
			}
		}

	case "BITCOIN":
		if code == "BTC" {
			return &domain.Asset{
				Chain:    "BITCOIN",
				Symbol:   "BTC",
				Type:     domain.AssetTypeNative,
				Decimals: 8,
			}
		}

	case "ETHEREUM":
		switch code {
		case "ETH":
			return &domain.Asset{
				Chain:    "ETHEREUM",
				Symbol:   "ETH",
				Decimals: 18,
				Type:     domain.AssetTypeNative,
			}
		case "USDC":
			return &domain.Asset{
				Chain:        "ETHEREUM",
				Symbol:       "USDC",
				ContractAddr: StringPtr("0x07865c6E87B9F70255377e024ace6630C1Eaa37F"), // Goerli
				Decimals:     6,
				Type:         domain.AssetTypeToken,
			}
		}

	case "CIRCLE":
		//  Circle only supports USDC
		if code == "USDC" {
			return &domain.Asset{
				Chain:    "CIRCLE",
				Symbol:   "USDC",
				Type:     domain.AssetTypeToken,
				Decimals: 6,
				// No ContractAddr - Circle manages this
			}
		}
	}

	return nil
}

//  GetAssetDecimals returns decimals for asset
func GetAssetDecimals(assetCode string) int {
	decimals := map[string]int{
		"TRX":  6,
		"USDT": 6,
		"BTC":  8,
		"ETH":  18,
		"USDC": 6,
	}

	if d, ok := decimals[strings.ToUpper(assetCode)]; ok {
		return d
	}
	return 6 // default
}


//  GetRequiredConfirmations returns required confirmations per chain
func GetRequiredConfirmations(chain string) int {
	confirmations := map[string]int{
		"TRON":     19,
		"BITCOIN":  3,
		"ETHEREUM": 12,
		"CIRCLE":   12, //  Circle uses Ethereum, same confirmations
	}

	if conf, ok := confirmations[strings.ToUpper(chain)]; ok {
		return conf
	}
	return 1
}

// formatBalance formats big.Int balance to human-readable string
func FormatBalance(balance *big.Int, decimals int, asset string) string {
	if balance == nil || balance.Cmp(big.NewInt(0)) == 0 {
		return fmt.Sprintf("0 %s", asset)
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	wholePart := new(big.Int).Div(balance, divisor)
	remainder := new(big.Int).Mod(balance, divisor)

	if remainder.Cmp(big.NewInt(0)) == 0 {
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

// FormatAmount is a shorthand for formatBalance
func FormatAmount(amount *big.Int, asset string) string {
	decimals := GetAssetDecimals(asset)
	return FormatBalance(amount, decimals, asset)
}

// ParseAmount converts string amount to big.Int (smallest unit)
func ParseAmount(amountStr string, decimals int) (*big.Int, error) {
	amountStr = strings.TrimSpace(amountStr)

	// If already in smallest unit (no decimal point), parse directly
	if !strings.Contains(amountStr, ".") {
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
	if wholePart == "" {
		wholePart = "0"
	}
	decimalPart := parts[1]

	// Pad or truncate decimal part to match decimals
	if len(decimalPart) > decimals {
		decimalPart = decimalPart[:decimals] // Truncate
	} else {
		decimalPart = decimalPart + strings.Repeat("0", decimals-len(decimalPart)) // Pad
	}

	// Combine whole and decimal parts
	combined := wholePart + decimalPart

	// Remove leading zeros
	combined = strings.TrimLeft(combined, "0")
	if combined == "" {
		combined = "0"
	}

	amount, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse amount")
	}

	return amount, nil
}

// StringPtr returns pointer to string
func StringPtr(s string) *string {
	return &s
}

// Int64Ptr returns pointer to int64
func Int64Ptr(i int64) *int64 {
	return &i
}
package receiptutil

import (
	"fmt"
	"time"
	"strconv"

	"x/shared/utils/id"
)


type ReceiptGenerator struct {
	sf *id.Snowflake
}

// NewReceiptGenerator creates a generator with your existing snowflake
func NewReceiptGenerator(sf *id.Snowflake) *ReceiptGenerator {
	return &ReceiptGenerator{
		sf: sf,
	}
}

// GenerateCode creates a unique receipt code with consistent format
// Format: PREFIX-YEAR-SNOWFLAKEID
// Examples:
//   - RCP-2025-000123456789 (real account)
func (rg *ReceiptGenerator) GenerateCode(accountType string) string {
	// Generate Snowflake ID using your existing generator
	rawID := rg.sf.Generate()
	snowflakeID, _ := strconv.ParseInt(rawID, 10, 64)
	
	// Determine prefix based on account type
	prefix := "RCP"
	if accountType == "demo" || accountType == "DEMO" {
		prefix = "DEMO"
	}
	
	// Get current year for readability and easy partitioning
	year := time.Now().Year()
	
	// Format: PREFIX-YEAR-SNOWFLAKEID (zero-padded to 12 digits)
	return fmt.Sprintf("%s-%d-%012d", prefix, year, snowflakeID)
}


// GenerateCodeWithType creates a receipt code with transaction type prefix
// Format: TYPE-YEAR-SNOWFLAKEID
// Examples:
//   - DP-2025-000123456789 (deposit)
//   - WD-2025-000123456789 (withdrawal)
func (rg *ReceiptGenerator) GenerateCodeWithType(transactionType string) string {
	rawID := rg.sf.Generate()
	snowflakeID, _ := strconv.ParseInt(rawID, 10, 64)
	year := time.Now().Year()
	
	// Map transaction type to prefix
	var prefix string
	switch transactionType {
	case "deposit":
		prefix = "DP"
	case "withdrawal":
		prefix = "WD"
	case "transfer":
		prefix = "TF"
	case "conversion":
		prefix = "CV"
	case "trade":
		prefix = "TD"
	case "fee":
		prefix = "FE"
	case "commission":
		prefix = "CM"
	case "reversal":
		prefix = "RV"
	case "adjustment":
		prefix = "AD"
	default:
		prefix = "TX" // Generic transaction
	}
	
	return fmt.Sprintf("%s-%d-%012d", prefix, year, snowflakeID)
}


// GenerateCodeCombined creates a receipt code with both account type and transaction type
// Format: ACCOUNTTYPE-TXTYPE-YEAR-SNOWFLAKEID
func (rg *ReceiptGenerator) GenerateCodeCombined(accountType, transactionType string) string {
	rawID := rg.sf.Generate()
	snowflakeID, _ := strconv.ParseInt(rawID, 10, 64)
	year := time.Now().Year()
	
	// Account type prefix
	accountPrefix := "REAL"
	if accountType == "demo" || accountType == "DEMO" {
		accountPrefix = "DEMO"
	}
	
	// Transaction type prefix
	var txPrefix string
	switch transactionType {
	case "deposit":
		txPrefix = "DP"
	case "withdrawal":
		txPrefix = "WD"
	case "transfer":
		txPrefix = "TF"
	case "conversion":
		txPrefix = "CV"
	case "trade":
		txPrefix = "TD"
	case "fee":
		txPrefix = "FE"
	case "commission":
		txPrefix = "CM"
	case "reversal":
		txPrefix = "RV"
	case "adjustment":
		txPrefix = "AD"
	default:
		txPrefix = "TX"
	}
	
	return fmt.Sprintf("%s-%s-%d-%012d", accountPrefix, txPrefix, year, snowflakeID)
}

// ===============================
// Validation & Parsing
// ===============================

// ValidateCode checks if a receipt code is valid
func ValidateCode(code string) bool {
	if len(code) < 17 {
		return false
	}
	
	// Check if it contains hyphens
	hyphenCount := 0
	for _, c := range code {
		if c == '-' {
			hyphenCount++
		}
	}
	
	// Should have at least 2 hyphens (PREFIX-YEAR-ID)
	return hyphenCount >= 2
}

// ParseCode extracts components from a receipt code
func ParseCode(code string) (prefix string, year int, snowflakeID int64, err error) {
	if !ValidateCode(code) {
		err = fmt.Errorf("invalid receipt code format")
		return
	}
	
	// Simple parsing (works for most formats)
	var yearStr, idStr string
	
	// Try to parse standard format: PREFIX-YEAR-ID
	// This is a simple implementation - you can make it more robust
	parts := len(code)
	if parts >= 17 {
		// Extract last 12 digits (Snowflake ID)
		idStr = code[len(code)-12:]
		
		// Extract year (4 digits before ID)
		if len(code) >= 17 {
			yearStr = code[len(code)-17 : len(code)-13]
		}
		
		// Prefix is everything before the last hyphen before year
		prefixEnd := len(code) - 17
		if prefixEnd > 0 {
			prefix = code[:prefixEnd-1]
		}
	}
	
	// Parse year and ID
	fmt.Sscanf(yearStr, "%d", &year)
	fmt.Sscanf(idStr, "%d", &snowflakeID)
	
	return
}

// ===============================
// Backward Compatibility
// ===============================

// GenerateReceiptID maintains backward compatibility with your old format
// but uses the new consistent approach
func (rg *ReceiptGenerator) GenerateReceiptID(criteria string) string {
	return rg.GenerateCodeWithType(criteria)
}

// ===============================
// Usage Examples
// ===============================
/*

// Example 1: Simple usage (recommended)
sf := id.NewSnowflake(machineID)
generator := NewReceiptGenerator(sf)

// Generate code for real account
code := generator.GenerateCode("real")
// Output: RCP-2025-000123456789

// Generate code for demo account
code := generator.GenerateCode("demo")
// Output: DEMO-2025-000123456789

// Example 2: With transaction type
code := generator.GenerateCodeWithType("deposit")
// Output: DP-2025-000123456789

code := generator.GenerateCodeWithType("withdrawal")
// Output: WD-2025-000123456789

// Example 3: Combined format (account type + transaction type)
code := generator.GenerateCodeCombined("real", "deposit")
// Output: REAL-DP-2025-000123456789

code := generator.GenerateCodeCombined("demo", "trade")
// Output: DEMO-TD-2025-000123456789

// Example 4: Backward compatibility (uses old method name)
code := generator.GenerateReceiptID("deposit")
// Output: DP-2025-000123456789

// Example 5: Validation
isValid := ValidateCode("RCP-2025-000123456789")
// Output: true

// Example 6: Parsing
prefix, year, snowflakeID, err := ParseCode("RCP-2025-000123456789")
// Output: prefix="RCP", year=2025, snowflakeID=123456789

*/
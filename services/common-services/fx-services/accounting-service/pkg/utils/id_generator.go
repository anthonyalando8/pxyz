package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
	"x/shared/utils/id"

	"github.com/oklog/ulid/v2"
)

// AccountNumberGenerator generates unique account numbers
type AccountNumberGenerator struct {
	snowflake *id.Snowflake
	mu        sync.Mutex
	entropy   *ulid.MonotonicEntropy
}

// AccountNumberFormat defines the format for account numbers
type AccountNumberFormat string

const (
	// FormatSnowflake uses snowflake ID (19 digits)
	// Example: 1234567890123456789
	FormatSnowflake AccountNumberFormat = "SNOWFLAKE"
	
	// FormatPrefixed uses prefix + snowflake (e.g., ACC-1234567890123456789)
	// Example: ACC-1234567890123456789
	FormatPrefixed AccountNumberFormat = "PREFIXED"
	
	// FormatULID uses ULID (26 characters, sortable, URL-safe)
	// Example: 01ARZ3NDEKTSV4RRFFQ69G5FAV
	FormatULID AccountNumberFormat = "ULID"
	
	// FormatNumeric uses numeric only (16 digits for readability)
	// Example: 1234-5678-9012-3456
	FormatNumeric AccountNumberFormat = "NUMERIC"
	
	// FormatAlphanumeric uses letters + numbers (12 chars)
	// Example: A1B2C3D4E5F6
	FormatAlphanumeric AccountNumberFormat = "ALPHANUMERIC"
	
	// FormatWallet uses wallet-style address (starts with W, 32 chars)
	// Example: W1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6
	FormatWallet AccountNumberFormat = "WALLET"
)

// NewAccountNumberGenerator creates a new account number generator
func NewAccountNumberGenerator(snowflake *id.Snowflake) *AccountNumberGenerator {
	entropy := ulid.Monotonic(rand.Reader, 0)
	
	return &AccountNumberGenerator{
		snowflake: snowflake,
		entropy:   entropy,
	}
}

// Generate generates a unique account number in the specified format
func (g *AccountNumberGenerator) Generate(format AccountNumberFormat, prefix ...string) (string, error) {
	switch format {
	case FormatSnowflake:
		return g.generateSnowflake(), nil
	case FormatPrefixed:
		return g.generatePrefixed(prefix...), nil
	case FormatULID:
		return g.generateULID(), nil
	case FormatNumeric:
		return g.generateNumeric(), nil
	case FormatAlphanumeric:
		return g.generateAlphanumeric(), nil
	case FormatWallet:
		return g.generateWallet(), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// generateSnowflake generates a pure snowflake ID
// Format: 19 digits
// Example: 1234567890123456789
func (g *AccountNumberGenerator) generateSnowflake() string {
	return g.snowflake.Generate()
}

// generatePrefixed generates a prefixed account number
// Format: PREFIX-{SNOWFLAKE}
// Example: ACC-1234567890123456789
func (g *AccountNumberGenerator) generatePrefixed(prefix ...string) string {
	p := "ACC"
	if len(prefix) > 0 && prefix[0] != "" {
		p = strings.ToUpper(prefix[0])
	}
	
	snowflakeID := g.snowflake.Generate()
	return fmt.Sprintf("%s-%s", p, snowflakeID)
}

// generateULID generates a ULID-based account number
// Format: 26 characters (sortable, timestamp-based)
// Example: 01ARZ3NDEKTSV4RRFFQ69G5FAV
func (g *AccountNumberGenerator) generateULID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	id := ulid.MustNew(ulid.Timestamp(time.Now()), g.entropy)
	return id.String()
}

// generateNumeric generates a numeric-only account number with dashes
// Format: 16 digits with dashes for readability
// Example: 1234-5678-9012-3456
func (g *AccountNumberGenerator) generateNumeric() string {
	snowflakeID := g.snowflake.Generate()
	
	// Pad to 16 digits
	if len(snowflakeID) < 16 {
		snowflakeID = fmt.Sprintf("%016s", snowflakeID)
	} else if len(snowflakeID) > 16 {
		snowflakeID = snowflakeID[:16]
	}
	
	// Add dashes for readability: 1234-5678-9012-3456
	return fmt.Sprintf("%s-%s-%s-%s",
		snowflakeID[0:4],
		snowflakeID[4:8],
		snowflakeID[8:12],
		snowflakeID[12:16],
	)
}

// generateAlphanumeric generates an alphanumeric account number
// Format: 12 characters (uppercase letters + numbers, no ambiguous chars)
// Example: A1B2C3D4E5F6
func (g *AccountNumberGenerator) generateAlphanumeric() string {
	// Use snowflake for uniqueness + random chars for security
	snowflakeID := g.snowflake.Generate()
	
	// Convert snowflake to base36 (0-9, A-Z)
	snowflakeInt, _ := strconv.ParseInt(snowflakeID, 10, 64)
	base36 := strings.ToUpper(strconv.FormatInt(snowflakeInt, 36))
	
	// Pad or trim to 12 chars
	if len(base36) < 12 {
		// Add random suffix
		suffix := g.generateRandomAlphanumeric(12 - len(base36))
		return base36 + suffix
	}
	
	return base36[:12]
}

// generateWallet generates a wallet-style address
// Format: W + 31 hexadecimal characters (32 total)
// Example: W1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6
func (g *AccountNumberGenerator) generateWallet() string {
	prefix := "W"
	
	// Use full snowflake (19 digits) encoded as base36
	snowflakeID := g.snowflake.Generate()
	snowflakeInt, _ := strconv.ParseInt(snowflakeID, 10, 64)
	
	// Base36 encoding: 19 digits → ~13 chars
	snowflakeB36 := strings.ToUpper(strconv.FormatInt(snowflakeInt, 36))
	
	// Add 18 random hex chars for total 31
	randomHex := g.generateRandomHex(31 - len(snowflakeB36))
	
	return prefix + snowflakeB36 + randomHex
}
// generateRandomAlphanumeric generates random alphanumeric string
// Uses crypto/rand for security
func (g *AccountNumberGenerator) generateRandomAlphanumeric(length int) string {
	// Exclude ambiguous characters: 0, O, I, 1
	const charset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	result := make([]byte, length)
	
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	
	return string(result)
}

// generateRandomHex generates random hexadecimal string
func (g *AccountNumberGenerator) generateRandomHex(length int) string {
	bytes := make([]byte, (length+1)/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// ========================================
// SPECIALIZED GENERATORS
// ========================================

// GenerateAccountNumber generates a standard account number
// Format: ACC-{SNOWFLAKE}
func (g *AccountNumberGenerator) GenerateAccountNumber() string {
	return g.generatePrefixed("ACC")
}

// GenerateWalletAddress generates a wallet address
// Format: W{32_CHARS}
func (g *AccountNumberGenerator) GenerateWalletAddress() string {
	return g.generateWallet()
}

// GenerateSystemAccount generates a system account number
// Format: SYS-{SNOWFLAKE}
func (g *AccountNumberGenerator) GenerateSystemAccount() string {
	return g.generatePrefixed("SYS")
}

// GenerateAgentAccount generates an agent account number
// Format: AGT-{SNOWFLAKE}
func (g *AccountNumberGenerator) GenerateAgentAccount() string {
	return g.generatePrefixed("AGT")
}

// GenerateDemoAccount generates a demo/test account number
// Format: DEMO-{SNOWFLAKE}
func (g *AccountNumberGenerator) GenerateDemoAccount() string {
	return g.generatePrefixed("DEMO")
}

// ========================================
// VALIDATION
// ========================================

// ValidateFormat validates if an account number matches the expected format
func ValidateFormat(accountNumber string, format AccountNumberFormat) bool {
	switch format {
	case FormatSnowflake:
		return validateSnowflake(accountNumber)
	case FormatPrefixed:
		return validatePrefixed(accountNumber)
	case FormatULID:
		return validateULID(accountNumber)
	case FormatNumeric:
		return validateNumeric(accountNumber)
	case FormatAlphanumeric:
		return validateAlphanumeric(accountNumber)
	case FormatWallet:
		return validateWallet(accountNumber)
	default:
		return false
	}
}

func validateSnowflake(s string) bool {
	if len(s) < 18 || len(s) > 19 {
		return false
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func validatePrefixed(s string) bool {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return false
	}
	return len(parts[0]) >= 2 && validateSnowflake(parts[1])
}

func validateULID(s string) bool {
	if len(s) != 26 {
		return false
	}
	_, err := ulid.Parse(s)
	return err == nil
}

func validateNumeric(s string) bool {
	// Format: 1234-5678-9012-3456
	parts := strings.Split(s, "-")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if len(part) != 4 {
			return false
		}
		if _, err := strconv.ParseInt(part, 10, 64); err != nil {
			return false
		}
	}
	return true
}

func validateAlphanumeric(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

func validateWallet(s string) bool {
	if len(s) != 32 || s[0] != 'W' {
		return false
	}
	// Validate remaining chars are hexadecimal
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// ========================================
// CHECKSUM SUPPORT (Optional)
// ========================================

// GenerateWithChecksum generates account number with checksum
// Uses Luhn algorithm for validation
func (g *AccountNumberGenerator) GenerateWithChecksum(format AccountNumberFormat) (string, error) {
	base, err := g.Generate(format)
	if err != nil {
		return "", err
	}
	
	// Extract numeric portion
	numeric := extractNumeric(base)
	checksum := calculateLuhnChecksum(numeric)
	
	return fmt.Sprintf("%s-%d", base, checksum), nil
}

// extractNumeric extracts only numeric characters from string
func extractNumeric(s string) string {
	var result strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// calculateLuhnChecksum calculates Luhn checksum digit
func calculateLuhnChecksum(s string) int {
	sum := 0
	isEven := false
	
	// Process from right to left
	for i := len(s) - 1; i >= 0; i-- {
		digit := int(s[i] - '0')
		
		if isEven {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		
		sum += digit
		isEven = !isEven
	}
	
	return (10 - (sum % 10)) % 10
}

// ValidateLuhnChecksum validates account number with Luhn checksum
func ValidateLuhnChecksum(accountNumber string) bool {
	parts := strings.Split(accountNumber, "-")
	if len(parts) < 2 {
		return false
	}
	
	// Last part should be checksum
	checksumStr := parts[len(parts)-1]
	if len(checksumStr) != 1 {
		return false
	}
	
	expectedChecksum, err := strconv.Atoi(checksumStr)
	if err != nil {
		return false
	}
	
	// Get base without checksum
	base := strings.Join(parts[:len(parts)-1], "-")
	numeric := extractNumeric(base)
	actualChecksum := calculateLuhnChecksum(numeric)
	
	return expectedChecksum == actualChecksum
}
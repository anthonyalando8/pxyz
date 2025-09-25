package receiptutil

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"x/shared/utils/id"
)

// ---- Receipt Generator ----
type ReceiptGenerator struct {
	sf     *id.Snowflake
	prefix string
}

// NewReceiptGenerator creates a generator with given snowflake and prefix
func NewReceiptGenerator(sf *id.Snowflake, prefix string) *ReceiptGenerator {
	// Seed randomness
	rand.Seed(time.Now().UnixNano())
	return &ReceiptGenerator{
		sf:     sf,
		prefix: prefix,
	}
}

// GenerateReceiptID returns a unique, obfuscated receipt ID
func (rg *ReceiptGenerator) GenerateReceiptID(criteria string) string {
	// 1. Generate Snowflake ID
	baseID := rg.sf.Generate()

	// 2. Add some randomness (2–3 digits)
	randPart := rand.Intn(900) + 100 // ensures 3 digits (100–999)

	// 3. Pick a dynamic prefix based on criteria (e.g., type or date)
	var dynamicPrefix string
	switch strings.ToLower(criteria) {
	case "deposit":
		dynamicPrefix = "DP"
	case "withdrawal":
		dynamicPrefix = "WD"
	case "transfer":
		dynamicPrefix = "TF"
	case "conversion":
		dynamicPrefix = "CV"
	default:
		// fallback prefix based on current date (YYMM)
		dynamicPrefix = time.Now().Format("0601")
	}

	// 4. Mix all parts together → PREFIX + random + Snowflake
	receiptID := fmt.Sprintf("%s-%d-%s", dynamicPrefix, randPart, baseID)

	return receiptID
}

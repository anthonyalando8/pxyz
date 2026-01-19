// internal/domain/conversion.go
package domain

import (
	"math/big"
	"time"
)

// Conversion represents crypto ↔ USD internal wallet conversion
type Conversion struct {
	ID            int64
	UserID        string
	
	// Source
	FromWalletType WalletType  // CRYPTO or INTERNAL_USD
	FromAsset     string       // TRX, USDT, BTC, or USD
	FromAmount    *big. Int
	
	// Destination
	ToWalletType  WalletType
	ToAsset       string
	ToAmount      *big.Int
	
	// Exchange rate used
	ExchangeRate  float64      // e.g., 1 USDT = 130 KES
	
	// Fees
	PlatformFee   *big. Int     // Platform conversion fee (revenue)
	NetworkFee    *big.Int     // If crypto involved
	
	// Status
	Status        ConversionStatus
	TxHash        *string      // If blockchain tx involved
	
	CreatedAt     time.Time
	CompletedAt   *time.Time
	FailedReason  *string
}

type WalletType string

const (
	WalletTypeCrypto      WalletType = "CRYPTO"
	WalletTypeInternalUSD WalletType = "INTERNAL_USD"
)

type ConversionStatus string

const (
	ConversionStatusPending   ConversionStatus = "PENDING"
	ConversionStatusCompleted ConversionStatus = "COMPLETED"
	ConversionStatusFailed    ConversionStatus = "FAILED"
)

// ConversionQuote provides upfront conversion information
type ConversionQuote struct {
	From          string
	FromAmount    *big.Int
	FromAmountUSD float64
	
	To            string
	ToAmount      *big. Int
	ToAmountUSD   float64
	
	ExchangeRate  float64
	
	// Fees
	PlatformFee   *big.Int
	PlatformFeeUSD float64
	
	// What user receives
	NetAmount     *big.Int
	NetAmountUSD  float64
	
	QuotedAt      time.Time
	ValidUntil    time.Time
	
	Explanation   string
}
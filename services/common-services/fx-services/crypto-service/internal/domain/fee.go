// internal/domain/fee.go
package domain

import (
	"math/big"
	"time"
)

// FeeConfiguration represents platform fee rules (DB-driven)
type FeeConfiguration struct {
	ID          int64
	Chain       string      // TRON, BTC, ETH
	Asset       string      // TRX, USDT, BTC
	Operation   Operation   // DEPOSIT, WITHDRAWAL, CONVERSION
	FeeType     FeeType     // FIXED, PERCENTAGE, TIERED
	
	// Fee amounts
	FixedFee    *big.Int    // Fixed fee in smallest unit (e.g., SUN for TRX)
	PercentFee  float64     // Percentage fee (e.g., 0.5 = 0.5%)
	MinFee      *big.Int    // Minimum fee (e.g., 100 KES equivalent)
	MaxFee      *big.Int    // Maximum fee (optional cap)
	
	// Network fee markup
	NetworkFeeMarkup float64  // Markup on network fees (e.g., 1.1 = 10% markup)
	
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Operation string

const (
	OperationDeposit     Operation = "DEPOSIT"
	OperationWithdrawal  Operation = "WITHDRAWAL"
	OperationConversion  Operation = "CONVERSION"
	OperationTransfer    Operation = "TRANSFER"
)

type FeeType string

const (
	FeeTypeFixed      FeeType = "FIXED"
	FeeTypePercentage FeeType = "PERCENTAGE"
	FeeTypeTiered     FeeType = "TIERED"
	FeeTypeCombined   FeeType = "COMBINED"  // Fixed + Percentage
)

// FeeBreakdown shows detailed fee calculation
type FeeBreakdown struct {
	// Network fees (pass-through)
	NetworkFee       *big.Int  `json:"network_fee"`
	NetworkFeeCrypto string    `json:"network_fee_crypto"`    // e.g., "0.001 TRX"
	NetworkFeeUSD    float64   `json:"network_fee_usd"`
	
	// Platform fees (revenue)
	PlatformFee       *big.Int  `json:"platform_fee"`
	PlatformFeeCrypto string    `json:"platform_fee_crypto"`   // e.g., "100 KES worth"
	PlatformFeeUSD    float64   `json:"platform_fee_usd"`
	
	// Total
	TotalFee       *big.Int  `json:"total_fee"`
	TotalFeeCrypto string    `json:"total_fee_crypto"`
	TotalFeeUSD    float64   `json:"total_fee_usd"`
	
	// Breakdown explanation
	Explanation string `json:"explanation"`
}

// TransactionCostEstimate provides upfront cost information
type TransactionCostEstimate struct {
	Amount          *big.Int
	AmountCrypto    string
	
	FeeBreakdown    *FeeBreakdown
	
	TotalCost       *big.Int       // Amount + Total Fees
	TotalCostCrypto string
	TotalCostUSD    float64
	
	RequiredBalance *big.Int       // What user needs in wallet
	
	EstimatedAt     time.Time
	ValidUntil      time.Time      // Estimate expires after X minutes
}
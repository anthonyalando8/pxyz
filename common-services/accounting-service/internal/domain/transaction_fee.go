package domain

import (
	"time"
)

type TransactionFeeRule struct {
	ID              int64     `db:"id"`
	TransactionType string    `db:"transaction_type"` // e.g. transfer, conversion, withdrawal
	SourceCurrency  *string   `db:"source_currency"`  // nullable
	TargetCurrency  *string   `db:"target_currency"`  // nullable
	FeeType         string    `db:"fee_type"`         // "percentage" or "fixed"
	FeeValue        int64     `db:"fee_value"`        // stored in atomic units (bps or cents)
	MinFee          *int64    `db:"min_fee"`          // optional lower bound
	MaxFee          *int64    `db:"max_fee"`          // optional upper bound
	CreatedAt       time.Time `db:"created_at"`
}

type TransactionFee struct {
	ID          int64     `db:"id"`
	ReceiptCode string    `db:"receipt_code"` // FK → receipt_lookup.code
	FeeRuleID   *int64    `db:"fee_rule_id"`  // optional, some fees may not come from a rule
	FeeType     string    `db:"fee_type"`     // e.g. platform, network, partner
	Amount      int64     `db:"amount"`       // in atomic units
	Currency    string    `db:"currency"`
	CreatedAt   time.Time `db:"created_at"`
}

var DefaultTransactionFeeRules = []*TransactionFeeRule{
	// ===== Conversions =====
	{
		ID:              1,
		TransactionType: "conversion",
		SourceCurrency:  strPtr("BTC"),
		TargetCurrency:  strPtr("USDT"),
		FeeType:         "percentage", // 0.25%
		FeeValue:        25,
		MinFee:          intPtr(1000),   // $10
		MaxFee:          intPtr(500000), // $5000
		CreatedAt:       time.Now(),
	},
	{
		ID:              2,
		TransactionType: "conversion",
		SourceCurrency:  strPtr("USDT"),
		TargetCurrency:  strPtr("BTC"),
		FeeType:         "percentage",
		FeeValue:        25,
		MinFee:          intPtr(1000),
		MaxFee:          intPtr(500000),
		CreatedAt:       time.Now(),
	},
	{
		ID:              3,
		TransactionType: "conversion",
		SourceCurrency:  strPtr("USD"),
		TargetCurrency:  strPtr("USDT"),
		FeeType:         "percentage", // 0.10%
		FeeValue:        10,
		MinFee:          intPtr(100),
		MaxFee:          intPtr(200000),
		CreatedAt:       time.Now(),
	},
	{
		ID:              4,
		TransactionType: "conversion",
		SourceCurrency:  strPtr("USDT"),
		TargetCurrency:  strPtr("USD"),
		FeeType:         "percentage",
		FeeValue:        10,
		MinFee:          intPtr(100),
		MaxFee:          intPtr(200000),
		CreatedAt:       time.Now(),
	},

	// ===== Transfers =====
	{
		ID:              5,
		TransactionType: "transfer",
		SourceCurrency:  strPtr("BTC"),
		TargetCurrency:  strPtr("BTC"),
		FeeType:         "percentage", // 0.05%
		FeeValue:        5,
		MinFee:          nil,
		MaxFee:          intPtr(50000),
		CreatedAt:       time.Now(),
	},
	{
		ID:              6,
		TransactionType: "transfer",
		SourceCurrency:  strPtr("USDT"),
		TargetCurrency:  strPtr("USDT"),
		FeeType:         "percentage", // 0.02%
		FeeValue:        2,
		MinFee:          nil,
		MaxFee:          intPtr(20000),
		CreatedAt:       time.Now(),
	},
	{
		ID:              7,
		TransactionType: "transfer",
		SourceCurrency:  strPtr("USD"),
		TargetCurrency:  strPtr("USD"),
		FeeType:         "fixed", // Flat $2
		FeeValue:        200,
		MinFee:          nil,
		MaxFee:          nil,
		CreatedAt:       time.Now(),
	},

	// ===== Withdrawals =====
	{
		ID:              8,
		TransactionType: "withdrawal",
		SourceCurrency:  strPtr("BTC"),
		FeeType:         "fixed", // 0.001 BTC
		FeeValue:        100000,
		MinFee:          nil,
		MaxFee:          nil,
		CreatedAt:       time.Now(),
	},
	{
		ID:              9,
		TransactionType: "withdrawal",
		SourceCurrency:  strPtr("USDT"),
		FeeType:         "fixed", // 5 USDT
		FeeValue:        5000,
		MinFee:          nil,
		MaxFee:          nil,
		CreatedAt:       time.Now(),
	},
	{
		ID:              10,
		TransactionType: "withdrawal",
		SourceCurrency:  strPtr("USD"),
		FeeType:         "fixed", // $3
		FeeValue:        300,
		MinFee:          nil,
		MaxFee:          nil,
		CreatedAt:       time.Now(),
	},
}

// Helpers for nullable fields
func intPtr(v int64) *int64 {
	return &v
}

func strPtr(s string) *string {
	return &s
}
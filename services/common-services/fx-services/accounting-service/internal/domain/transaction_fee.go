package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// FeeType represents the type of fee
type FeeType string

const (
	FeeTypePlatform        FeeType = "platform"
	FeeTypeNetwork         FeeType = "network"
	FeeTypeConversion      FeeType = "conversion"
	FeeTypeWithdrawal      FeeType = "withdrawal"
	FeeTypeAgentCommission FeeType = "agent_commission"
)

// FeeCalculationMethod represents how the fee is calculated
type FeeCalculationMethod string

const (
	FeeCalculationPercentage FeeCalculationMethod = "percentage"
	FeeCalculationFixed      FeeCalculationMethod = "fixed"
	FeeCalculationTiered     FeeCalculationMethod = "tiered"
)

// TransactionFeeRule represents a fee rule configuration
type TransactionFeeRule struct {
	ID                int64                `json:"id" db:"id"`
	RuleName          string               `json:"rule_name" db:"rule_name"`
	TransactionType   TransactionType      `json:"transaction_type" db:"transaction_type"`
	SourceCurrency    *string              `json:"source_currency,omitempty" db:"source_currency"` // Max 8 chars
	TargetCurrency    *string              `json:"target_currency,omitempty" db:"target_currency"` // Max 8 chars
	AccountType       *AccountType         `json:"account_type,omitempty" db:"account_type"`       // NULL or 'real'
	OwnerType         *OwnerType           `json:"owner_type,omitempty" db:"owner_type"`
	FeeType           FeeType              `json:"fee_type" db:"fee_type"`
	CalculationMethod FeeCalculationMethod `json:"calculation_method" db:"calculation_method"`
	FeeValue          string               `json:"fee_value" db:"fee_value"`       // NUMERIC(10,6) as string
	MinFee            *float64             `json:"min_fee,omitempty" db:"min_fee"` // In smallest unit
	MaxFee            *float64             `json:"max_fee,omitempty" db:"max_fee"` // In smallest unit
	Tiers             json.RawMessage      `json:"tiers,omitempty" db:"tiers"`     // JSONB for tiered fees
	Tariffs           *string         `json:"tariffs,omitempty"`
	ValidFrom         time.Time            `json:"valid_from" db:"valid_from"`
	ValidTo           *time.Time           `json:"valid_to,omitempty" db:"valid_to"`
	IsActive          bool                 `json:"is_active" db:"is_active"`
	Priority          int                  `json:"priority" db:"priority"` // Higher = selected first
	CreatedAt         time.Time            `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at" db:"updated_at"`
}

// FeeTier represents a tier in tiered fee structure
type FeeTier struct {
	MinAmount float64  `json:"min_amount"`
	MaxAmount *float64 `json:"max_amount,omitempty"` // NULL means unlimited
	Rate      *float64  `json:"rate,omitempty"`       // Percentage rate as string
	FixedFee  *float64 `json:"fixed_fee,omitempty"`  // Fixed fee in smallest unit
}

// TransactionFee represents an applied fee
type TransactionFee struct {
	ID                   int64     `json:"id" db:"id"`
	ReceiptCode          string    `json:"receipt_code" db:"receipt_code"` // FK → receipt_lookup.code
	FeeRuleID            *int64    `json:"fee_rule_id,omitempty" db:"fee_rule_id"`
	FeeType              FeeType   `json:"fee_type" db:"fee_type"`
	Amount               float64   `json:"amount" db:"amount"`     // In smallest unit
	Currency             string    `json:"currency" db:"currency"` // Max 8 chars
	CollectedByAccountID *int64    `json:"collected_by_account_id,omitempty" db:"collected_by_account_id"`
	LedgerID             *int64    `json:"ledger_id,omitempty" db:"ledger_id"`
	AgentExternalID      *string   `json:"agent_external_id,omitempty" db:"agent_external_id"`
	CommissionRate       *string   `json:"commission_rate,omitempty" db:"commission_rate"` // NUMERIC(5,4) as string
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
}

// FeeRuleCreate represents data needed to create a new fee rule
type FeeRuleCreate struct {
	RuleName          string
	TransactionType   TransactionType
	SourceCurrency    *string
	TargetCurrency    *string
	AccountType       *AccountType
	OwnerType         *OwnerType
	FeeType           FeeType
	CalculationMethod FeeCalculationMethod
	FeeValue          string
	MinFee            *float64
	MaxFee            *float64
	Tiers             json.RawMessage
	ValidFrom         time.Time
	ValidTo           *time.Time
	IsActive          bool
	Priority          int
	Tariffs           *string         `json:"tariffs,omitempty"`
}

// FeeRuleFilter represents filter criteria for fee rule queries
type FeeRuleFilter struct {
	TransactionType *TransactionType
	SourceCurrency  *string
	TargetCurrency  *string
	AccountType     *AccountType
	OwnerType       *OwnerType
	FeeType         *FeeType
	IsActive        *bool
	ValidAt         *time.Time // Find rules valid at this time
	Limit           int
	Offset          int
}

// // FeeCalculation represents the result of a fee calculation
// type FeeCalculation struct {
// 	RuleID         *int64
// 	FeeType        FeeType
// 	Amount         float64
// 	Currency       string
// 	AppliedRate    *string // For percentage fees
// 	CalculatedFrom string  // Description of how fee was calculated
// }

// accounting-service/internal/domain/fee. go

// accounting-service/internal/domain/fee. go

type FeeCalculation struct {
	RuleID    *int64  `json:"rule_id,omitempty"`
	FeeType   FeeType `json:"fee_type"`
	
	// Platform fee
	Amount   float64 `json:"amount"`   // Platform fee in transaction currency
	Currency string  `json:"currency"` // Transaction currency (USDT, BTC, etc.)
	
	// Network fee (converted to transaction currency)
	NetworkFee  float64 `json:"network_fee"`  // Network fee (converted to transaction currency)
	
	// Network fee (original)
	NetworkFeeOriginal         float64 `json:"network_fee_original"`          // ✅ Original amount (e.g., 0.69 TRX)
	NetworkFeeOriginalCurrency string  `json:"network_fee_original_currency"` // ✅ Original currency (e.g., "TRX")
	
	// Total
	TotalFee float64 `json:"total_fee"` // ✅ Platform + Network (both in transaction currency)
	
	// Metadata
	AppliedRate    *string `json:"applied_rate,omitempty"`
	CalculatedFrom string  `json:"calculated_from"`
}

// GetTotalFee returns total fee (platform + network)
func (fc *FeeCalculation) GetTotalFee() float64 {
	return fc.  TotalFee
}

// HasNetworkFee checks if network fee is applicable
func (fc *FeeCalculation) HasNetworkFee() bool {
	return fc.NetworkFee > 0
}

// GetNetworkFeeDisplay returns human-readable network fee
func (fc *FeeCalculation) GetNetworkFeeDisplay() string {
	if ! fc.HasNetworkFee() {
		return "No network fee"
	}
	
	if fc.NetworkFeeOriginalCurrency != "" && fc.NetworkFeeOriginalCurrency != fc.Currency {
		return fmt. Sprintf("%.8f %s (from %.8f %s)",
			fc.NetworkFee,
			fc.Currency,
			fc.NetworkFeeOriginal,
			fc.NetworkFeeOriginalCurrency)
	}
	
	return fmt.Sprintf("%.8f %s", fc.NetworkFee, fc.Currency)
}

// WithdrawalFeeBreakdown contains complete withdrawal fee breakdown
type WithdrawalFeeBreakdown struct {
	Currency           string  `json:"currency"`
	Amount             float64 `json:"amount"`
	PlatformFee        float64 `json:"platform_fee"`
	NetworkFee         float64 `json:"network_fee"`
	NetworkFeeCurrency string  `json:"network_fee_currency,omitempty"`
	TotalFee           float64 `json:"total_fee"`
	Breakdown          string  `json:"breakdown"` // Human-readable explanation
}

// NetworkFeeCalculation contains network fee details
type NetworkFeeCalculation struct {
	Amount      float64       `json:"amount"`
	Currency    string        `json:"currency"`
	EstimatedAt time. Time     `json:"estimated_at"`
	ValidFor    time.Duration `json:"valid_for"`
	Explanation string        `json:"explanation"`
}

// IsValid checks if the fee rule has valid required fields
func (r *TransactionFeeRule) IsValid() bool {
	if r.RuleName == "" || r.TransactionType == "" || r.FeeType == "" {
		return false
	}
	if r.CalculationMethod == "" {
		return false
	}
	if r.SourceCurrency != nil && len(*r.SourceCurrency) > 8 {
		return false
	}
	if r.TargetCurrency != nil && len(*r.TargetCurrency) > 8 {
		return false
	}
	// Tiered method must have tiers
	if r.CalculationMethod == FeeCalculationTiered && r.Tiers == nil {
		return false
	}
	return true
}

// IsRealOnly checks if this rule only applies to real accounts
func (r *TransactionFeeRule) IsRealOnly() bool {
	return r.AccountType != nil && *r.AccountType == AccountTypeReal
}

// IsCurrentlyValid checks if the rule is valid at the current time
func (r *TransactionFeeRule) IsCurrentlyValid() bool {
	now := time.Now()
	return r.IsActive && r.ValidFrom.Before(now) && (r.ValidTo == nil || r.ValidTo.After(now))
}

// GetTiers unmarshals the tiers JSONB field
func (r *TransactionFeeRule) GetTiers() ([]FeeTier, error) {
	if r.Tiers == nil {
		return nil, nil
	}
	var tiers []FeeTier
	if err := json.Unmarshal(r.Tiers, &tiers); err != nil {
		return nil, err
	}
	return tiers, nil
}

// SetTiers marshals tiers into JSONB field
func (r *TransactionFeeRule) SetTiers(tiers []FeeTier) error {
	if tiers == nil {
		r.Tiers = nil
		return nil
	}
	bytes, err := json.Marshal(tiers)
	if err != nil {
		return err
	}
	r.Tiers = bytes
	return nil
}

// domain/fee_rule. go

// Tariff structure (amount-based pricing)
type Tariff struct {
    MinAmount         float64  `json:"min_amount"`                    // Minimum amount (inclusive)
    MaxAmount         *float64 `json:"max_amount,omitempty"`          // Maximum amount (inclusive), null = infinity
    CalculationMethod string   `json:"calculation_method"`             // ✅ NEW: "percentage" or "fixed"
    FeeBps            *float64 `json:"fee_bps,omitempty"`             // ✅ CHANGED: Optional - for percentage
    FixedFee          *float64 `json:"fixed_fee,omitempty"`           // Optional - can be used alone or added to percentage
}

// Validation constants
const (
    TariffCalculationPercentage = "percentage"
    TariffCalculationFixed      = "fixed"
)
// ✅ NEW:  GetTariffs parses the tariffs JSON
func (r *TransactionFeeRule) GetTariffs() ([]Tariff, error) {
	if r.Tariffs == nil || *r.Tariffs == "" {
		return nil, nil
	}

	var tariffs []Tariff
	err := json.Unmarshal([]byte(*r.Tariffs), &tariffs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tariffs: %w", err)
	}

	return tariffs, nil
}

// ✅ NEW: FindApplicableTariff finds the tariff for a given amount
func (r *TransactionFeeRule) FindApplicableTariff(amount float64) (*Tariff, error) {
	tariffs, err := r.GetTariffs()
	if err != nil {
		return nil, err
	}

	if len(tariffs) == 0 {
		return nil, nil // No tariffs defined
	}

	// Find matching tariff based on amount
	for _, tariff := range tariffs {
		if amount >= tariff.MinAmount {
			if tariff.MaxAmount == nil || amount <= *tariff. MaxAmount {
				return &tariff, nil
			}
		}
	}

	return nil, fmt.Errorf("no tariff found for amount: %.2f", amount)
}


// DefaultTransactionFeeRules returns realistic fee rules matching the schema
func DefaultTransactionFeeRules() []*TransactionFeeRule {
	now := time.Now()
	realAccountType := AccountTypeReal

	return []*TransactionFeeRule{
		// ===== DEPOSIT FEES =====
		{
			RuleName:          "Standard USD Deposit Fee",
			TransactionType:   TransactionTypeDeposit,
			SourceCurrency:    strPtr("USD"),
			AccountType:       accountTypePtr(realAccountType),
			FeeType:           FeeTypePlatform,
			CalculationMethod: FeeCalculationPercentage,
			FeeValue:          "0.001",
			MinFee:            float64Ptr(1.00),    // ✅ $1.00
			MaxFee:            float64Ptr(500.00),  // ✅ $500.00
			ValidFrom:         now,
			IsActive:          true,
			Priority:          1,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			RuleName:          "BTC Deposit Network Fee",
			TransactionType:   TransactionTypeDeposit,
			SourceCurrency:    strPtr("BTC"),
			AccountType:       &realAccountType,
			FeeType:           FeeTypeNetwork,
			CalculationMethod: FeeCalculationFixed,
			FeeValue:          "0",
			MinFee:            float64Ptr(0.005),  // ✅ 0.005 BTC (500,000 satoshis)
			ValidFrom:         now,
			IsActive:          true,
			Priority:          1,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// ===== WITHDRAWAL FEES =====
		{
			RuleName:          "USD Withdrawal Fee",
			TransactionType:   TransactionTypeWithdrawal,
			SourceCurrency:    strPtr("USD"),
			AccountType:       &realAccountType,
			FeeType:           FeeTypePlatform,
			CalculationMethod: FeeCalculationFixed,
			FeeValue:          "0",
			MinFee:            float64Ptr(2.00),  // ✅ $2.00
			ValidFrom:         now,
			IsActive:          true,
			Priority:          1,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			RuleName:          "BTC Withdrawal Fee",
			TransactionType:   TransactionTypeWithdrawal,
			SourceCurrency:    strPtr("BTC"),
			AccountType:       &realAccountType,
			FeeType:           FeeTypeNetwork,
			CalculationMethod: FeeCalculationPercentage,
			FeeValue:          "0.0005",
			MinFee:            float64Ptr(0.0005),  // ✅ 0.0005 BTC
			MaxFee:            float64Ptr(0.005),   // ✅ 0. 005 BTC
			ValidFrom:         now,
			IsActive:          true,
			Priority:          1,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// ===== CONVERSION FEES =====
		{
			RuleName:          "USD to USDT Conversion",
			TransactionType:   TransactionTypeConversion,
			SourceCurrency:    strPtr("USD"),
			TargetCurrency:    strPtr("USDT"),
			AccountType:       &realAccountType,
			FeeType:           FeeTypeConversion,
			CalculationMethod: FeeCalculationPercentage,
			FeeValue:          "0.003",
			MinFee:            float64Ptr(0.50),   // ✅ $0.50
			MaxFee:            float64Ptr(50.00),  // ✅ $50.00
			ValidFrom:         now,
			IsActive:          true,
			Priority:          1,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			RuleName:          "USD to BTC Conversion",
			TransactionType:   TransactionTypeConversion,
			SourceCurrency:    strPtr("USD"),
			TargetCurrency:    strPtr("BTC"),
			AccountType:       &realAccountType,
			FeeType:           FeeTypeConversion,
			CalculationMethod: FeeCalculationPercentage,
			FeeValue:          "0.005",
			MinFee:            float64Ptr(1.00),    // ✅ $1. 00
			MaxFee:            float64Ptr(500.00),  // ✅ $500.00
			ValidFrom:         now,
			IsActive:          true,
			Priority:          1,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// ===== TRADE FEES =====
		{
			RuleName:          "Standard Trading Fee",
			TransactionType:   TransactionTypeTrade,
			AccountType:       &realAccountType,
			FeeType:           FeeTypePlatform,
			CalculationMethod: FeeCalculationPercentage,
			FeeValue:          "0.002",
			MinFee:            float64Ptr(0.50),    // ✅ $0. 50
			MaxFee:            float64Ptr(100.00),  // ✅ $100.00
			ValidFrom:         now,
			IsActive:          true,
			Priority:          1,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// ===== TRANSFER FEES =====
		{
			RuleName:          "P2P Transfer Fee USD",
			TransactionType:   TransactionTypeTransfer,
			SourceCurrency:    strPtr("USD"),
			AccountType:       &realAccountType,
			FeeType:           FeeTypePlatform,
			CalculationMethod: FeeCalculationFixed,
			FeeValue:          "0",
			MinFee:            float64Ptr(0.50),  // ✅ $0. 50
			ValidFrom:         now,
			IsActive:          true,
			Priority:          2,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}
}

// Helper functions for nullable fields

func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func strPtr(s string) *string {
	return &s
}

func accountTypePtr(a AccountType) *AccountType {
	return &a
}

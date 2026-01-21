package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	cryptoclient "x/shared/common/crypto"

	//"math/big"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"

	//xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// TransactionFeeCalculator handles business logic for transaction fees
// TransactionFeeCalculator handles business logic for transaction fees
type TransactionFeeCalculator struct {
	feeRepo      repository.TransactionFeeRepository
	feeRuleRepo  repository.TransactionFeeRuleRepository
	redisClient  *redis.Client
	cryptoClient *cryptoclient. CryptoClient // ✅ Add crypto client
}

// NewTransactionFeeCalculator initializes a new TransactionFeeCalculator
func NewTransactionFeeCalculator(
	feeRepo repository.TransactionFeeRepository,
	feeRuleRepo repository.TransactionFeeRuleRepository,
	redisClient *redis.Client,
	cryptoClient *cryptoclient. CryptoClient, // ✅ Add parameter
) *TransactionFeeCalculator {
	return &TransactionFeeCalculator{
		feeRepo:      feeRepo,
		feeRuleRepo:   feeRuleRepo,
		redisClient:  redisClient,
		cryptoClient:  cryptoClient, // ✅ Store it
	}
}

// BeginTx starts a db transaction
func (uc *TransactionFeeCalculator) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.feeRepo.BeginTx(ctx)
}


// ===============================
// FEE CALCULATION
// ===============================

// CalculateFee is the MAIN entry point for all fee calculations
// It automatically handles: 
// - Platform fees (from rules)
// - Network fees (for crypto withdrawals)
// - Returns unified breakdown
func (uc *TransactionFeeCalculator) CalculateFee(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
	// ✅ NEW: Optional parameters for crypto withdrawals
	toAddress *string, // Destination address (for network fee estimation)
) (*domain.FeeCalculation, error) {
	
	// Validate inputs
	if sourceCurrency == nil {
		return nil, fmt.Errorf("source currency is required")
	}

	currency := *sourceCurrency

	// 1. Calculate platform fee using rules
	platformFee, err := uc.calculatePlatformFee(
		ctx,
		transactionType,
		amount,
		sourceCurrency,
		targetCurrency,
		accountType,
		ownerType,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate platform fee: %w", err)
	}

	// 2. Check if we need network fee (only for crypto withdrawals)
	needsNetworkFee := transactionType == domain.TransactionTypeWithdrawal && 
		uc.isCryptoCurrency(currency)

	// Start with platform fee
	totalFee := platformFee. Amount
	breakdown := platformFee. CalculatedFrom
	networkFeeCurrency := ""
	var networkFeeAmount float64

	// 3. Add network fee if applicable
	if needsNetworkFee && uc.cryptoClient != nil {
		destAddress := ""
		if toAddress != nil {
			destAddress = *toAddress
		}

		networkFeeCalc, err := uc.getNetworkFee(ctx, currency, amount, destAddress)
		if err != nil {
			// Log warning but don't fail - use estimate
			fmt.Printf("⚠️  Failed to get network fee for %s:  %v, using estimate\n", currency, err)
			networkFeeCalc = uc.getEstimatedNetworkFee(currency)
		}

		if networkFeeCalc != nil {
			networkFeeAmount = networkFeeCalc.Amount
			networkFeeCurrency = networkFeeCalc.Currency
			
			// For crypto, network fee is in native currency (TRX, BTC, etc.)
			// Platform fee is in transaction currency (USDT, etc.)
			// We keep them separate but report both
			
			breakdown += fmt.Sprintf(" | network fee: %.8f %s (%s)", 
				networkFeeCalc.Amount, 
				networkFeeCalc.Currency,
				networkFeeCalc. Explanation)
		}
	}

	// 4. Round total fee
	totalFee = math.Round(totalFee*100000000) / 100000000
	networkFeeAmount = math.Round(networkFeeAmount*100000000) / 100000000

	// 5. Return unified calculation
	result := &domain.FeeCalculation{
		RuleID:             platformFee.RuleID,
		FeeType:            platformFee. FeeType,
		Amount:              totalFee,          // Platform fee (in transaction currency)
		Currency:           currency,
		NetworkFee:         networkFeeAmount,  // ✅ Network fee (in native currency)
		NetworkFeeCurrency: networkFeeCurrency, // ✅ Network fee currency
		AppliedRate:        platformFee.AppliedRate,
		CalculatedFrom:     breakdown,
	}

	return result, nil
}

// ===============================
// INTERNAL HELPER:  PLATFORM FEE
// ===============================

// calculatePlatformFee handles platform fee calculation using rules
func (uc *TransactionFeeCalculator) calculatePlatformFee(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain. AccountType,
	ownerType *domain.OwnerType,
) (*domain.FeeCalculation, error) {
	
	// Try cache first
	cacheKey := uc.buildFeeCalculationCacheKey(transactionType, sourceCurrency, targetCurrency, accountType, ownerType)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var cachedRule domain.TransactionFeeRule
		if jsonErr := json.Unmarshal([]byte(val), &cachedRule); jsonErr == nil {
			return uc.calculateFeeFromRule(&cachedRule, amount)
		}
	}

	// Find best matching rule
	rule, err := uc.feeRuleRepo.FindBestMatch(ctx, transactionType, sourceCurrency, targetCurrency, accountType, ownerType)
	if err != nil {
		return nil, fmt. Errorf("no fee rule found: %w", err)
	}

	if rule == nil {
		// No rule found = no fee
		return &domain.FeeCalculation{
			FeeType:        domain.FeeTypePlatform,
			Amount:          0,
			Currency:       ptrStrToStr(sourceCurrency),
			CalculatedFrom: "no matching rule",
		}, nil
	}

	// Cache the rule (5 minutes)
	if data, err := json.Marshal(rule); err == nil {
		_ = uc.redisClient. Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	// Calculate fee from rule
	return uc.calculateFeeFromRule(rule, amount)
}

// ===============================
// BACKWARD COMPATIBILITY
// ===============================

// CalculateMultipleFees calculates all applicable fees for a transaction
func (uc *TransactionFeeCalculator) CalculateMultipleFees(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) ([]*domain.FeeCalculation, error) {
	
	// Use unified CalculateFee
	mainFee, err := uc. CalculateFee(ctx, transactionType, amount, sourceCurrency, targetCurrency, accountType, ownerType, nil)
	if err != nil {
		return nil, err
	}

	return []*domain.FeeCalculation{mainFee}, nil
}
// service/fee_calculator.go

func (uc *TransactionFeeCalculator) calculateFeeFromRule(rule *domain.TransactionFeeRule, amount float64) (*domain.FeeCalculation, error) {
	calc := &domain.FeeCalculation{
		RuleID:  &rule.ID,
		FeeType: rule.FeeType,
	}

	if rule.TargetCurrency != nil && *rule.TargetCurrency != "" {
		calc.Currency = *rule.TargetCurrency
	} else {
		calc.Currency = ptrStrToStr(rule.SourceCurrency)
	}

	var feeAmount float64

	// ✅ NEW: Check if rule has tariffs (amount-based pricing)
	if rule.Tariffs != nil && *rule.Tariffs != "" {
		return uc.calculateFeeFromTariffs(rule, amount, calc)
	}

	// ✅ Existing calculation methods (percentage, fixed, tiered)
	switch rule.CalculationMethod {
	case domain.FeeCalculationPercentage:
		basisPoints, err := strconv.ParseFloat(rule.FeeValue, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid fee value (basis points expected): %s", rule.FeeValue)
		}

		if basisPoints < 0 || basisPoints > 10000 {
			return nil, fmt.Errorf("basis points out of range (0-10000): %.2f", basisPoints)
		}

		feeRate := basisPoints / 10000.0
		feeAmount = amount * feeRate

		calc.AppliedRate = ptrStr(fmt.Sprintf("%.4f%%", feeRate)) //&rule.FeeValue
		calc.CalculatedFrom = fmt.Sprintf("percentage: %.4f%% (%s bps)", feeRate*100, rule.FeeValue)

	case domain.FeeCalculationFixed:
		fixedAmount, err := strconv.ParseFloat(rule.FeeValue, 64)
		if err != nil {
			if rule.MinFee != nil {
				feeAmount = *rule.MinFee
			} else {
				return nil, fmt.Errorf("invalid fixed fee value: %s", rule.FeeValue)
			}
		} else {
			feeAmount = fixedAmount
		}

		calc.CalculatedFrom = fmt.Sprintf("fixed fee: %.2f", feeAmount)

	case domain.FeeCalculationTiered:
		tiers, err := rule.GetTiers()
		if err != nil {
			return nil, fmt.Errorf("failed to parse tiers: %w", err)
		}

		tierFound := false
		for _, tier := range tiers {
			inRange := amount >= tier.MinAmount
			if tier.MaxAmount != nil {
				inRange = inRange && amount <= *tier.MaxAmount
			}

			if inRange {
				tierFound = true

				if tier.Rate != nil {
					basisPoints, err := strconv.ParseFloat(*tier.Rate, 64)
					if err != nil {
						return nil, fmt.Errorf("invalid tier rate:  %s", *tier.Rate)
					}

					if basisPoints < 0 || basisPoints > 10000 {
						return nil, fmt.Errorf("tier basis points out of range:  %.2f", basisPoints)
					}

					feeRate := basisPoints / 10000.0
					feeAmount += amount * feeRate

					calc.AppliedRate = tier.Rate

					maxAmountStr := "∞"
					if tier.MaxAmount != nil {
						maxAmountStr = fmt.Sprintf("%.2f", *tier.MaxAmount)
					}
					calc.CalculatedFrom = fmt.Sprintf("tiered rate: %.4f%% (%s bps) for range %.2f-%s",
						feeRate*100, *tier.Rate, tier.MinAmount, maxAmountStr)
				}

				if tier.FixedFee != nil {
					feeAmount += *tier.FixedFee
					if calc.CalculatedFrom != "" {
						calc.CalculatedFrom += fmt.Sprintf(" + fixed:  %.2f", *tier.FixedFee)
					}
				}

				break
			}
		}

		if !tierFound {
			return nil, fmt.Errorf("no tier found for amount: %.2f", amount)
		}

	default:
		return nil, fmt.Errorf("unsupported calculation method: %s", rule.CalculationMethod)
	}

	// Apply min/max limits
	originalFee := feeAmount

	if rule.MinFee != nil && feeAmount < *rule.MinFee {
		feeAmount = *rule.MinFee
		calc.CalculatedFrom += fmt.Sprintf(" → min %.2f applied (was %.2f)", *rule.MinFee, originalFee)
	}

	if rule.MaxFee != nil && feeAmount > *rule.MaxFee {
		feeAmount = *rule.MaxFee
		calc.CalculatedFrom += fmt.Sprintf(" → max %.2f applied (was %.2f)", *rule.MaxFee, originalFee)
	}

	feeAmount = math.Round(feeAmount*100) / 100
	calc.Amount = feeAmount

	return calc, nil
}

// ✅ NEW: Calculate fee using tariffs (amount-based pricing)
// service/fee_calculator.go

// ✅ UPDATED: Calculate fee using tariffs (supports both percentage and fixed)
func (uc *TransactionFeeCalculator) calculateFeeFromTariffs(
	rule *domain.TransactionFeeRule,
	amount float64,
	calc *domain.FeeCalculation,
) (*domain.FeeCalculation, error) {
	// Find applicable tariff for this amount
	tariff, err := rule.FindApplicableTariff(amount)
	if err != nil {
		return nil, err
	}

	if tariff == nil {
		return nil, fmt.Errorf("no tariff found for amount:  %.2f", amount)
	}

	var feeAmount float64
	
	switch tariff.CalculationMethod {
	case domain.TariffCalculationPercentage:
		// Percentage-based fee
		if tariff.FeeBps == nil {
			return nil, fmt.Errorf("fee_bps is required for percentage calculation method")
		}

		if *tariff.FeeBps < 0 || *tariff.FeeBps > 10000 {
			return nil, fmt.Errorf("tariff fee_bps out of range (0-10000): %.2f", *tariff.FeeBps)
		}

		feeRate := *tariff.FeeBps / 10000.0
		feeAmount = amount * feeRate

		// Add fixed fee if present (combination)
		if tariff.FixedFee != nil {
			feeAmount += *tariff.FixedFee
		}

		// Build description
		maxAmountStr := "∞"
		if tariff.MaxAmount != nil {
			maxAmountStr = fmt.Sprintf("%.2f", *tariff.MaxAmount)
		}

		feeBpsStr := fmt.Sprintf("%.0f", *tariff.FeeBps)
		calc.AppliedRate = &feeBpsStr
		calc.CalculatedFrom = fmt.Sprintf("tariff percentage:  %.4f%% (%.0f bps) for $%.2f-$%s",
			feeRate*100, *tariff.FeeBps, tariff.MinAmount, maxAmountStr)

		if tariff.FixedFee != nil {
			calc.CalculatedFrom += fmt.Sprintf(" + fixed: $%.2f", *tariff.FixedFee)
		}

	case domain.TariffCalculationFixed:
		// Fixed fee only
		if tariff.FixedFee == nil {
			return nil, fmt.Errorf("fixed_fee is required for fixed calculation method")
		}

		feeAmount = *tariff.FixedFee

		// Build description
		maxAmountStr := "∞"
		if tariff.MaxAmount != nil {
			maxAmountStr = fmt.Sprintf("%.2f", *tariff.MaxAmount)
		}

		calc.CalculatedFrom = fmt.Sprintf("tariff fixed: $%.2f for $%.2f-$%s",
			*tariff.FixedFee, tariff.MinAmount, maxAmountStr)

	default:
		return nil, fmt.Errorf("unsupported tariff calculation method: %s", tariff.CalculationMethod)
	}

	// Apply min/max limits (from rule-level)
	originalFee := feeAmount

	if rule.MinFee != nil && feeAmount < *rule.MinFee {
		feeAmount = *rule.MinFee
		calc.CalculatedFrom += fmt.Sprintf(" → min $%.2f applied (was $%.2f)", *rule.MinFee, originalFee)
	}

	if rule.MaxFee != nil && feeAmount > *rule.MaxFee {
		feeAmount = *rule.MaxFee
		calc.CalculatedFrom += fmt.Sprintf(" → max $%.2f applied (was $%.2f)", *rule.MaxFee, originalFee)
	}

	// Round to 2 decimal places
	feeAmount = math.Round(feeAmount*100) / 100
	calc.Amount = feeAmount

	return calc, nil
}

// ===============================
// CACHE MANAGEMENT
// ===============================


// buildFeeCalculationCacheKey builds cache key for fee calculation
func (uc *TransactionFeeCalculator) buildFeeCalculationCacheKey(
	transactionType domain.TransactionType,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) string {
	key := fmt.Sprintf("fee:calc:%s", transactionType)

	if sourceCurrency != nil {
		key += fmt.Sprintf(":%s", *sourceCurrency)
	} else {
		key += ":nil"
	}

	if targetCurrency != nil {
		key += fmt.Sprintf(":%s", *targetCurrency)
	} else {
		key += ":nil"
	}

	if accountType != nil {
		key += fmt.Sprintf(":%s", *accountType)
	} else {
		key += ":nil"
	}

	if ownerType != nil {
		key += fmt.Sprintf(":%s", *ownerType)
	} else {
		key += ":nil"
	}

	return key
}

func ptrStrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptrStr(s string) *string {
	return &s
}
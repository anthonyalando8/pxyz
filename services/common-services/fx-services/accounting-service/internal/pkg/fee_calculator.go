package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	//"math/big"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"

	//xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// TransactionFeeCalculator handles business logic for transaction fees
type TransactionFeeCalculator struct {
	feeRepo     repository.TransactionFeeRepository
	feeRuleRepo repository.TransactionFeeRuleRepository
	redisClient *redis.Client
}

// NewTransactionFeeCalculator initializes a new TransactionFeeCalculator
func NewTransactionFeeCalculator(
	feeRepo repository.TransactionFeeRepository,
	feeRuleRepo repository.TransactionFeeRuleRepository,
	redisClient *redis.Client,
) *TransactionFeeCalculator {
	return &TransactionFeeCalculator{
		feeRepo:     feeRepo,
		feeRuleRepo: feeRuleRepo,
		redisClient: redisClient,
	}
}

// BeginTx starts a db transaction
func (uc *TransactionFeeCalculator) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.feeRepo.BeginTx(ctx)
}


// ===============================
// FEE CALCULATION
// ===============================

// CalculateFee calculates the fee for a transaction based on active rules
func (uc *TransactionFeeCalculator) CalculateFee(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) (*domain.FeeCalculation, error) {
	// Try cache first (fee rules are relatively stable)
	cacheKey := uc.buildFeeCalculationCacheKey(transactionType, sourceCurrency, targetCurrency, accountType, ownerType)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var cachedRule domain.TransactionFeeRule
		if jsonErr := json.Unmarshal([]byte(val), &cachedRule); jsonErr == nil {
			// Recalculate with cached rule
			return uc.calculateFeeFromRule(&cachedRule, amount)
		}
	}

	// Find best matching rule
	rule, err := uc.feeRuleRepo.FindBestMatch(ctx, transactionType, sourceCurrency, targetCurrency, accountType, ownerType)
	if err != nil {
		return nil, fmt.Errorf("no fee rule found: %w", err)
	}

	if rule == nil {
		// No rule found = no fee
		return &domain.FeeCalculation{
			FeeType:        domain.FeeTypePlatform,
			Amount:         0,
			Currency:       *sourceCurrency,
			CalculatedFrom: "no matching rule",
		}, nil
	}

	// Cache the rule (5 minutes)
	if data, err := json.Marshal(rule); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	// Calculate fee from rule
	return uc.calculateFeeFromRule(rule, amount)
}

// CalculateMultipleFees calculates all applicable fees for a transaction
func (uc *TransactionFeeCalculator) CalculateMultipleFees(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) ([]*domain.FeeCalculation, error) {
	// Find all matching rules
	rules, err := uc.feeRuleRepo.FindAllMatches(ctx, transactionType, sourceCurrency, targetCurrency, accountType, ownerType)
	if err != nil {
		return nil, fmt.Errorf("failed to find fee rules: %w", err)
	}

	if len(rules) == 0 {
		return []*domain.FeeCalculation{}, nil
	}

	// Calculate fee for each rule
	var calculations []*domain.FeeCalculation

	for _, rule := range rules {
		calc, err := uc.calculateFeeFromRule(rule, amount)
		if err != nil {
			// Log error but continue
			continue
		}

		if calc.Amount > 0 {
			calculations = append(calculations, calc)
		}
	}

	return calculations, nil
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

		calc.AppliedRate = &rule.FeeValue
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
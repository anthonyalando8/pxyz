package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
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

// calculateFeeFromRule calculates fee based on a specific rule
func (uc *TransactionFeeCalculator) calculateFeeFromRule(rule *domain.TransactionFeeRule, amount float64) (*domain.FeeCalculation, error) {
	calc := &domain.FeeCalculation{
		RuleID:  &rule.ID,
		FeeType: rule.FeeType,
	}
	if rule.TargetCurrency != nil && *rule.TargetCurrency != "" {
		calc.Currency = *rule.TargetCurrency
	}else {
		calc.Currency = ptrStrToStr(rule.SourceCurrency)
	}

	var feeAmount float64

	switch rule.CalculationMethod {
	case domain.FeeCalculationPercentage:
	// Parse fee value as decimal
		feeRate := new(big.Float)
		if _, ok := feeRate.SetString(rule.FeeValue); !ok {
			return nil, fmt.Errorf("invalid fee value: %s", rule.FeeValue)
		}

		// ✅ Calculate: amount * rate (both float64)
		amountFloat := new(big.Float).SetFloat64(amount)  // ✅ Use SetFloat64
		feeFloat := new(big.Float).Mul(amountFloat, feeRate)

		// ✅ Convert back to float64
		feeAmount, _ = feeFloat.Float64()  // ✅ Direct float64

		calc.AppliedRate = &rule.FeeValue
		calc.CalculatedFrom = fmt.Sprintf("percentage: %s", rule.FeeValue)

	case domain.FeeCalculationFixed:
		// Fixed fee - just use min_fee
		if rule.MinFee != nil {
			feeAmount = *rule.MinFee
		} else {
			feeAmount = 0
		}
		calc.CalculatedFrom = "fixed fee"

	case domain.FeeCalculationTiered:
		// Tiered fee - find applicable tier
		tiers, err := rule.GetTiers()
		if err != nil {
			return nil, fmt.Errorf("failed to parse tiers: %w", err)
		}

		for _, tier := range tiers {
			// Check if amount falls in this tier
			if amount >= tier.MinAmount && (tier.MaxAmount == nil || amount <= *tier.MaxAmount) {
				if tier.Rate != nil {
					// Apply tier rate
					feeRate := new(big.Float)
					if _, ok := feeRate.SetString(*tier.Rate); !ok {
						return nil, fmt.Errorf("invalid tier rate: %s", *tier. Rate)
					}

					// ✅ Use SetFloat64 instead of SetInt64
					amountFloat := new(big.Float).SetFloat64(amount)
					feeFloat := new(big.Float).Mul(amountFloat, feeRate)
					
					// ✅ Convert to float64 instead of int64
					feeAmount, _ = feeFloat.Float64()

					calc.AppliedRate = tier.Rate
					calc. CalculatedFrom = fmt.Sprintf("tiered rate: %s (amount: %.2f-%.2f)",
						*tier.Rate, tier.MinAmount, *tier.MaxAmount)  // ✅ Use %. 2f for float
				}

				if tier.FixedFee != nil {
					feeAmount += *tier.FixedFee
				}

				break
			}
		}

	default:
		return nil, fmt.Errorf("unsupported calculation method: %s", rule.CalculationMethod)
	}

	// Apply min/max limits
	if rule.MinFee != nil && feeAmount < *rule.MinFee {
		feeAmount = *rule.MinFee
		calc.CalculatedFrom += " (min applied)"
	}

	if rule.MaxFee != nil && feeAmount > *rule.MaxFee {
		feeAmount = *rule.MaxFee
		calc.CalculatedFrom += " (max applied)"
	}

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
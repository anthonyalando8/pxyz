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
	currencyRepo repository.CurrencyRepository //  Add currency repo for FX rates
	redisClient  *redis.Client
	cryptoClient *cryptoclient.CryptoClient
}

func NewTransactionFeeCalculator(
	feeRepo repository.TransactionFeeRepository,
	feeRuleRepo repository.TransactionFeeRuleRepository,
	currencyRepo repository.CurrencyRepository, //  Add parameter
	redisClient *redis.Client,
	cryptoClient *cryptoclient.CryptoClient,
) *TransactionFeeCalculator {
	return &TransactionFeeCalculator{
		feeRepo:      feeRepo,
		feeRuleRepo:  feeRuleRepo,
		currencyRepo: currencyRepo, //  Store it
		redisClient:  redisClient,
		cryptoClient: cryptoClient,
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
// CalculateFee - UPDATED to handle cross-currency network fees
func (uc *TransactionFeeCalculator) CalculateFee(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
	toAddress *string,
) (*domain.FeeCalculation, error) {

	if sourceCurrency == nil {
		return nil, fmt.Errorf("source currency is required")
	}

	currency := *sourceCurrency

	// 1. Calculate platform fee
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

	// 2. Check if we need network fee
	needsNetworkFee := transactionType == domain.TransactionTypeWithdrawal &&
		uc.isCryptoCurrency(currency)

	// Start with platform fee
	totalFee := platformFee.Amount
	breakdown := platformFee.CalculatedFrom

	var networkFeeInSourceCurrency float64
	var networkFeeOriginal float64
	var networkFeeOriginalCurrency string

	// 3. Add network fee if applicable
	if needsNetworkFee && uc.cryptoClient != nil {
		destAddress := ""
		if toAddress != nil {
			destAddress = *toAddress
		}

		// Get network fee from crypto service
		networkFeeCalc, err := uc.getNetworkFee(ctx, currency, amount, destAddress)
		if err != nil {
			fmt.Printf("⚠️  Failed to get network fee for %s: %v, using estimate\n", currency, err)
			networkFeeCalc = uc.getEstimatedNetworkFee(currency)
		}

		if networkFeeCalc != nil {
			networkFeeOriginal = networkFeeCalc.Amount
			networkFeeOriginalCurrency = networkFeeCalc.Currency

			//  Convert network fee to source currency if needed
			if networkFeeCalc.Currency != currency {
				converted, err := uc.convertCurrency(
					ctx,
					networkFeeCalc.Amount,
					networkFeeCalc.Currency,
					currency,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to convert network fee from %s to %s: %w",
						networkFeeCalc.Currency, currency, err)
				}
				networkFeeInSourceCurrency = converted

				breakdown += fmt.Sprintf(" | network fee: %.8f %s (%.8f %s converted)",
					networkFeeOriginal,
					networkFeeOriginalCurrency,
					networkFeeInSourceCurrency,
					currency)
			} else {
				// Same currency - no conversion needed
				networkFeeInSourceCurrency = networkFeeCalc.Amount

				breakdown += fmt.Sprintf(" | network fee: %.8f %s",
					networkFeeInSourceCurrency,
					currency)
			}

			//  Add converted network fee to total (now in same currency)
			totalFee += networkFeeInSourceCurrency
		}
	}

	// 4. Round total fee
	totalFee = math.Round(totalFee*100000000) / 100000000
	networkFeeInSourceCurrency = math.Round(networkFeeInSourceCurrency*100000000) / 100000000

	// 5. Return unified calculation
	result := &domain.FeeCalculation{
		RuleID:                     platformFee.RuleID,
		FeeType:                    platformFee.FeeType,
		Amount:                     platformFee.Amount, // Platform fee only
		Currency:                   currency,
		NetworkFee:                 networkFeeInSourceCurrency, //  Network fee (converted)
		NetworkFeeOriginal:         networkFeeOriginal,         //  Original network fee amount
		NetworkFeeOriginalCurrency: networkFeeOriginalCurrency, //  Original currency (TRX, BTC, etc.)
		TotalFee:                   totalFee,                   //  Platform + Network (both in source currency)
		AppliedRate:                platformFee.AppliedRate,
		CalculatedFrom:             breakdown,
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
	accountType *domain.AccountType,
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
		return nil, fmt.Errorf("no fee rule found: %w", err)
	}

	if rule == nil {
		// No rule found = no fee
		return &domain.FeeCalculation{
			FeeType:        domain.FeeTypePlatform,
			Amount:         0,
			Currency:       ptrStrToStr(sourceCurrency),
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
	mainFee, err := uc.CalculateFee(ctx, transactionType, amount, sourceCurrency, targetCurrency, accountType, ownerType, nil)
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

	//  NEW: Check if rule has tariffs (amount-based pricing)
	if rule.Tariffs != nil && *rule.Tariffs != "" {
		return uc.calculateFeeFromTariffs(rule, amount, calc)
	}

	//  Existing calculation methods (percentage, fixed, tiered)
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

	// accounting-service/internal/service/fee_calculator.go

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
					basisPoints := *tier.Rate // Already float64
					
					if basisPoints < 0 || basisPoints > 10000 {
						return nil, fmt.Errorf("tier basis points out of range: %.2f", basisPoints)
					}

					feeRate := basisPoints / 10000.0
					feeAmount += amount * feeRate

					//  FIX: Show the actual basis points, not fee rate
					rateStr := fmt.Sprintf("%.0f", basisPoints) // "100" not "0.01"
					calc.AppliedRate = &rateStr

					maxAmountStr := "∞"
					if tier.MaxAmount != nil {
						maxAmountStr = fmt.Sprintf("%.2f", *tier.MaxAmount)
					}
					
					//  FIX:  Show correct percentage calculation
					calc. CalculatedFrom = fmt.Sprintf("tiered rate: %.4f%% (%.2f bps) for range %.2f-%s",
						feeRate*100,  //  This converts 0.01 → 1.00%
						basisPoints,  //  Shows "100 bps"
						tier.MinAmount, 
						maxAmountStr)
				}

				if tier.FixedFee != nil {
					feeAmount += *tier.FixedFee
					if calc. CalculatedFrom != "" {
						calc.CalculatedFrom += fmt.Sprintf(" + fixed:  %.8f", *tier.FixedFee)
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

//  NEW: Calculate fee using tariffs (amount-based pricing)
// service/fee_calculator.go

//  UPDATED: Calculate fee using tariffs (supports both percentage and fixed)
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

func (uc *TransactionFeeCalculator) convertCurrency(
	ctx context.Context,
	amount float64,
	fromCurrency, toCurrency string,
) (float64, error) {

	// Check if conversion needed
	if fromCurrency == toCurrency {
		return amount, nil
	}

	// Try cache first
	cacheKey := fmt.Sprintf("fx: rate:%s:%s", fromCurrency, toCurrency)
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var rate float64
		if _, parseErr := fmt.Sscanf(val, "%f", &rate); parseErr == nil {
			return amount * rate, nil
		}
	}

	// Get FX rate from database
	fxRate, err := uc.currencyRepo.GetCurrentFXRate(ctx, fromCurrency, toCurrency)
	if err != nil {
		// Try inverse rate
		inverseFxRate, inverseErr := uc.currencyRepo.GetCurrentFXRate(ctx, toCurrency, fromCurrency)
		if inverseErr != nil {
			return 0, fmt.Errorf("no FX rate found for %s/%s", fromCurrency, toCurrency)
		}

		// Use inverse rate:  1 / rate
		inverseFxRateValue, err := strconv.ParseFloat(inverseFxRate.Rate, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid FX rate format: %w", err)
		}

		if inverseFxRateValue == 0 {
			return 0, fmt.Errorf("invalid FX rate (zero)")
		}

		rate := 1.0 / inverseFxRateValue
		converted := amount * rate

		// Cache the rate (5 minutes)
		_ = uc.redisClient.Set(ctx, cacheKey, fmt.Sprintf("%f", rate), 5*time.Minute).Err()

		return converted, nil
	}

	rateValue, err := strconv.ParseFloat(fxRate.Rate, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid FX rate format: %w", err)
	}
	converted := amount * rateValue

	// Cache the rate (5 minutes)
	_ = uc.redisClient.Set(ctx, cacheKey, fmt.Sprintf("%f", rateValue), 5*time.Minute).Err()

	return converted, nil
}

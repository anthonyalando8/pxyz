package usecase

import (
	"context"
	"fmt"
	"encoding/json"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// TransactionFeeRuleUsecase handles business logic for transaction fee rules
type TransactionFeeRuleUsecase struct {
	ruleRepo repository.TransactionFeeRuleRepository
	redisClient *redis.Client
	cacheTTL     time.Duration
}

// NewTransactionFeeRuleUsecase initializes a new TransactionFeeRuleUsecase
func NewTransactionFeeRuleUsecase(ruleRepo repository.TransactionFeeRuleRepository, redisClient *redis.Client, cacheTTL time.Duration) *TransactionFeeRuleUsecase {
	return &TransactionFeeRuleUsecase{
		ruleRepo: ruleRepo,
		redisClient: redisClient,
		cacheTTL: cacheTTL,	
	}
}

// CalculateFee calculates the fee for a given transaction type and amount
// CalculateFee calculates the fee in atomic units based on a matching rule.
// amount must be passed in atomic units of the given currency (e.g. cents).
func (uc *TransactionFeeRuleUsecase) CalculateFee(
	ctx context.Context,
	txType string,
	srcCurrency string,
	tgtCurrency string,
	amount float64, // in normal units (e.g., 12.34 USD)
) (float64, error) {
	// 1. Find the applicable rule
	rule, err := uc.GetByTypeAndCurrencies(ctx, txType, srcCurrency, tgtCurrency)
	if err != nil {
		return 0, err
	}
	if rule == nil {
		return 0, errors.New("no matching transaction fee rule found")
	}

	var fee float64

	switch rule.FeeType {
	case "fixed":
		// Convert atomic units to float
		fee = float64(rule.FeeValue) / 100.0 // assuming USD cents or similar
	case "percentage":
		// FeeValue in bps (1/100 of a percent)
		fee = amount * float64(rule.FeeValue) / 10_000.0
	default:
		return 0, errors.New("unsupported fee type: " + rule.FeeType)
	}

	// 2. Enforce min and max limits if set
	if rule.MinFee != nil && fee < float64(*rule.MinFee)/100.0 {
		fee = float64(*rule.MinFee) / 100.0
	}
	if rule.MaxFee != nil && fee > float64(*rule.MaxFee)/100.0 {
		fee = float64(*rule.MaxFee) / 100.0
	}

	return fee, nil
}


// CreateBatch inserts multiple fee rules
func (uc *TransactionFeeRuleUsecase) CreateBatch(ctx context.Context, rules []*domain.TransactionFeeRule, tx pgx.Tx) map[int]error {
	if len(rules) == 0 {
		return map[int]error{0: errors.New("transaction fee rules list is empty")}
	}
	return uc.ruleRepo.CreateBatch(ctx, rules, tx)
}

// GetByTypeAndCurrencies fetches a fee rule by txType + source + target currency
func (uc *TransactionFeeRuleUsecase) GetByTypeAndCurrencies(
	ctx context.Context,
	txType, srcCurrency, tgtCurrency string,
) (*domain.TransactionFeeRule, error) {
	cacheKey := fmt.Sprintf("fee_rule:%s:%s:%s", txType, srcCurrency, tgtCurrency)

	// 1. Try cache
	if uc.redisClient != nil {
		if data, err := uc.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
			var rule domain.TransactionFeeRule
			if err := json.Unmarshal(data, &rule); err == nil {
				return &rule, nil
			}
		}
	}

	// 2. Fallback to DB
	rule, err := uc.ruleRepo.GetByTypeAndCurrencies(ctx, txType, srcCurrency, tgtCurrency)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, errors.New("no matching transaction fee rule found")
	}

	// 3. Save to cache
	if uc.redisClient != nil {
		if data, err := json.Marshal(rule); err == nil {
			_ = uc.redisClient.Set(ctx, cacheKey, data, uc.cacheTTL).Err()
		}
	}

	return rule, nil
}

// ListAll fetches all rules
func (uc *TransactionFeeRuleUsecase) ListAll(ctx context.Context) ([]*domain.TransactionFeeRule, error) {
	return uc.ruleRepo.ListAll(ctx)
}

// Delete removes a rule by ID
func (uc *TransactionFeeRuleUsecase) Delete(ctx context.Context, id int64, tx pgx.Tx) error {
	return uc.ruleRepo.Delete(ctx, id, tx)
}

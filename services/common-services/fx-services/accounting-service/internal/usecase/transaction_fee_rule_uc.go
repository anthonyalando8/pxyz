package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// TransactionFeeRuleUsecase handles business logic for transaction fee rules
type TransactionFeeRuleUsecase struct {
	feeRuleRepo repository.TransactionFeeRuleRepository
	redisClient *redis.Client
}

// NewTransactionFeeRuleUsecase initializes a new TransactionFeeRuleUsecase
func NewTransactionFeeRuleUsecase(
	feeRuleRepo repository.TransactionFeeRuleRepository,
	redisClient *redis.Client,
) *TransactionFeeRuleUsecase {
	return &TransactionFeeRuleUsecase{
		feeRuleRepo: feeRuleRepo,
		redisClient: redisClient,
	}
}

// BeginTx starts a db transaction
func (uc *TransactionFeeRuleUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.feeRuleRepo.BeginTx(ctx)
}

// ===============================
// FEE RULE CREATION
// ===============================

// Create creates a new fee rule
func (uc *TransactionFeeRuleUsecase) Create(
	ctx context.Context,
	tx pgx.Tx,
	rule *domain.FeeRuleCreate,
) (*domain.TransactionFeeRule, error) {
	if tx == nil {
		return nil, errors.New("transaction required for Create")
	}

	// Validate rule
	if err := uc.validateFeeRuleCreate(rule); err != nil {
		return nil, fmt.Errorf("invalid fee rule: %w", err)
	}

	// Create rule
	feeRule, err := uc.feeRuleRepo.Create(ctx, tx, rule)
	if err != nil {
		return nil, fmt.Errorf("failed to create fee rule: %w", err)
	}

	// Invalidate cache
	_ = uc.InvalidateAllRulesCache(ctx)

	return feeRule, nil
}

// CreateBatch creates multiple fee rules
func (uc *TransactionFeeRuleUsecase) CreateBatch(
	ctx context.Context,
	tx pgx.Tx,
	rules []*domain.FeeRuleCreate,
) ([]*domain.TransactionFeeRule, map[int]error) {
	if tx == nil {
		return nil, map[int]error{0: errors.New("transaction required")}
	}

	if len(rules) == 0 {
		return nil, map[int]error{0: errors.New("fee rules list is empty")}
	}

	// Validate all rules first
	errs := make(map[int]error)
	for i, rule := range rules {
		if err := uc.validateFeeRuleCreate(rule); err != nil {
			errs[i] = fmt.Errorf("invalid rule at index %d: %w", i, err)
		}
	}

	// If validation errors exist, return early
	if len(errs) > 0 {
		return nil, errs
	}

	// Create rules in batch
	feeRules, createErrs := uc.feeRuleRepo.CreateBatch(ctx, tx, rules)

	// Invalidate cache
	_ = uc.InvalidateAllRulesCache(ctx)

	return feeRules, createErrs
}

// ===============================
// FEE RULE UPDATES
// ===============================

// Update updates an existing fee rule
func (uc *TransactionFeeRuleUsecase) Update(
	ctx context.Context,
	tx pgx.Tx,
	rule *domain.TransactionFeeRule,
) error {
	if tx == nil {
		return errors.New("transaction required for Update")
	}

	if !rule.IsValid() {
		return errors.New("invalid fee rule")
	}

	if err := uc.feeRuleRepo.Update(ctx, tx, rule); err != nil {
		return fmt.Errorf("failed to update fee rule: %w", err)
	}

	// Invalidate cache
	_ = uc.InvalidateRuleCache(ctx, rule.ID)
	_ = uc.InvalidateAllRulesCache(ctx)

	return nil
}

// ExpireRule marks a rule as expired (sets valid_to)
func (uc *TransactionFeeRuleUsecase) ExpireRule(
	ctx context.Context,
	tx pgx.Tx,
	ruleID int64,
	validTo time.Time,
) error {
	if tx == nil {
		return errors.New("transaction required for ExpireRule")
	}

	if err := uc.feeRuleRepo.ExpireRule(ctx, tx, ruleID, validTo); err != nil {
		return fmt.Errorf("failed to expire fee rule: %w", err)
	}

	// Invalidate cache
	_ = uc.InvalidateRuleCache(ctx, ruleID)
	_ = uc.InvalidateAllRulesCache(ctx)

	return nil
}

// DeactivateRule deactivates a rule (sets is_active = false)
func (uc *TransactionFeeRuleUsecase) DeactivateRule(
	ctx context.Context,
	tx pgx.Tx,
	ruleID int64,
) error {
	if tx == nil {
		return errors.New("transaction required for DeactivateRule")
	}

	// Get rule
	rule, err := uc.GetByID(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("failed to get rule: %w", err)
	}

	// Update is_active
	rule.IsActive = false

	if err := uc.feeRuleRepo.Update(ctx, tx, rule); err != nil {
		return fmt.Errorf("failed to deactivate rule: %w", err)
	}

	// Invalidate cache
	_ = uc.InvalidateRuleCache(ctx, ruleID)
	_ = uc.InvalidateAllRulesCache(ctx)

	return nil
}

// ActivateRule activates a rule (sets is_active = true)
func (uc *TransactionFeeRuleUsecase) ActivateRule(
	ctx context.Context,
	tx pgx.Tx,
	ruleID int64,
) error {
	if tx == nil {
		return errors.New("transaction required for ActivateRule")
	}

	// Get rule
	rule, err := uc.GetByID(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("failed to get rule: %w", err)
	}

	// Update is_active
	rule.IsActive = true

	if err := uc.feeRuleRepo.Update(ctx, tx, rule); err != nil {
		return fmt.Errorf("failed to activate rule: %w", err)
	}

	// Invalidate cache
	_ = uc.InvalidateRuleCache(ctx, ruleID)
	_ = uc.InvalidateAllRulesCache(ctx)

	return nil
}

// Delete deletes a fee rule
func (uc *TransactionFeeRuleUsecase) Delete(
	ctx context.Context,
	tx pgx.Tx,
	ruleID int64,
) error {
	if tx == nil {
		return errors.New("transaction required for Delete")
	}

	if err := uc.feeRuleRepo.Delete(ctx, tx, ruleID); err != nil {
		return fmt.Errorf("failed to delete fee rule: %w", err)
	}

	// Invalidate cache
	_ = uc.InvalidateRuleCache(ctx, ruleID)
	_ = uc.InvalidateAllRulesCache(ctx)

	return nil
}

// ===============================
// FEE RULE QUERIES
// ===============================

// GetByID fetches a fee rule by ID
func (uc *TransactionFeeRuleUsecase) GetByID(ctx context.Context, id int64) (*domain.TransactionFeeRule, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("fee_rule:id:%d", id)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var rule domain.TransactionFeeRule
		if jsonErr := json.Unmarshal([]byte(val), &rule); jsonErr == nil {
			return &rule, nil
		}
	}

	// Fetch from database
	rule, err := uc.feeRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get fee rule: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(rule); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return rule, nil
}

// List fetches fee rules based on filter
func (uc *TransactionFeeRuleUsecase) List(
	ctx context.Context,
	filter *domain.FeeRuleFilter,
) ([]*domain.TransactionFeeRule, error) {
	rules, err := uc.feeRuleRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list fee rules: %w", err)
	}

	return rules, nil
}

// ListActive fetches all active fee rules (heavily cached)
func (uc *TransactionFeeRuleUsecase) ListActive(ctx context.Context) ([]*domain.TransactionFeeRule, error) {
	// Try cache first (5 minutes - these are critical for transactions)
	cacheKey := "fee_rules:active:all"

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var rules []*domain.TransactionFeeRule
		if jsonErr := json.Unmarshal([]byte(val), &rules); jsonErr == nil {
			return rules, nil
		}
	}

	// Fetch from database
	rules, err := uc.feeRuleRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active fee rules: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(rules); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return rules, nil
}

// FindBestMatch finds the best matching rule (priority-based)
func (uc *TransactionFeeRuleUsecase) FindBestMatch(
	ctx context.Context,
	transactionType domain.TransactionType,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) (*domain.TransactionFeeRule, error) {
	// Try cache first
	cacheKey := uc.buildMatchCacheKey(transactionType, sourceCurrency, targetCurrency, accountType, ownerType)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var rule domain.TransactionFeeRule
		if jsonErr := json.Unmarshal([]byte(val), &rule); jsonErr == nil {
			return &rule, nil
		}
	}

	// Fetch from database
	rule, err := uc.feeRuleRepo.FindBestMatch(ctx, transactionType, sourceCurrency, targetCurrency, accountType, ownerType)
	if err != nil {
		return nil, fmt.Errorf("failed to find best match: %w", err)
	}

	if rule == nil {
		return nil, xerrors.ErrNotFound
	}

	// Cache result (5 minutes)
	if data, err := json.Marshal(rule); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return rule, nil
}

// FindAllMatches finds all matching rules
func (uc *TransactionFeeRuleUsecase) FindAllMatches(
	ctx context.Context,
	transactionType domain.TransactionType,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) ([]*domain.TransactionFeeRule, error) {
	rules, err := uc.feeRuleRepo.FindAllMatches(ctx, transactionType, sourceCurrency, targetCurrency, accountType, ownerType)
	if err != nil {
		return nil, fmt.Errorf("failed to find all matches: %w", err)
	}

	return rules, nil
}

// ===============================
// BULK OPERATIONS
// ===============================

// InitializeDefaultRules creates default fee rules
func (uc *TransactionFeeRuleUsecase) InitializeDefaultRules(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction required")
	}

	// Get default rules
	defaultRules := domain.DefaultTransactionFeeRules()

	// Convert to FeeRuleCreate
	var ruleCreates []*domain.FeeRuleCreate
	for _, rule := range defaultRules {
		ruleCreate := &domain.FeeRuleCreate{
			RuleName:          rule.RuleName,
			TransactionType:   rule.TransactionType,
			SourceCurrency:    rule.SourceCurrency,
			TargetCurrency:    rule.TargetCurrency,
			AccountType:       rule.AccountType,
			OwnerType:         rule.OwnerType,
			FeeType:           rule.FeeType,
			CalculationMethod: rule.CalculationMethod,
			FeeValue:          rule.FeeValue,
			MinFee:            rule.MinFee,
			MaxFee:            rule.MaxFee,
			Tiers:             rule.Tiers,
			ValidFrom:         rule.ValidFrom,
			ValidTo:           rule.ValidTo,
			IsActive:          rule.IsActive,
			Priority:          rule.Priority,
		}
		ruleCreates = append(ruleCreates, ruleCreate)
	}

	// Create in batch
	_, errs := uc.CreateBatch(ctx, tx, ruleCreates)
	if len(errs) > 0 {
		return fmt.Errorf("failed to initialize default rules: %v", errs)
	}

	return nil
}

// ===============================
// VALIDATION HELPERS
// ===============================

// validateFeeRuleCreate validates fee rule creation request
func (uc *TransactionFeeRuleUsecase) validateFeeRuleCreate(rule *domain.FeeRuleCreate) error {
	if rule.RuleName == "" {
		return errors.New("rule_name is required")
	}

	if rule.TransactionType == "" {
		return errors.New("transaction_type is required")
	}

	if rule.FeeType == "" {
		return errors.New("fee_type is required")
	}

	if rule.CalculationMethod == "" {
		return errors.New("calculation_method is required")
	}

	// Validate currency codes
	if rule.SourceCurrency != nil && len(*rule.SourceCurrency) > 8 {
		return errors.New("source_currency must be 8 characters or less")
	}

	if rule.TargetCurrency != nil && len(*rule.TargetCurrency) > 8 {
		return errors.New("target_currency must be 8 characters or less")
	}

	// Tiered method must have tiers
	if rule.CalculationMethod == domain.FeeCalculationTiered && rule.Tiers == nil {
		return errors.New("tiers required for tiered calculation method")
	}

	// Validate priority
	if rule.Priority < 0 {
		return errors.New("priority cannot be negative")
	}

	// Validate min/max fees
	if rule.MinFee != nil && *rule.MinFee < 0 {
		return errors.New("min_fee cannot be negative")
	}

	if rule.MaxFee != nil && *rule.MaxFee < 0 {
		return errors.New("max_fee cannot be negative")
	}

	if rule.MinFee != nil && rule.MaxFee != nil && *rule.MinFee > *rule.MaxFee {
		return errors.New("min_fee cannot be greater than max_fee")
	}

	// Validate valid_from/valid_to
	if rule.ValidTo != nil && rule.ValidTo.Before(rule.ValidFrom) {
		return errors.New("valid_to must be after valid_from")
	}

	return nil
}

// ===============================
// CACHE MANAGEMENT
// ===============================

// InvalidateRuleCache invalidates cache for a specific rule
func (uc *TransactionFeeRuleUsecase) InvalidateRuleCache(ctx context.Context, ruleID int64) error {
	cacheKey := fmt.Sprintf("fee_rule:id:%d", ruleID)
	return uc.redisClient.Del(ctx, cacheKey).Err()
}

// InvalidateAllRulesCache invalidates all fee rule caches
func (uc *TransactionFeeRuleUsecase) InvalidateAllRulesCache(ctx context.Context) error {
	pattern := "fee_rule*"

	iter := uc.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := uc.redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key: %w", err)
		}
	}

	return iter.Err()
}

// buildMatchCacheKey builds cache key for rule matching
func (uc *TransactionFeeRuleUsecase) buildMatchCacheKey(
	transactionType domain.TransactionType,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) string {
	key := fmt.Sprintf("fee_rule:match:%s", transactionType)

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

// ===============================
// HELPER METHODS
// ===============================

// ValidateRuleConflicts checks for conflicting rules before creation
func (uc *TransactionFeeRuleUsecase) ValidateRuleConflicts(
	ctx context.Context,
	rule *domain.FeeRuleCreate,
) (bool, error) {
	// Find existing matching rules
	filter := &domain.FeeRuleFilter{
		TransactionType: &rule.TransactionType,
		SourceCurrency:  rule.SourceCurrency,
		TargetCurrency:  rule.TargetCurrency,
		AccountType:     rule.AccountType,
		OwnerType:       rule.OwnerType,
		FeeType:         &rule.FeeType,
		IsActive:        boolPtr(true),
		ValidAt:         &rule.ValidFrom,
	}

	existingRules, err := uc.List(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to check conflicts: %w", err)
	}

	// If rules exist with same specificity, it's a conflict
	if len(existingRules) > 0 {
		return true, nil
	}

	return false, nil
}

// GetRulesByTransactionType fetches rules by transaction type
func (uc *TransactionFeeRuleUsecase) GetRulesByTransactionType(
	ctx context.Context,
	transactionType domain.TransactionType,
) ([]*domain.TransactionFeeRule, error) {
	filter := &domain.FeeRuleFilter{
		TransactionType: &transactionType,
		IsActive:        boolPtr(true),
	}

	return uc.List(ctx, filter)
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
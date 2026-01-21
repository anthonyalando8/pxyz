package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	"accounting-service/internal/pkg"

	//xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// TransactionFeeUsecase handles business logic for transaction fees
type TransactionFeeUsecase struct {
	feeRepo     repository.TransactionFeeRepository
	feeRuleRepo repository.TransactionFeeRuleRepository
	redisClient *redis.Client
	feeCalculator *service.TransactionFeeCalculator
}

// NewTransactionFeeUsecase initializes a new TransactionFeeUsecase
func NewTransactionFeeUsecase(
	feeRepo repository.TransactionFeeRepository,
	feeRuleRepo repository.TransactionFeeRuleRepository,
	redisClient *redis.Client,
	feeCalculator *service.TransactionFeeCalculator,
) *TransactionFeeUsecase {
	return &TransactionFeeUsecase{
		feeRepo:     feeRepo,
		feeRuleRepo: feeRuleRepo,
		redisClient: redisClient,
		feeCalculator: feeCalculator,
	}
}

// BeginTx starts a db transaction
func (uc *TransactionFeeUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.feeRepo.BeginTx(ctx)
}

// ===============================
// FEE CREATION
// ===============================

// Create inserts a single transaction fee
func (uc *TransactionFeeUsecase) Create(ctx context.Context, fee *domain.TransactionFee, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction required for Create")
	}

	if err := uc.validateTransactionFee(fee); err != nil {
		return fmt.Errorf("invalid transaction fee: %w", err)
	}

	if err := uc.feeRepo.Create(ctx, tx, fee); err != nil {
		return fmt.Errorf("failed to create transaction fee: %w", err)
	}

	return nil
}

// BatchCreate inserts multiple transaction fees at once
func (uc *TransactionFeeUsecase) BatchCreate(ctx context.Context, fees []*domain.TransactionFee, tx pgx.Tx) map[int]error {
	if tx == nil {
		return map[int]error{0: errors.New("transaction required")}
	}

	if len(fees) == 0 {
		return map[int]error{0: errors.New("transaction fees list is empty")}
	}

	// Validate all fees first
	errs := make(map[int]error)
	for i, fee := range fees {
		if err := uc.validateTransactionFee(fee); err != nil {
			errs[i] = fmt.Errorf("invalid fee at index %d: %w", i, err)
		}
	}

	// If validation errors exist, return early
	if len(errs) > 0 {
		return errs
	}

	// Create fees in batch
	return uc.feeRepo.CreateBatch(ctx, tx, fees)
}

// ===============================
// FEE CALCULATION
// ===============================

// CalculateFee calculates the fee for a transaction based on active rules
func (uc *TransactionFeeUsecase) CalculateFee(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
	toAddress *string,
) (*domain.FeeCalculation, error) {
	return uc.feeCalculator.CalculateFee(ctx, transactionType, amount, sourceCurrency, targetCurrency, accountType, ownerType, toAddress)
}

// CalculateMultipleFees calculates all applicable fees for a transaction
func (uc *TransactionFeeUsecase) CalculateMultipleFees(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
) ([]*domain.FeeCalculation, error) {
	return uc.feeCalculator.CalculateMultipleFees(ctx, transactionType, amount, sourceCurrency, targetCurrency, accountType, ownerType)

}

// ===============================
// FEE QUERIES
// ===============================

// GetByReceipt fetches all fees for a given receipt
func (uc *TransactionFeeUsecase) GetByReceipt(ctx context.Context, receiptCode string) ([]*domain.TransactionFee, error) {
	if receiptCode == "" {
		return nil, errors.New("receipt code cannot be empty")
	}

	// Try cache first (1 minute)
	cacheKey := fmt.Sprintf("fees:receipt:%s", receiptCode)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var fees []*domain.TransactionFee
		if jsonErr := json.Unmarshal([]byte(val), &fees); jsonErr == nil {
			return fees, nil
		}
	}

	// Fetch from database
	fees, err := uc.feeRepo.GetByReceipt(ctx, receiptCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get fees by receipt: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(fees); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 1*time.Minute).Err()
	}

	return fees, nil
}

// GetByFeeRule fetches fees applied using a specific rule
func (uc *TransactionFeeUsecase) GetByFeeRule(ctx context.Context, feeRuleID int64, limit int) ([]*domain.TransactionFee, error) {
	if limit <= 0 {
		limit = 100
	}

	fees, err := uc.feeRepo.GetByFeeRule(ctx, feeRuleID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get fees by rule: %w", err)
	}

	return fees, nil
}

// GetByAgent fetches all fees for a specific agent
func (uc *TransactionFeeUsecase) GetByAgent(ctx context.Context, agentExternalID string, from, to time.Time) ([]*domain.TransactionFee, error) {
	if agentExternalID == "" {
		return nil, errors.New("agent external ID cannot be empty")
	}

	// Try cache first (2 minutes)
	cacheKey := fmt.Sprintf("fees:agent:%s:%d:%d", agentExternalID, from.Unix(), to.Unix())

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var fees []*domain.TransactionFee
		if jsonErr := json.Unmarshal([]byte(val), &fees); jsonErr == nil {
			return fees, nil
		}
	}

	// Fetch from database
	fees, err := uc.feeRepo.GetByAgent(ctx, agentExternalID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get fees by agent: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(fees); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return fees, nil
}

// ===============================
// FEE STATISTICS
// ===============================

// GetTotalFeesByType returns total fees collected by type
func (uc *TransactionFeeUsecase) GetTotalFeesByType(
	ctx context.Context,
	feeType domain.FeeType,
	from, to time.Time,
) (float64, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("fees:total:%s:%d:%d", feeType, from.Unix(), to.Unix())

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var total float64
		if jsonErr := json.Unmarshal([]byte(val), &total); jsonErr == nil {
			return total, nil
		}
	}

	// Fetch from database
	total, err := uc.feeRepo.GetTotalFeesByType(ctx, feeType, from, to)
	if err != nil {
		return 0, fmt.Errorf("failed to get total fees: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(total); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return total, nil
}

// GetAgentCommissionSummary returns agent commission summary
func (uc *TransactionFeeUsecase) GetAgentCommissionSummary(
	ctx context.Context,
	agentExternalID string,
	from, to time.Time,
) (map[string]float64, error) {  // ✅ Changed to float64
	if agentExternalID == "" {
		return nil, errors. New("agent external ID cannot be empty")
	}

	// Try cache first (5 minutes)
	cacheKey := fmt. Sprintf("fees:commission:%s:%d:%d", agentExternalID, from.Unix(), to. Unix())

	if val, err := uc.redisClient. Get(ctx, cacheKey). Result(); err == nil {
		var summary map[string]float64  // ✅ Correct
		if jsonErr := json. Unmarshal([]byte(val), &summary); jsonErr == nil {
			return summary, nil  // ✅ Now matches
		}
	}

	// Fetch from database
	summary, err := uc.feeRepo.GetAgentCommissionSummary(ctx, agentExternalID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent commission summary: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(summary); err == nil {
		_ = uc.redisClient. Set(ctx, cacheKey, data, 5*time. Minute).Err()
	}

	return summary, nil
}

// ===============================
// VALIDATION HELPERS
// ===============================

// validateTransactionFee validates a transaction fee
func (uc *TransactionFeeUsecase) validateTransactionFee(fee *domain.TransactionFee) error {
	if fee.ReceiptCode == "" {
		return errors.New("receipt_code is required")
	}

	if fee.FeeType == "" {
		return errors.New("fee_type is required")
	}

	if fee.Amount < 0 {
		return errors.New("amount cannot be negative")
	}

	if fee.Currency == "" || len(fee.Currency) > 8 {
		return errors.New("invalid currency code")
	}

	// Agent commission must have agent_external_id
	if fee.FeeType == domain.FeeTypeAgentCommission && fee.AgentExternalID == nil {
		return errors.New("agent_external_id required for agent_commission")
	}

	return nil
}

// ===============================
// CACHE MANAGEMENT
// ===============================

// InvalidateFeeCache invalidates fee cache for a receipt
func (uc *TransactionFeeUsecase) InvalidateFeeCache(ctx context.Context, receiptCode string) error {
	cacheKey := fmt.Sprintf("fees:receipt:%s", receiptCode)
	return uc.redisClient.Del(ctx, cacheKey).Err()
}

// InvalidateAgentFeeCache invalidates agent fee cache
func (uc *TransactionFeeUsecase) InvalidateAgentFeeCache(ctx context.Context, agentExternalID string) error {
	pattern := fmt.Sprintf("fees:agent:%s:*", agentExternalID)

	iter := uc.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := uc.redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key: %w", err)
		}
	}

	return iter.Err()
}


// ===============================
// HELPER METHODS
// ===============================

// PreviewFee calculates fee without creating a record (for UI display)
func (uc *TransactionFeeUsecase) PreviewFee(
	ctx context.Context,
	transactionType domain.TransactionType,
	amount float64,
	sourceCurrency, targetCurrency *string,
	accountType *domain.AccountType,
	ownerType *domain.OwnerType,
	toAddress *string,
) (*domain.FeeCalculation, error) {
	return uc.CalculateFee(ctx, transactionType, amount, sourceCurrency, targetCurrency, accountType, ownerType, toAddress,)
}

// CreateFeeForTransaction creates fees for a transaction and returns total
func (uc *TransactionFeeUsecase) CreateFeeForTransaction(
	ctx context.Context,
	tx pgx.Tx,
	receiptCode string,
	transactionType domain.TransactionType,
	amount float64,
	currency string,
	accountType domain.AccountType,
	ownerType *domain.OwnerType,
) (float64, error) {
	// Calculate fees
	calculations, err := uc.CalculateMultipleFees(
		ctx,
		transactionType,
		amount,
		&currency,
		nil,
		&accountType,
		ownerType,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate fees: %w", err)
	}

	if len(calculations) == 0 {
		return 0, nil // No fees
	}

	// Create fee records
	var fees []*domain.TransactionFee
	var totalFee float64

	for _, calc := range calculations {
		fee := &domain.TransactionFee{
			ReceiptCode: receiptCode,
			FeeRuleID:   calc.RuleID,
			FeeType:     calc.FeeType,
			Amount:      calc.Amount,
			Currency:    currency,
		}

		fees = append(fees, fee)
		totalFee += calc.Amount
	}

	// Batch create fees
	errs := uc.BatchCreate(ctx, fees, tx)
	if len(errs) > 0 {
		return 0, fmt.Errorf("failed to create fees: %v", errs)
	}

	return totalFee, nil
}

package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type LedgerUsecase struct {
	ledgerRepo  repository.LedgerRepository
	accountRepo repository.AccountRepository
	redisClient *redis.Client
}

func NewLedgerUsecase(
	ledgerRepo repository.LedgerRepository,
	accountRepo repository.AccountRepository,
	redisClient *redis.Client,
) *LedgerUsecase {
	return &LedgerUsecase{
		ledgerRepo:  ledgerRepo,
		accountRepo: accountRepo,
		redisClient: redisClient,
	}
}

// ===============================
// EXTERNAL API METHODS (gRPC)
// ===============================

// GetByID retrieves a ledger entry by ID with caching
func (uc *LedgerUsecase) GetByID(ctx context.Context, id int64) (*domain.Ledger, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("ledger:id:%d", id)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var ledger domain.Ledger
		if jsonErr := json.Unmarshal([]byte(val), &ledger); jsonErr == nil {
			return &ledger, nil
		}
	}

	// Fetch from database
	ledger, err := uc.ledgerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(ledger); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return ledger, nil
}

// ListByJournal retrieves all ledger entries for a journal
func (uc *LedgerUsecase) ListByJournal(ctx context.Context, journalID int64) ([]*domain.Ledger, error) {
	// Try cache first (5 minutes - ledgers for a journal don't change)
	cacheKey := fmt.Sprintf("ledger:journal:%d", journalID)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var ledgers []*domain.Ledger
		if jsonErr := json.Unmarshal([]byte(val), &ledgers); jsonErr == nil {
			return ledgers, nil
		}
	}

	// Fetch from database
	ledgers, err := uc.ledgerRepo.ListByJournal(ctx, journalID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by journal: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(ledgers); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return ledgers, nil
}

// ListByAccount retrieves ledger entries for an account with pagination
func (uc *LedgerUsecase) ListByAccount(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
	from, to *time.Time,
	limit, offset int,
) ([]*domain.Ledger, int, error) {
	// Validate pagination
	if limit <= 0 {
		limit = 100
	}

	if limit > 1000 {
		limit = 1000
	}

	// Try cache for recent queries (1 minute)
	cacheKey := uc.buildAccountLedgersCacheKey(accountNumber, accountType, from, to, limit, offset)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var result struct {
			Ledgers []*domain.Ledger `json:"ledgers"`
			Total   int              `json:"total"`
		}
		if jsonErr := json.Unmarshal([]byte(val), &result); jsonErr == nil {
			return result.Ledgers, result.Total, nil
		}
	}

	// Fetch from database
	ledgers, _, err := uc.ledgerRepo.ListByAccount(ctx, accountNumber, accountType, from, to, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list ledgers by account: %w", err)
	}

	// For total count, we'd need a separate count query
	// For now, return fetched count
	total := len(ledgers)

	// Cache result
	result := struct {
		Ledgers []*domain.Ledger `json:"ledgers"`
		Total   int              `json:"total"`
	}{
		Ledgers: ledgers,
		Total:   total,
	}

	if data, err := json.Marshal(result); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 1*time.Minute).Err()
	}

	return ledgers, total, nil
}

// ListByOwner retrieves all ledger entries for an owner's accounts
func (uc *LedgerUsecase) ListByOwner(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
	from, to time.Time,
) ([]*domain.Ledger, error) {
	// Try cache first (2 minutes)
	cacheKey := fmt.Sprintf("ledger:owner:%s:%s:%s:%d:%d",
		ownerType, ownerID, accountType, from.Unix(), to.Unix())

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var ledgers []*domain.Ledger
		if jsonErr := json.Unmarshal([]byte(val), &ledgers); jsonErr == nil {
			return ledgers, nil
		}
	}

	// Fetch from database
	ledgers, err := uc.ledgerRepo.ListByOwner(ctx, ownerType, ownerID, accountType, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by owner: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(ledgers); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return ledgers, nil
}

// ListByReceipt retrieves all ledger entries for a receipt code
func (uc *LedgerUsecase) ListByReceipt(ctx context.Context, receiptCode string) ([]*domain.Ledger, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("ledger:receipt:%s", receiptCode)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var ledgers []*domain.Ledger
		if jsonErr := json.Unmarshal([]byte(val), &ledgers); jsonErr == nil {
			return ledgers, nil
		}
	}

	// Fetch from database
	ledgers, err := uc.ledgerRepo.ListByReceipt(ctx, receiptCode)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by receipt: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(ledgers); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return ledgers, nil
}

// ===============================
// INTERNAL METHODS (used by transaction usecase)
// ===============================

// Create creates a new ledger entry (internal use)
func (uc *LedgerUsecase) Create(ctx context.Context, tx pgx.Tx, ledger *domain.LedgerCreate) (*domain.Ledger, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}

	// Validate
	if err := uc.validateLedgerCreate(ledger); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create ledger
	createdLedger, err := uc.ledgerRepo.Create(ctx, tx, ledger)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger: %w", err)
	}

	return createdLedger, nil
}

// CreateBatch creates multiple ledger entries (internal use)
func (uc *LedgerUsecase) CreateBatch(ctx context.Context, tx pgx.Tx, ledgers []*domain.LedgerCreate) ([]*domain.Ledger, map[int]error) {
	if tx == nil {
		return nil, map[int]error{0: fmt.Errorf("transaction is required")}
	}

	if len(ledgers) == 0 {
		return []*domain.Ledger{}, nil
	}

	// Validate all ledgers
	errs := make(map[int]error)
	for i, ledger := range ledgers {
		if err := uc.validateLedgerCreate(ledger); err != nil {
			errs[i] = fmt.Errorf("validation failed: %w", err)
		}
	}

	// If validation errors exist, return early
	if len(errs) > 0 {
		return nil, errs
	}

	// Create batch
	createdLedgers, createErrs := uc.ledgerRepo.CreateBatch(ctx, tx, ledgers)

	return createdLedgers, createErrs
}

// ===============================
// ANALYTICS & REPORTING
// ===============================

// GetAccountTotals calculates total debits and credits for an account
func (uc *LedgerUsecase) GetAccountTotals(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
	from, to time.Time,
) (*domain.AccountTotals, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("ledger:totals:%s:%s:%d:%d",
		accountNumber, accountType, from.Unix(), to.Unix())

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var totals domain.AccountTotals
		if jsonErr := json.Unmarshal([]byte(val), &totals); jsonErr == nil {
			return &totals, nil
		}
	}

	// Fetch ledgers
	ledgers, _, err := uc.ledgerRepo.ListByAccount(ctx, accountNumber, accountType, &from, &to, 10000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledgers: %w", err)
	}

	// Calculate totals
	var totalDebits, totalCredits float64
	var transactionCount int64

	for _, ledger := range ledgers {
		if ledger.DrCr == domain.DrCrDebit {
			totalDebits += ledger.Amount
		} else {
			totalCredits += ledger.Amount
		}
		transactionCount++
	}

	totals := &domain.AccountTotals{
		AccountNumber:    accountNumber,
		AccountType:      accountType,
		TotalDebits:      totalDebits,
		TotalCredits:     totalCredits,
		NetChange:        totalCredits - totalDebits,
		TransactionCount: transactionCount,
		PeriodStart:      from,
		PeriodEnd:        to,
	}

	// Cache result
	if data, err := json.Marshal(totals); err == nil {
		// Cache for longer if historical (1 hour), shorter if recent (5 min)
		ttl := 1 * time.Hour
		if to.After(time.Now().Add(-24 * time.Hour)) {
			ttl = 5 * time.Minute
		}
		_ = uc.redisClient.Set(ctx, cacheKey, data, ttl).Err()
	}

	return totals, nil
}

// GetRecentActivity retrieves recent ledger activity for an account
func (uc *LedgerUsecase) GetRecentActivity(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
	limit int,
) ([]*domain.Ledger, error) {
	if limit <= 0 {
		limit = 50
	}

	if limit > 500 {
		limit = 500
	}

	// Try cache first (1 minute for recent activity)
	cacheKey := fmt.Sprintf("ledger:recent:%s:%s:%d", accountNumber, accountType, limit)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var ledgers []*domain.Ledger
		if jsonErr := json.Unmarshal([]byte(val), &ledgers); jsonErr == nil {
			return ledgers, nil
		}
	}

	// Fetch from database
	ledgers, _, err := uc.ledgerRepo.ListByAccount(ctx, accountNumber, accountType, nil, nil, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(ledgers); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 1*time.Minute).Err()
	}

	return ledgers, nil
}

// ===============================
// VALIDATION
// ===============================

func (uc *LedgerUsecase) validateLedgerCreate(ledger *domain.LedgerCreate) error {
	if ledger.JournalID == 0 {
		return fmt.Errorf("journal_id is required")
	}

	if ledger.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}

	if ledger.Amount < 0 {
		return fmt.Errorf("amount must be positive")
	}

	if ledger.DrCr != domain.DrCrDebit && ledger.DrCr != domain.DrCrCredit {
		return fmt.Errorf("dr_cr must be either DR or CR")
	}

	if ledger.Currency == "" {
		return fmt.Errorf("currency is required")
	}

	if len(ledger.Currency) > 8 {
		return fmt.Errorf("currency code must be 8 characters or less")
	}

	return nil
}

// ===============================
// CACHE MANAGEMENT
// ===============================

// InvalidateLedgerCache invalidates cache for a specific ledger
func (uc *LedgerUsecase) InvalidateLedgerCache(ctx context.Context, ledgerID int64) error {
	cacheKey := fmt.Sprintf("ledger:id:%d", ledgerID)
	return uc.redisClient.Del(ctx, cacheKey).Err()
}

// InvalidateJournalLedgersCache invalidates cache for journal ledgers
func (uc *LedgerUsecase) InvalidateJournalLedgersCache(ctx context.Context, journalID int64) error {
	cacheKey := fmt.Sprintf("ledger:journal:%d", journalID)
	return uc.redisClient.Del(ctx, cacheKey).Err()
}

// InvalidateAccountLedgersCache invalidates cache for account ledgers
func (uc *LedgerUsecase) InvalidateAccountLedgersCache(ctx context.Context, accountNumber string) error {
	// Delete all ledger caches for this account
	pattern := fmt.Sprintf("ledger:*:%s:*", accountNumber)

	iter := uc.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := uc.redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key: %w", err)
		}
	}

	return iter.Err()
}

// InvalidateReceiptLedgersCache invalidates cache for receipt ledgers
func (uc *LedgerUsecase) InvalidateReceiptLedgersCache(ctx context.Context, receiptCode string) error {
	cacheKey := fmt.Sprintf("ledger:receipt:%s", receiptCode)
	return uc.redisClient.Del(ctx, cacheKey).Err()
}

// ===============================
// HELPER METHODS
// ===============================

// buildAccountLedgersCacheKey builds cache key for account ledgers query
func (uc *LedgerUsecase) buildAccountLedgersCacheKey(
	accountNumber string,
	accountType domain.AccountType,
	from, to *time.Time,
	limit, offset int,
) string {
	key := fmt.Sprintf("ledger:account:%s:%s", accountNumber, accountType)

	if from != nil {
		key += fmt.Sprintf(":from_%d", from.Unix())
	}

	if to != nil {
		key += fmt.Sprintf(":to_%d", to.Unix())
	}

	key += fmt.Sprintf(":limit_%d:offset_%d", limit, offset)

	return key
}

// ValidateDoubleEntry ensures DR = CR for a set of ledgers
func (uc *LedgerUsecase) ValidateDoubleEntry(ledgers []*domain.LedgerCreate) error {
	if len(ledgers) < 2 {
		return fmt.Errorf("at least 2 ledger entries required for double-entry")
	}

	// Group by currency
	balancesByCurrency := make(map[string]float64)

	for _, ledger := range ledgers {
		switch ledger.DrCr {
		case domain.DrCrDebit:
			balancesByCurrency[ledger.Currency] -= ledger.Amount
		case domain.DrCrCredit:
			balancesByCurrency[ledger.Currency] += ledger.Amount
		}
	}

	// Check that each currency balances
	for currency, balance := range balancesByCurrency {
		if balance != 0 {
			return fmt.Errorf("ledgers don't balance for currency %s: difference = %.2f", currency, balance)
		}
	}

	return nil
}

// GetLedgersByDateRange retrieves ledgers within a date range (for reporting)
func (uc *LedgerUsecase) GetLedgersByDateRange(
	ctx context.Context,
	accountType domain.AccountType,
	from, to time.Time,
	limit int,
) ([]*domain.Ledger, error) {
	if limit <= 0 {
		limit = 1000
	}

	if limit > 10000 {
		limit = 10000
	}

	// This would need a new repository method for date-based queries
	// For now, return error indicating not implemented
	return nil, xerrors.ErrNotFound
}

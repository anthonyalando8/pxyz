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


type JournalUsecase struct {
	journalRepo repository.JournalRepository
	ledgerRepo  repository.LedgerRepository
	redisClient *redis.Client
}

func NewJournalUsecase(
	journalRepo repository.JournalRepository,
	ledgerRepo repository.LedgerRepository,
	redisClient *redis.Client,
) *JournalUsecase {
	return &JournalUsecase{
		journalRepo: journalRepo,
		ledgerRepo:  ledgerRepo,
		redisClient: redisClient,
	}
}

// ===============================
// EXTERNAL API METHODS (gRPC)
// ===============================

// GetByID retrieves a journal by ID with caching
func (uc *JournalUsecase) GetByID(ctx context.Context, id int64) (*domain.Journal, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("journal:id:%d", id)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var journal domain.Journal
		if jsonErr := json.Unmarshal([]byte(val), &journal); jsonErr == nil {
			return &journal, nil
		}
	}

	// Fetch from database
	journal, err := uc.journalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get journal: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(journal); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return journal, nil
}

// GetByIdempotencyKey retrieves a journal by idempotency key (for duplicate detection)
func (uc *JournalUsecase) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Journal, error) {
	// Try cache first (24 hours - idempotency keys are permanent)
	cacheKey := fmt.Sprintf("journal:idempotency:%s", idempotencyKey)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var journal domain.Journal
		if jsonErr := json.Unmarshal([]byte(val), &journal); jsonErr == nil {
			return &journal, nil
		}
	}

	// Fetch from database
	journal, err := uc.journalRepo.GetByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get journal by idempotency key: %w", err)
	}

	// Cache result (24 hours)
	if data, err := json.Marshal(journal); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 24*time.Hour).Err()
	}

	return journal, nil
}

// List retrieves journals based on filter criteria with pagination
func (uc *JournalUsecase) List(ctx context.Context, filter *domain.JournalFilter) ([]*domain.Journal, int, error) {
	// Validate filter
	if filter == nil {
		filter = &domain.JournalFilter{
			Limit: 100,
		}
	}

	// Set default limit if not provided
	if filter.Limit <= 0 {
		filter.Limit = 100
	}

	// Max limit to prevent overload
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	// Try cache for common queries (1 minute)
	cacheKey := uc.buildListCacheKey(filter)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var result struct {
			Journals []*domain.Journal `json:"journals"`
			Total    int               `json:"total"`
		}
		if jsonErr := json.Unmarshal([]byte(val), &result); jsonErr == nil {
			return result.Journals, result.Total, nil
		}
	}

	// Fetch from database
	journals, err := uc.journalRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list journals: %w", err)
	}

	// For total count, we'd need a separate count query
	// For now, return the fetched count (can be improved with COUNT query)
	total := len(journals)

	// Cache result (1 minute for list queries)
	result := struct {
		Journals []*domain.Journal `json:"journals"`
		Total    int               `json:"total"`
	}{
		Journals: journals,
		Total:    total,
	}

	if data, err := json.Marshal(result); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 1*time.Minute).Err()
	}

	return journals, total, nil
}

// GetByExternalRef retrieves journals by external reference (receipt code)
func (uc *JournalUsecase) GetByExternalRef(ctx context.Context, externalRef string) ([]*domain.Journal, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("journal:external_ref:%s", externalRef)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var journals []*domain.Journal
		if jsonErr := json.Unmarshal([]byte(val), &journals); jsonErr == nil {
			return journals, nil
		}
	}

	// Fetch from database
	journals, err := uc.journalRepo.ListByExternalRef(ctx, externalRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get journals by external ref: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(journals); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return journals, nil
}

// ListByAccount retrieves journals for a specific account
func (uc *JournalUsecase) ListByAccount(ctx context.Context, accountID int64, accountType domain.AccountType) ([]*domain.Journal, error) {
	// Try cache first (2 minutes)
	cacheKey := fmt.Sprintf("journal:account:%d:%s", accountID, accountType)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var journals []*domain.Journal
		if jsonErr := json.Unmarshal([]byte(val), &journals); jsonErr == nil {
			return journals, nil
		}
	}

	// Fetch from database
	journals, err := uc.journalRepo.ListByAccount(ctx, accountID, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to list journals by account: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(journals); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return journals, nil
}

// ListByTransactionType retrieves journals by transaction type
func (uc *JournalUsecase) ListByTransactionType(
	ctx context.Context,
	transactionType domain.TransactionType,
	accountType domain.AccountType,
	limit int,
) ([]*domain.Journal, error) {
	if limit <= 0 {
		limit = 100
	}

	if limit > 1000 {
		limit = 1000
	}

	// Try cache first (2 minutes)
	cacheKey := fmt.Sprintf("journal:type:%s:%s:%d", transactionType, accountType, limit)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var journals []*domain.Journal
		if jsonErr := json.Unmarshal([]byte(val), &journals); jsonErr == nil {
			return journals, nil
		}
	}

	// Fetch from database
	journals, err := uc.journalRepo.ListByTransactionType(ctx, transactionType, accountType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list journals by transaction type: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(journals); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return journals, nil
}

// ===============================
// INTERNAL METHODS (used by transaction usecase)
// ===============================

// Create creates a new journal (internal use - called from transaction usecase)
func (uc *JournalUsecase) Create(ctx context.Context, tx pgx.Tx, journal *domain.JournalCreate) (*domain.Journal, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}

	// Validate
	if err := uc.validateJournalCreate(journal); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create journal
	createdJournal, err := uc.journalRepo.Create(ctx, tx, journal)
	if err != nil {
		return nil, fmt.Errorf("failed to create journal: %w", err)
	}

	return createdJournal, nil
}

// CreateBatch creates multiple journals (internal use)
func (uc *JournalUsecase) CreateBatch(ctx context.Context, tx pgx.Tx, journals []*domain.JournalCreate) ([]*domain.Journal, map[int]error) {
	if tx == nil {
		return nil, map[int]error{0: fmt.Errorf("transaction is required")}
	}

	if len(journals) == 0 {
		return []*domain.Journal{}, nil
	}

	// Validate all journals
	errs := make(map[int]error)
	for i, journal := range journals {
		if err := uc.validateJournalCreate(journal); err != nil {
			errs[i] = fmt.Errorf("validation failed: %w", err)
		}
	}

	// If validation errors exist, return early
	if len(errs) > 0 {
		return nil, errs
	}

	// Create batch
	createdJournals, createErrs := uc.journalRepo.CreateBatch(ctx, tx, journals)

	return createdJournals, createErrs
}

// ===============================
// STATISTICS & REPORTING
// ===============================

// GetStatisticsByType returns journal counts by transaction type
func (uc *JournalUsecase) GetStatisticsByType(
	ctx context.Context,
	accountType domain.AccountType,
	startDate, endDate time.Time,
) (map[domain.TransactionType]int64, error) {
	// Try cache first (5 minutes)
	dateKey := fmt.Sprintf("%s:%s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	cacheKey := fmt.Sprintf("journal:stats:type:%s:%s", accountType, dateKey)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var stats map[domain.TransactionType]int64
		if jsonErr := json.Unmarshal([]byte(val), &stats); jsonErr == nil {
			return stats, nil
		}
	}

	// Fetch from database
	stats, err := uc.journalRepo.CountByType(ctx, accountType, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics by type: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(stats); err == nil {
		// Cache for longer if historical data (1 hour), shorter if recent (5 min)
		ttl := 1 * time.Hour
		if endDate.After(time.Now().Add(-24 * time.Hour)) {
			ttl = 5 * time.Minute
		}
		_ = uc.redisClient.Set(ctx, cacheKey, data, ttl).Err()
	}

	return stats, nil
}

// GetJournalWithLedgers retrieves a journal with all its ledger entries
func (uc *JournalUsecase) GetJournalWithLedgers(ctx context.Context, journalID int64) (*domain.LedgerAggregate, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("journal:with_ledgers:%d", journalID)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var aggregate domain.LedgerAggregate
		if jsonErr := json.Unmarshal([]byte(val), &aggregate); jsonErr == nil {
			return &aggregate, nil
		}
	}

	// Fetch journal
	journal, err := uc.journalRepo.GetByID(ctx, journalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get journal: %w", err)
	}

	// Fetch ledgers
	ledgers, err := uc.ledgerRepo.ListByJournal(ctx, journalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledgers: %w", err)
	}

	aggregate := &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}

	// Cache result
	if data, err := json.Marshal(aggregate); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return aggregate, nil
}

// ===============================
// VALIDATION
// ===============================

func (uc *JournalUsecase) validateJournalCreate(journal *domain.JournalCreate) error {
	if journal.IdempotencyKey == nil || *journal.IdempotencyKey == "" {
		return fmt.Errorf("idempotency_key is required")
	}

	if journal.TransactionType == "" {
		return fmt.Errorf("transaction_type is required")
	}

	if journal.AccountType == "" {
		return fmt.Errorf("account_type is required")
	}

	if journal.CreatedByExternalID == nil || *journal.CreatedByExternalID == "" {
		return fmt.Errorf("created_by_external_id is required")
	}

	if journal.CreatedByType == nil || *journal.CreatedByType == "" {
		return fmt.Errorf("created_by_type is required")
	}

	// Validate account type restrictions
	if journal.AccountType == domain.AccountTypeDemo {
		restrictedTypes := []domain.TransactionType{
			domain.TransactionTypeDeposit,
			domain.TransactionTypeWithdrawal,
			domain.TransactionTypeTransfer,
			domain.TransactionTypeFee,
			domain.TransactionTypeCommission,
		}

		for _, restricted := range restrictedTypes {
			if journal.TransactionType == restricted {
				return fmt.Errorf("transaction type %s not allowed for demo accounts", journal.TransactionType)
			}
		}
	}

	return nil
}

// ===============================
// CACHE MANAGEMENT
// ===============================

// InvalidateJournalCache invalidates cache for a specific journal
func (uc *JournalUsecase) InvalidateJournalCache(ctx context.Context, journalID int64) error {
	keys := []string{
		fmt.Sprintf("journal:id:%d", journalID),
		fmt.Sprintf("journal:with_ledgers:%d", journalID),
	}

	for _, key := range keys {
		if err := uc.redisClient.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key %s: %w", key, err)
		}
	}

	return nil
}

// InvalidateExternalRefCache invalidates cache for external reference
func (uc *JournalUsecase) InvalidateExternalRefCache(ctx context.Context, externalRef string) error {
	cacheKey := fmt.Sprintf("journal:external_ref:%s", externalRef)
	return uc.redisClient.Del(ctx, cacheKey).Err()
}

// InvalidateAccountJournalsCache invalidates cache for account journals
func (uc *JournalUsecase) InvalidateAccountJournalsCache(ctx context.Context, accountID int64, accountType domain.AccountType) error {
	cacheKey := fmt.Sprintf("journal:account:%d:%s", accountID, accountType)
	return uc.redisClient.Del(ctx, cacheKey).Err()
}

// InvalidateListCache invalidates list query caches (called after journal creation)
func (uc *JournalUsecase) InvalidateListCache(ctx context.Context) error {
	// Delete all journal list caches (they use pattern matching)
	pattern := "journal:list:*"
	
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

// buildListCacheKey builds a cache key for list queries
func (uc *JournalUsecase) buildListCacheKey(filter *domain.JournalFilter) string {
	key := "journal:list"

	if filter.TransactionType != nil {
		key += fmt.Sprintf(":type_%s", *filter.TransactionType)
	}

	if filter.AccountType != nil {
		key += fmt.Sprintf(":acct_%s", *filter.AccountType)
	}

	if filter.ExternalRef != nil {
		key += fmt.Sprintf(":ref_%s", *filter.ExternalRef)
	}

	if filter.CreatedByID != nil {
		key += fmt.Sprintf(":creator_%s", *filter.CreatedByID)
	}

	if filter.StartDate != nil {
		key += fmt.Sprintf(":from_%d", filter.StartDate.Unix())
	}

	if filter.EndDate != nil {
		key += fmt.Sprintf(":to_%d", filter.EndDate.Unix())
	}

	key += fmt.Sprintf(":limit_%d:offset_%d", filter.Limit, filter.Offset)

	return key
}

// GetRecentJournals retrieves the most recent journals (for dashboard)
func (uc *JournalUsecase) GetRecentJournals(
	ctx context.Context,
	accountType domain.AccountType,
	limit int,
) ([]*domain.Journal, error) {
	if limit <= 0 {
		limit = 50
	}

	if limit > 500 {
		limit = 500
	}

	// Try cache first (1 minute for recent data)
	cacheKey := fmt.Sprintf("journal:recent:%s:%d", accountType, limit)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var journals []*domain.Journal
		if jsonErr := json.Unmarshal([]byte(val), &journals); jsonErr == nil {
			return journals, nil
		}
	}

	// Fetch from database
	filter := &domain.JournalFilter{
		AccountType: &accountType,
		Limit:       limit,
		Offset:      0,
	}

	journals, err := uc.journalRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent journals: %w", err)
	}

	// Cache result (1 minute)
	if data, err := json.Marshal(journals); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 1*time.Minute).Err()
	}

	return journals, nil
}
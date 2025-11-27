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

type StatementUsecase struct {
	statementRepo repository.StatementRepository
	accountRepo   repository.AccountRepository
	balanceRepo   repository.BalanceRepository
	redisClient   *redis.Client
}

// NewStatementUsecase initializes the usecase
func NewStatementUsecase(
	statementRepo repository.StatementRepository,
	accountRepo repository.AccountRepository,
	balanceRepo repository.BalanceRepository,
	redisClient *redis.Client,
) *StatementUsecase {
	return &StatementUsecase{
		statementRepo: statementRepo,
		accountRepo:   accountRepo,
		balanceRepo:   balanceRepo,
		redisClient:   redisClient,
	}
}

func (uc *StatementUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.statementRepo.BeginTx(ctx)
}

// ===============================
// BALANCE QUERIES
// ===============================

// GetAccountBalance fetches current balance for an account
func (uc *StatementUsecase) GetAccountBalance(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
) (int64, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("balance:account:%s", accountNumber)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var balance int64
		if jsonErr := json.Unmarshal([]byte(val), &balance); jsonErr == nil {
			return balance, nil
		}
	}

	// Fetch from database
	balance, err := uc.statementRepo.GetCurrentBalance(ctx, accountNumber, accountType)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return 0, nil // No balance = 0
		}
		return 0, fmt.Errorf("failed to get account balance: %w", err)
	}

	// Cache for 30 seconds (balance changes frequently)
	if data, err := json.Marshal(balance.Balance); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 30*time.Second).Err()
	}

	return balance.Balance, nil
}

// GetCachedBalance fetches balance from cached balances table (fast)
func (uc *StatementUsecase) GetCachedBalance(ctx context.Context, accountID int64) (*domain.Balance, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("balance:id:%d", accountID)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var balance domain.Balance
		if jsonErr := json.Unmarshal([]byte(val), &balance); jsonErr == nil {
			return &balance, nil
		}
	}

	// Fetch from balances table
	balance, err := uc.statementRepo.GetCachedBalance(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached balance: %w", err)
	}

	// Cache for 30 seconds
	if data, err := json.Marshal(balance); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 30*time.Second).Err()
	}

	return balance, nil
}

// ===============================
// ACCOUNT STATEMENTS
// ===============================

// GetAccountStatement generates a detailed account statement for a period
func (uc *StatementUsecase) GetAccountStatement(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
	from, to time.Time,
) (*domain.AccountStatement, error) {
	// Try cache for recent statements (1 minute)
	cacheKey := fmt.Sprintf("statement:account:%s:%s:%d:%d", 
		accountNumber, accountType, from.Unix(), to.Unix())
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var statement domain.AccountStatement
		if jsonErr := json.Unmarshal([]byte(val), &statement); jsonErr == nil {
			return &statement, nil
		}
	}

	// Fetch full statement from repository
	statement, err := uc.statementRepo.GetAccountStatement(ctx, accountNumber, accountType, from, to)
	if err != nil {
		if err == xerrors.ErrNotFound {
			// Return empty statement
			return &domain.AccountStatement{
				AccountNumber:  accountNumber,
				AccountType:    accountType,
				Ledgers:        []*domain.Ledger{},
				OpeningBalance: 0,
				ClosingBalance: 0,
				TotalDebits:    0,
				TotalCredits:   0,
				PeriodStart:    from,
				PeriodEnd:      to,
			}, nil
		}
		return nil, fmt.Errorf("failed to get account statement: %w", err)
	}

	// Cache for 1 minute
	if data, err := json.Marshal(statement); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 1*time.Minute).Err()
	}

	return statement, nil
}

// ListAccountLedgers fetches ledger entries for an account in a period
func (uc *StatementUsecase) ListAccountLedgers(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
	from, to time.Time,
) ([]*domain.Ledger, error) {
	ledgers, err := uc.statementRepo.ListLedgersByAccount(ctx, accountNumber, accountType, from, to)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.Ledger{}, nil
		}
		return nil, fmt.Errorf("failed to list account ledgers: %w", err)
	}

	return ledgers, nil
}

// ===============================
// OWNER STATEMENTS
// ===============================

// GetOwnerStatement aggregates statements for all accounts of an owner
func (uc *StatementUsecase) GetOwnerStatement(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
	from, to time.Time,
) ([]*domain.AccountStatement, error) {
	// Get all owner accounts
	accounts, err := uc.accountRepo.GetByOwner(ctx, ownerType, ownerID, accountType)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.AccountStatement{}, nil
		}
		return nil, fmt.Errorf("failed to get owner accounts: %w", err)
	}

	if len(accounts) == 0 {
		return []*domain.AccountStatement{}, nil
	}

	// Fetch statement for each account
	var statements []*domain.AccountStatement
	
	for _, account := range accounts {
		statement, err := uc.GetAccountStatement(ctx, account.AccountNumber, accountType, from, to)
		if err != nil {
			// Log error but continue with other accounts
			continue
		}
		
		statements = append(statements, statement)
	}

	return statements, nil
}

// GetOwnerSummary returns aggregated summary for all owner accounts
func (uc *StatementUsecase) GetOwnerSummary(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
) (*domain.OwnerSummary, error) {
	// Try cache first (2 minutes)
	cacheKey := fmt.Sprintf("summary:owner:%s:%s:%s", ownerType, ownerID, accountType)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var summary domain.OwnerSummary
		if jsonErr := json.Unmarshal([]byte(val), &summary); jsonErr == nil {
			return &summary, nil
		}
	}

	// Fetch from repository
	summary, err := uc.statementRepo.GetOwnerSummary(ctx, ownerType, ownerID, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get owner summary: %w", err)
	}

	// Cache for 2 minutes
	if data, err := json.Marshal(summary); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return summary, nil
}


// ListOwnerLedgers fetches all ledger entries for an owner in a period
func (uc *StatementUsecase) ListOwnerLedgers(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
	from, to time.Time,
) ([]*domain.Ledger, error) {
	ledgers, err := uc.statementRepo.ListLedgersByOwner(ctx, ownerType, ownerID, accountType, from, to)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.Ledger{}, nil
		}
		return nil, fmt.Errorf("failed to list owner ledgers: %w", err)
	}

	return ledgers, nil
}

// ===============================
// REPORTS AND ANALYTICS
// ===============================

// GenerateDailyReport produces aggregated report for a specific date
func (uc *StatementUsecase) GenerateDailyReport(
	ctx context.Context,
	date time.Time,
	accountType domain.AccountType,
) ([]*domain.DailyReport, error) {
	// Try cache first (daily reports are stable)
	dateKey := date.Format("2006-01-02")
	cacheKey := fmt.Sprintf("report:daily:%s:%s", dateKey, accountType)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var reports []*domain.DailyReport
		if jsonErr := json.Unmarshal([]byte(val), &reports); jsonErr == nil {
			return reports, nil
		}
	}

	// Fetch from repository
	reports, err := uc.statementRepo.GetDailySummary(ctx, date, accountType)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.DailyReport{}, nil
		}
		return nil, fmt.Errorf("failed to generate daily report: %w", err)
	}

	// Cache for 1 hour (historical data doesn't change)
	if data, err := json.Marshal(reports); err == nil {
		ttl := 1 * time.Hour
		
		// If date is today, use shorter TTL (5 minutes)
		today := time.Now().Format("2006-01-02")
		if dateKey == today {
			ttl = 5 * time.Minute
		}
		
		_ = uc.redisClient.Set(ctx, cacheKey, data, ttl).Err()
	}

	return reports, nil
}

// GenerateOwnerDailyReport generates daily report for specific owner
func (uc *StatementUsecase) GenerateOwnerDailyReport(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
	date time.Time,
) (*domain.DailyReport, error) {
	// Try cache first
	dateKey := date.Format("2006-01-02")
	cacheKey := fmt.Sprintf("report:owner:%s:%s:%s:%s", ownerType, ownerID, accountType, dateKey)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var report domain.DailyReport
		if jsonErr := json.Unmarshal([]byte(val), &report); jsonErr == nil {
			return &report, nil
		}
	}

	// Fetch from repository
	report, err := uc.statementRepo.GetOwnerDailySummary(ctx, ownerType, ownerID, accountType, date)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to generate owner daily report: %w", err)
	}

	// Cache with appropriate TTL
	if data, err := json.Marshal(report); err == nil {
		ttl := 1 * time.Hour
		today := time.Now().Format("2006-01-02")
		if dateKey == today {
			ttl = 5 * time.Minute
		}
		_ = uc.redisClient.Set(ctx, cacheKey, data, ttl).Err()
	}

	return report, nil
}

// GetTransactionSummary returns transaction statistics for a period
func (uc *StatementUsecase) GetTransactionSummary(
	ctx context.Context,
	accountType domain.AccountType,
	from, to time.Time,
) ([]*domain.TransactionSummary, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("summary:transactions:%s:%d:%d", accountType, from.Unix(), to.Unix())
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var summaries []*domain.TransactionSummary
		if jsonErr := json.Unmarshal([]byte(val), &summaries); jsonErr == nil {
			return summaries, nil
		}
	}

	// Fetch from repository
	summaries, err := uc.statementRepo.GetTransactionSummary(ctx, accountType, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction summary: %w", err)
	}

	// Cache for 5 minutes
	if data, err := json.Marshal(summaries); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return summaries, nil
}

// ===============================
// SYSTEM ANALYTICS
// ===============================

// GetSystemHoldings returns total system holdings by currency
// Uses materialized view for fast aggregation
func (uc *StatementUsecase) GetSystemHoldings(
	ctx context.Context,
	accountType domain.AccountType,
) (map[string]int64, error) {
	// Try cache first (5 minutes)
	cacheKey := fmt.Sprintf("holdings:system:%s", accountType)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var holdings map[string]int64
		if jsonErr := json.Unmarshal([]byte(val), &holdings); jsonErr == nil {
			return holdings, nil
		}
	}

	// Fetch from materialized view
	holdings, err := uc.statementRepo.GetSystemHoldings(ctx, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get system holdings: %w", err)
	}

	// Cache for 5 minutes
	if data, err := json.Marshal(holdings); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return holdings, nil
}

// GetDailyTransactionVolume returns daily transaction volume from materialized view
func (uc *StatementUsecase) GetDailyTransactionVolume(
	ctx context.Context,
	accountType domain.AccountType,
	date time.Time,
) ([]*domain.TransactionSummary, error) {
	// Try cache first (1 hour for historical, 5 min for today)
	dateKey := date.Format("2006-01-02")
	cacheKey := fmt.Sprintf("volume:daily:%s:%s", accountType, dateKey)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var summaries []*domain.TransactionSummary
		if jsonErr := json.Unmarshal([]byte(val), &summaries); jsonErr == nil {
			return summaries, nil
		}
	}

	// Fetch from materialized view
	summaries, err := uc.statementRepo.GetDailyTransactionVolume(ctx, accountType, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily transaction volume: %w", err)
	}

	// Cache with appropriate TTL
	if data, err := json.Marshal(summaries); err == nil {
		ttl := 1 * time.Hour
		today := time.Now().Format("2006-01-02")
		if dateKey == today {
			ttl = 5 * time.Minute
		}
		_ = uc.redisClient.Set(ctx, cacheKey, data, ttl).Err()
	}

	return summaries, nil
}

// ===============================
// CACHE MANAGEMENT
// ===============================

// InvalidateAccountCache invalidates all cache entries for an account
func (uc *StatementUsecase) InvalidateAccountCache(ctx context.Context, accountNumber string) error {
	pattern := fmt.Sprintf("*:account:%s*", accountNumber)
	
	iter := uc.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := uc.redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key %s: %w", iter.Val(), err)
		}
	}
	
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan cache keys: %w", err)
	}
	
	return nil
}

// InvalidateOwnerCache invalidates all cache entries for an owner
func (uc *StatementUsecase) InvalidateOwnerCache(ctx context.Context, ownerType domain.OwnerType, ownerID string) error {
	pattern := fmt.Sprintf("*:owner:%s:%s*", ownerType, ownerID)
	
	iter := uc.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := uc.redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key %s: %w", iter.Val(), err)
		}
	}
	
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan cache keys: %w", err)
	}
	
	return nil
}

// InvalidateBalanceCache invalidates balance cache after transaction
func (uc *StatementUsecase) InvalidateBalanceCache(ctx context.Context, accountID int64, accountNumber string) error {
	keys := []string{
		fmt.Sprintf("balance:id:%d", accountID),
		fmt.Sprintf("balance:account:%s", accountNumber),
	}

	for _, key := range keys {
		if err := uc.redisClient.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key %s: %w", key, err)
		}
	}

	return nil
}

// ===============================
// HELPER METHODS
// ===============================

// GetAccountBalanceHistory returns balance history for visualization
func (uc *StatementUsecase) GetAccountBalanceHistory(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
	from, to time.Time,
	interval string, // "hour", "day", "week", "month"
) ([]domain.BalanceSnapshot, error) {
	// This would query ledgers and aggregate by time bucket
	// Implementation depends on specific requirements
	// For now, return empty to show structure
	return []domain.BalanceSnapshot{}, nil
}

// BalanceSnapshot represents balance at a point in time
type BalanceSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Balance   int64     `json:"balance"`
}
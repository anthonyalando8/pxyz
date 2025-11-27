package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	xerrors "x/shared/utils/errors"
	"accounting-service/pkg/utils"
	"x/shared/utils/id"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type AccountUsecase struct {
	accountRepo repository.AccountRepository
	balanceRepo repository.BalanceRepository
	accountNumberGen *utils.AccountNumberGenerator
	redisClient *redis.Client
}

// NewAccountUsecase initializes a new AccountUsecase
func NewAccountUsecase(
	accountRepo repository.AccountRepository,
	balanceRepo repository.BalanceRepository,
	sf *id.Snowflake,
	redisClient *redis.Client,
) *AccountUsecase {
	return &AccountUsecase{
		accountRepo: accountRepo,
		balanceRepo: balanceRepo,
		redisClient: redisClient,
		accountNumberGen: utils.NewAccountNumberGenerator(sf),
	}
}

func (uc *AccountUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.accountRepo.BeginTx(ctx)
}

// ===============================
// SYSTEM ACCOUNT OPERATIONS
// ===============================

// GetSystemAccounts fetches all system accounts (cached for 5 minutes)
// System accounts are critical for transactions, so we cache them aggressively
func (uc *AccountUsecase) GetSystemAccounts(ctx context.Context) ([]*domain.Account, error) {
	cacheKey := "accounts:system:all"

	// Check Redis cache first
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var accounts []*domain.Account
		if jsonErr := json.Unmarshal([]byte(val), &accounts); jsonErr == nil {
			return accounts, nil
		}
	}

	// Query database with filter
	ownerType := domain.OwnerTypeSystem
	accountType := domain.AccountTypeReal // System accounts are always real
	
	filter := &domain.AccountFilter{
		OwnerType:   &ownerType,
		AccountType: &accountType,
	}

	accounts, err := uc.accountRepo.GetByFilter(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get system accounts: %w", err)
	}

	// Cache result in Redis (5 minutes)
	if data, err := json.Marshal(accounts); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return accounts, nil
}

// GetSystemAccount fetches a single system account by currency + purpose (cached)
// This is the most common system account lookup pattern
func (uc *AccountUsecase) GetSystemAccount(ctx context.Context, currency string, purpose domain.AccountPurpose) (*domain.Account, error) {
	// Try specific cache key first (faster)
	cacheKey := fmt.Sprintf("accounts:system:%s:%s", currency, purpose)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var account domain.Account
		if jsonErr := json.Unmarshal([]byte(val), &account); jsonErr == nil {
			return &account, nil
		}
	}

	// Fallback to fetching all system accounts and filtering
	accounts, err := uc.GetSystemAccounts(ctx)
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		if acc.Currency == currency && acc.Purpose == purpose {
			// Cache this specific account
			if data, err := json.Marshal(acc); err == nil {
				_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
			}
			return acc, nil
		}
	}

	return nil, xerrors.ErrNotFound
}

// InvalidateSystemAccountCache invalidates system account cache
// Call this when system accounts are modified (rare)
func (uc *AccountUsecase) InvalidateSystemAccountCache(ctx context.Context) error {
	// Delete all system account cache keys
	pattern := "accounts:system:*"
	
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

// ===============================
// ACCOUNT RETRIEVAL
// ===============================

// GetByAccountNumber fetches an account by its account number
func (uc *AccountUsecase) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("accounts:number:%s", accountNumber)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var account domain.Account
		if jsonErr := json.Unmarshal([]byte(val), &account); jsonErr == nil {
			return &account, nil
		}
	}

	// Fetch from database
	account, err := uc.accountRepo.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get account by number: %w", err)
	}

	// Cache result (2 minutes - shorter than system accounts)
	if data, err := json.Marshal(account); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return account, nil
}

// GetByID fetches an account by its ID
func (uc *AccountUsecase) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("accounts:id:%d", id)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var account domain.Account
		if jsonErr := json.Unmarshal([]byte(val), &account); jsonErr == nil {
			return &account, nil
		}
	}

	// Fetch from database
	account, err := uc.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get account by ID: %w", err)
	}

	// Cache result
	if data, err := json.Marshal(account); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return account, nil
}

// GetByOwner fetches all accounts for a given owner
// Uses GetOrCreateUserAccounts pattern for lazy account creation
func (uc *AccountUsecase) GetByOwner(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
) ([]*domain.Account, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("accounts:owner:%s:%s:%s", ownerType, ownerID, accountType)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var accounts []*domain.Account
		if jsonErr := json.Unmarshal([]byte(val), &accounts); jsonErr == nil {
			return accounts, nil
		}
	}

	// Fetch from database
	accounts, err := uc.accountRepo.GetByOwner(ctx, ownerType, ownerID, accountType)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.Account{}, nil // Return empty slice instead of error
		}
		return nil, fmt.Errorf("failed to get accounts by owner: %w", err)
	}

	// Cache result (1 minute - frequent changes)
	if data, err := json.Marshal(accounts); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 1*time.Minute).Err()
	}

	return accounts, nil
}

// GetOrCreateUserAccounts implements lazy account creation pattern
// If user has no accounts, creates default accounts based on account type
// MUST be called within a transaction
func (uc *AccountUsecase) GetOrCreateUserAccounts(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
	tx pgx.Tx,
) ([]*domain.Account, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction required for GetOrCreateUserAccounts")
	}

	accounts, err := uc.accountRepo.GetOrCreateUserAccounts(ctx, ownerType, ownerID, accountType, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create user accounts: %w", err)
	}

	// Invalidate cache after creation
	cacheKey := fmt.Sprintf("accounts:owner:%s:%s:%s", ownerType, ownerID, accountType)
	_ = uc.redisClient.Del(ctx, cacheKey).Err()

	return accounts, nil
}

// GetAccountWithBalance fetches account with its balance
func (uc *AccountUsecase) GetAccountWithBalance(ctx context.Context, accountNumber string) (*domain.Account, error) {
	// Get account
	account, err := uc.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}

	// Get balance
	balance, err := uc.balanceRepo.GetByAccountID(ctx, account.ID)
	if err != nil {
		// If balance doesn't exist, return account with nil balance
		if err == xerrors.ErrNotFound {
			return account, nil
		}
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	account.Balance = balance
	return account, nil
}

// ===============================
// ACCOUNT CREATION
// ===============================

// CreateAccount creates a single account inside a transaction
func (uc *AccountUsecase) CreateAccount(
	ctx context.Context,
	req *domain.CreateAccountRequest,
	tx pgx.Tx,
) (*domain.Account, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction required for CreateAccount")
	}

	// Validate request
	if err := uc.validateCreateAccountRequest(req); err != nil {
		return nil, fmt.Errorf("invalid create account request: %w", err)
	}

	// Create account
	now := time.Now()
	account := &domain.Account{
		OwnerType:             req.OwnerType,
		OwnerID:               req.OwnerID,
		Currency:              req.Currency,
		Purpose:               req.Purpose,
		AccountType:           req.AccountType,
		AccountNumber: uc.generateAccountNumber(
			req.AccountType,
			req.Purpose,
			req.OwnerType,
		),
		IsActive:              true,
		IsLocked:              false,
		OverdraftLimit:        req.OverdraftLimit,
		ParentAgentExternalID: req.ParentAgentExternalID,
		CommissionRate:        req.CommissionRate,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	// Insert account
	if err := uc.accountRepo.Create(ctx, account, tx); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// Invalidate owner cache
	cacheKey := fmt.Sprintf("accounts:owner:%s:%s:%s", req.OwnerType, req.OwnerID, req.AccountType)
	_ = uc.redisClient.Del(ctx, cacheKey).Err()

	return account, nil
}

// CreateAccounts creates multiple accounts inside a transaction
// Returns a map of errors keyed by account index for failed inserts
func (uc *AccountUsecase) CreateAccounts(
	ctx context.Context,
	requests []*domain.CreateAccountRequest,
	tx pgx.Tx,
) map[int]error {
	if tx == nil {
		return map[int]error{0: fmt.Errorf("transaction required")}
	}

	if len(requests) == 0 {
		return map[int]error{0: xerrors.ErrInvalidRequest}
	}

	// Validate requests and convert to Account domain
	errs := make(map[int]error)
	accounts := make([]*domain.Account, len(requests))
	now := time.Now()

	for i, req := range requests {
		// Validate required fields
		if err := uc.validateCreateAccountRequest(req); err != nil {
			errs[i] = err
			continue
		}

		// Convert to Account domain
		account := &domain.Account{
			OwnerType:             req.OwnerType,
			OwnerID:               req.OwnerID,
			Currency:              req.Currency,
			Purpose:               req.Purpose,
			AccountType:           req.AccountType,
			AccountNumber: uc.generateAccountNumber(
				req.AccountType,
				req.Purpose,
				req.OwnerType,
			),
			IsActive:              true,
			IsLocked:              false,
			OverdraftLimit:        req.OverdraftLimit,
			ParentAgentExternalID: req.ParentAgentExternalID,
			CommissionRate:        req.CommissionRate,
			CreatedAt:             now,
			UpdatedAt:             now,
		}

		accounts[i] = account
	}

	// If validation errors exist, return early
	if len(errs) > 0 {
		return errs
	}

	// Delegate to repository batch insert
	repoErrs := uc.accountRepo.CreateMany(ctx, accounts, tx)

	// Invalidate caches for all created accounts
	for i, account := range accounts {
		if _, hasError := repoErrs[i]; !hasError {
			cacheKey := fmt.Sprintf("accounts:owner:%s:%s:%s", account.OwnerType, account.OwnerID, account.AccountType)
			_ = uc.redisClient.Del(ctx, cacheKey).Err()
		}
	}

	return repoErrs
}

// ===============================
// ACCOUNT UPDATES
// ===============================

// UpdateAccount updates an existing account inside a transaction
func (uc *AccountUsecase) UpdateAccount(ctx context.Context, account *domain.Account, tx pgx.Tx) error {
	if tx == nil {
		return fmt.Errorf("transaction required for UpdateAccount")
	}

	if !account.IsValid() {
		return xerrors.ErrInvalidRequest
	}

	account.UpdatedAt = time.Now()

	if err := uc.accountRepo.Update(ctx, account, tx); err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	// Invalidate caches
	_ = uc.redisClient.Del(ctx, fmt.Sprintf("accounts:id:%d", account.ID)).Err()
	_ = uc.redisClient.Del(ctx, fmt.Sprintf("accounts:number:%s", account.AccountNumber)).Err()
	_ = uc.redisClient.Del(ctx, fmt.Sprintf("accounts:owner:%s:%s:%s", account.OwnerType, account.OwnerID, account.AccountType)).Err()

	return nil
}

// LockAccount locks an account (fraud prevention, maintenance)
func (uc *AccountUsecase) LockAccount(ctx context.Context, accountID int64, tx pgx.Tx) error {
	if tx == nil {
		return fmt.Errorf("transaction required for LockAccount")
	}

	if err := uc.accountRepo.Lock(ctx, accountID, tx); err != nil {
		return fmt.Errorf("failed to lock account: %w", err)
	}

	// Invalidate cache
	_ = uc.redisClient.Del(ctx, fmt.Sprintf("accounts:id:%d", accountID)).Err()

	return nil
}

// UnlockAccount unlocks an account
func (uc *AccountUsecase) UnlockAccount(ctx context.Context, accountID int64, tx pgx.Tx) error {
	if tx == nil {
		return fmt.Errorf("transaction required for UnlockAccount")
	}

	if err := uc.accountRepo.Unlock(ctx, accountID, tx); err != nil {
		return fmt.Errorf("failed to unlock account: %w", err)
	}

	// Invalidate cache
	_ = uc.redisClient.Del(ctx, fmt.Sprintf("accounts:id:%d", accountID)).Err()

	return nil
}

// ===============================
// ACCOUNT QUERIES
// ===============================

// GetByFilter supports flexible filtering
func (uc *AccountUsecase) GetByFilter(ctx context.Context, filter *domain.AccountFilter) ([]*domain.Account, error) {
	accounts, err := uc.accountRepo.GetByFilter(ctx, filter)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.Account{}, nil
		}
		return nil, fmt.Errorf("failed to get accounts by filter: %w", err)
	}

	return accounts, nil
}

// GetByParentAgent fetches all accounts under an agent (agent hierarchy)
func (uc *AccountUsecase) GetByParentAgent(ctx context.Context, agentExternalID string) ([]*domain.Account, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("accounts:agent:%s", agentExternalID)
	
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var accounts []*domain.Account
		if jsonErr := json.Unmarshal([]byte(val), &accounts); jsonErr == nil {
			return accounts, nil
		}
	}

	accounts, err := uc.accountRepo.GetByParentAgent(ctx, agentExternalID)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.Account{}, nil
		}
		return nil, fmt.Errorf("failed to get accounts by parent agent: %w", err)
	}

	// Cache result (2 minutes)
	if data, err := json.Marshal(accounts); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 2*time.Minute).Err()
	}

	return accounts, nil
}

// ===============================
// VALIDATION HELPERS
// ===============================

// validateCreateAccountRequest validates account creation request
func (uc *AccountUsecase) validateCreateAccountRequest(req *domain.CreateAccountRequest) error {
	if req.OwnerType == "" {
		return fmt.Errorf("owner_type is required")
	}
	if req.OwnerID == "" {
		return fmt.Errorf("owner_id is required")
	}
	if req.Currency == "" || len(req.Currency) > 8 {
		return fmt.Errorf("invalid currency code")
	}
	if req.Purpose == "" {
		return fmt.Errorf("purpose is required")
	}
	if req.AccountType == "" {
		return fmt.Errorf("account_type is required")
	}

	// Demo accounts cannot have overdraft
	if req.AccountType == domain.AccountTypeDemo && req.OverdraftLimit > 0 {
		return fmt.Errorf("demo accounts cannot have overdraft limit")
	}

	// Agent accounts must have commission rate
	if req.OwnerType == domain.OwnerTypeAgent && req.CommissionRate == nil {
		return fmt.Errorf("agent accounts must have commission_rate")
	}

	// Real account fees only
	if req.AccountType == domain.AccountTypeDemo && req.Purpose == domain.PurposeFees {
		return fmt.Errorf("demo accounts cannot be fee accounts")
	}

	return nil
}

// ===============================
// CACHE MANAGEMENT
// ===============================

// InvalidateAccountCache invalidates all cache entries for an account
func (uc *AccountUsecase) InvalidateAccountCache(ctx context.Context, account *domain.Account) error {
	keys := []string{
		fmt.Sprintf("accounts:id:%d", account.ID),
		fmt.Sprintf("accounts:number:%s", account.AccountNumber),
		fmt.Sprintf("accounts:owner:%s:%s:%s", account.OwnerType, account.OwnerID, account.AccountType),
	}

	if account.ParentAgentExternalID != nil {
		keys = append(keys, fmt.Sprintf("accounts:agent:%s", *account.ParentAgentExternalID))
	}

	for _, key := range keys {
		if err := uc.redisClient.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key %s: %w", key, err)
		}
	}

	return nil
}

func (uc *AccountUsecase) generateAccountNumber(
	accountType domain.AccountType,
	purpose domain.AccountPurpose,
	ownerType domain.OwnerType,
) string {
	// Use different formats based on account type
	switch accountType {
	case domain.AccountTypeDemo:
		// Demo accounts get prefixed format with DEMO prefix
		return uc.accountNumberGen.GenerateDemoAccount()
		
	case domain.AccountTypeSystem:
		// System accounts get prefixed format with SYS prefix
		return uc.accountNumberGen.GenerateSystemAccount()
		
	case domain.AccountTypeReal:
		// Real accounts - different formats based on purpose/owner
		switch purpose {
		case domain.PurposeWallet:
			// Wallet purpose gets wallet-style address
			return uc.accountNumberGen.GenerateWalletAddress()
			
		case domain.PurposeSavings, domain.PurposeInvestment:
			// Savings/Investment get readable numeric format
			accountNumber, _ := uc.accountNumberGen.Generate(utils.FormatNumeric)
			return accountNumber
			
		default:
			// Default real accounts get standard ACC- prefix
			return uc.accountNumberGen.GenerateAccountNumber()
		}
		
	default:
		// Fallback to standard account number
		return uc.accountNumberGen.GenerateAccountNumber()
	}
}

// generateAccountNumberWithPrefix generates account number with custom prefix
// func (uc *AccountUsecase) generateAccountNumberWithPrefix(prefix string) string {
// 	accountNumber, _ := uc.accountNumberGen.Generate(utils.FormatPrefixed, prefix)
// 	return accountNumber
// }
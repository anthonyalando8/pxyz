package usecase

import (
	"context"
	"encoding/json"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type AccountUsecase struct {
	accountRepo repository.AccountRepository
	redisClient *redis.Client
}

// NewAccountUsecase initializes a new AccountUsecase
func NewAccountUsecase(accountRepo repository.AccountRepository, redisClient *redis.Client) *AccountUsecase {
	return &AccountUsecase{
		accountRepo: accountRepo,
		redisClient: redisClient,
	}
}

func (uc *AccountUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.accountRepo.BeginTx(ctx)
}


func (uc *AccountUsecase) GetSystemAccounts(ctx context.Context) ([]*domain.Account, error) {
	cacheKey := "accounts:system"

	// --- Check Redis cache first ---
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var accounts []*domain.Account
		if jsonErr := json.Unmarshal([]byte(val), &accounts); jsonErr == nil {
			return accounts, nil
		}
	}

	// --- Query repo with filter ---
	filter := &domain.AccountFilter{
		OwnerType: nullableStr("system"),
	}

	accounts, err := uc.accountRepo.GetByFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	// --- Cache result in Redis ---
	if data, err := json.Marshal(accounts); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return accounts, nil
}



// GetSystemAccount fetches a single system account by currency + purpose (cached via GetSystemAccounts)
func (uc *AccountUsecase) GetSystemAccount(ctx context.Context, currency, purpose string) (*domain.Account, error) {
	accounts, err := uc.GetSystemAccounts(ctx)
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		if acc.Currency == currency && acc.Purpose == purpose {
			return acc, nil
		}
	}

	return nil, xerrors.ErrNotFound
}

// GetByID fetches an account by its ID
func (uc *AccountUsecase) GetByAccountNumber(ctx context.Context, id string) (*domain.Account, error) {
	acc, err := uc.accountRepo.GetByAccountNumber(ctx, id)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

// GetByOwner fetches all accounts for a given owner
func (uc *AccountUsecase) GetByOwner(ctx context.Context, ownerType, ownerID string, tx pgx.Tx) ([]*domain.Account, error) {
	accounts, err := uc.accountRepo.GetOrCreateUserAccounts(ctx, ownerType, ownerID, tx)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.Account{}, nil // return empty slice instead of error
		}
		return nil, err
	}
	return accounts, nil
}

// CreateAccount creates a new account inside a transaction
// CreateAccounts creates multiple accounts inside a transaction.
// Returns a map of errors keyed by account index for failed inserts.
func (uc *AccountUsecase) CreateAccounts(ctx context.Context, accounts []*domain.Account, tx pgx.Tx) map[int]error {
	if len(accounts) == 0 {
		return map[int]error{0: xerrors.ErrInvalidRequest}
	}

	// Set defaults and validate each account
	for i, a := range accounts {
		if a.OwnerType == "" || a.OwnerID == "" || a.Currency == "" || a.Purpose == "" || a.AccountType == "" {
			return map[int]error{i: xerrors.ErrInvalidRequest}
		}

		if !a.IsActive {
			a.IsActive = true
		}

		a.CreatedAt = time.Now()
		a.UpdatedAt = time.Now()
	}

	// Delegate to repository batch insert
	return uc.accountRepo.CreateMany(ctx, accounts, tx)
}

// UpdateAccount updates an existing account inside a transaction
func (uc *AccountUsecase) UpdateAccount(ctx context.Context, a *domain.Account, tx pgx.Tx) error {
	a.UpdatedAt = time.Now()
	return uc.accountRepo.Update(ctx, a, tx)
}

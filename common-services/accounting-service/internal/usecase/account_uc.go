package usecase

import (
	"context"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
)

type AccountUsecase struct {
	accountRepo repository.AccountRepository
}

// NewAccountUsecase initializes a new AccountUsecase
func NewAccountUsecase(accountRepo repository.AccountRepository) *AccountUsecase {
	return &AccountUsecase{
		accountRepo: accountRepo,
	}
}

func (uc *AccountUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.accountRepo.BeginTx(ctx)
}


// GetByID fetches an account by its ID
func (uc *AccountUsecase) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	acc, err := uc.accountRepo.GetByID(ctx, id)
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

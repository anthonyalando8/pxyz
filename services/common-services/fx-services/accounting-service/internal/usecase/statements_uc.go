package usecase

import (
	"context"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

)

type StatementUsecase struct {
	statementRepo repository.StatementRepository
	redisClient *redis.Client
}

// NewStatementUsecase initializes the usecase
func NewStatementUsecase(statementRepo repository.StatementRepository, redisClient *redis.Client) *StatementUsecase {
	return &StatementUsecase{statementRepo: statementRepo, redisClient: redisClient,}
}

func (uc *StatementUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.statementRepo.BeginTx(ctx)
}

func (uc *StatementUsecase) GetAccountBalance(ctx context.Context, accountID string) (float64, error){
	balance, err := uc.statementRepo.GetCurrentBalance(ctx, accountID)
	if err != nil {
		return 0, err
	}
	return balance.Balance, nil
}

// GetAccountStatement fetches postings and current balance for a single account
func (uc *StatementUsecase) GetAccountStatement(ctx context.Context, accountID string, from, to time.Time) (*domain.AccountStatement, error) {
	postings, err := uc.statementRepo.ListPostingsByAccount(ctx, accountID, from, to)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	balance, err := uc.statementRepo.GetCurrentBalance(ctx, accountID)
	if err != nil {
		if err == xerrors.ErrNotFound {
			balance = &domain.Balance{AccountNumber: accountID, Balance: 0}
		} else {
			return nil, err
		}
	}

	return &domain.AccountStatement{
		AccountNumber: accountID,
		Postings:  postings,
		Balance:   balance.Balance,
	}, nil
}

// GetOwnerStatement aggregates postings for all accounts of an owner
func (uc *StatementUsecase) GetOwnerStatement(ctx context.Context, ownerType, ownerID string, from, to time.Time, accountUC *AccountUsecase, tx pgx.Tx) ([]*domain.AccountStatement, error) {
	accounts, err := accountUC.GetByOwner(ctx, ownerType, ownerID, tx)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return []*domain.AccountStatement{}, nil
	}

	var statements []*domain.AccountStatement
	for _, acct := range accounts {
		postings, err := uc.statementRepo.ListPostingsByAccount(ctx, acct.AccountNumber, from, to)
		if err != nil && err != xerrors.ErrNotFound {
			return nil, err
		}

		balance, err := uc.statementRepo.GetCurrentBalance(ctx, acct.AccountNumber)
		if err != nil && err != xerrors.ErrNotFound {
			return nil, err
		}

		bal := 0.0
		if balance != nil {
			bal = balance.Balance
		}

		statements = append(statements, &domain.AccountStatement{
			AccountID: acct.ID,
			Postings:  postings,
			Balance:   bal,
		})
	}

	return statements, nil
}


// GetJournalPostings fetches all postings for a specific journal
func (uc *StatementUsecase) GetJournalPostings(ctx context.Context, journalID int64) ([]*domain.Posting, error) {
	return uc.statementRepo.ListPostingsByJournal(ctx, journalID)
}

// GenerateDailyReport produces aggregated report for a specific date
func (uc *StatementUsecase) GenerateDailyReport(ctx context.Context, date time.Time) ([]*domain.DailyReport, error) {
	reports, err := uc.statementRepo.GetDailySummary(ctx, date)
	if err != nil {
		if err == xerrors.ErrNotFound {
			return []*domain.DailyReport{}, nil
		}
		return nil, err
	}
	return reports, nil
}

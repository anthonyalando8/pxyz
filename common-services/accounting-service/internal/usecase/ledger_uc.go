package usecase

import (
	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"
	"x/shared/utils/id"

	"github.com/jackc/pgx/v5"
)

type LedgerUsecase struct {
	ledgerRepo repository.LedgerRepository
	sf *id.Snowflake
}

func NewLedgerUsecase(
	ledgerRepo repository.LedgerRepository,
	sf *id.Snowflake,
) *LedgerUsecase {
	return &LedgerUsecase{
		ledgerRepo: ledgerRepo,
		sf: sf,
	}
}

func (uc *LedgerUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.ledgerRepo.BeginTx(ctx)
}

// CreateTransactionMulti handles a transaction with multiple postings
// Example: user deposit → credit user wallet + debit partner liquidity + fee
func (uc *LedgerUsecase) CreateTransactionMulti(
    ctx context.Context,
    journal *domain.Journal,
    postings []*domain.Posting,
	tx pgx.Tx,
) (journalID int64, err error) {

    if len(postings) == 0 {
        return 0, errors.New("no postings provided")
    }

    // Validate postings
    for _, p := range postings {
        if p.DrCr != "DR" && p.DrCr != "CR" {
            return 0, fmt.Errorf("invalid DR/CR for account %d", p.AccountID)
        }
        if p.Amount <= 0 {
            return 0, fmt.Errorf("amount must be positive for account %d", p.AccountID)
        }
        if p.Currency == "" {
            return 0, fmt.Errorf("currency required for account %d", p.AccountID)
        }
    }

    // Set journal defaults if needed
    if journal.IdempotencyKey == "" {
        journal.IdempotencyKey = uc.sf.Generate()
    }
    if journal.ExternalRef == "" {
        journal.ExternalRef = fmt.Sprintf("TX-%d", time.Now().UnixNano())
    }
    if journal.CreatedAt.IsZero() {
        journal.CreatedAt = time.Now()
    }

    journalID, err = uc.ledgerRepo.ApplyTransaction(ctx, journal, postings, tx)
    if err != nil {
        return 0, fmt.Errorf("failed to apply multi-posting transaction: %w", err)
    }

    return journalID, nil
}


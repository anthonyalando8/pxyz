package repository

import (
	"accounting-service/internal/domain"
	"context"
	"errors"

	//xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LedgerRepository interface {
	// ApplyTransaction: journal + postings + balance update in one atomic tx
	ApplyTransaction(ctx context.Context, journal *domain.Journal, postings []*domain.Posting, tx pgx.Tx) (int64, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type ledgerRepo struct {
	db          *pgxpool.Pool
	accountRepo AccountRepository
	journalRepo JournalRepository
	postingRepo PostingRepository
	balanceRepo BalanceRepository
}

func (r *ledgerRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return tx, nil
}
// ApplyTransaction implements LedgerRepository.
func (l *ledgerRepo) ApplyTransaction(
    ctx context.Context,
    journal *domain.Journal,
    postings []*domain.Posting,
    tx pgx.Tx,
) (int64, error) {
    var ownTx bool

    if tx == nil {
        var err error
        tx, err = l.db.BeginTx(ctx, pgx.TxOptions{})
        if err != nil {
            return 0, err
        }
        ownTx = true
        defer func() {
            if err != nil {
                tx.Rollback(ctx)
            }
        }()
    }

    // Create Journal
    if err := l.journalRepo.Create(ctx, journal, tx); err != nil {
        return 0, err
    }

    for _, p := range postings {
        if p.DrCr != "DR" && p.DrCr != "CR" {
            return 0, errors.New("invalid DR/CR value")
        }
        if p.Amount <= 0 {
            return 0, errors.New("posting amount must be positive")
        }

        // Ensure account exists (must also use tx!)
        account, err := l.accountRepo.GetByIDTx(ctx, p.AccountID, tx)
        if err != nil {
            return 0, err
        }
		p.JournalID = journal.ID
		
        if p.Currency == "" {
            p.Currency = account.Currency
        }

        if err = l.postingRepo.Create(ctx, p, tx); err != nil {
            return 0, err
        }
        if err = l.balanceRepo.UpdateBalance(ctx, p.AccountID, p.DrCr, p.Amount, tx); err != nil {
            return 0, err
        }
    }

    if ownTx {
        if err := tx.Commit(ctx); err != nil {
            return 0, err
        }
    }

    return journal.ID, nil
}


func NewLedgerRepo(
	db *pgxpool.Pool,
	ar AccountRepository,
	jr JournalRepository,
	pr PostingRepository,
	br BalanceRepository,
) LedgerRepository {
	return &ledgerRepo{
		db:          db,
		accountRepo: ar,
		journalRepo: jr,
		postingRepo: pr,
		balanceRepo: br,
	}
}

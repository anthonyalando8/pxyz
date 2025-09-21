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
	ApplyTransaction(ctx context.Context, journal *domain.Journal, postings []*domain.Posting, tx pgx.Tx) (*domain.Ledger, error)
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
) (*domain.Ledger, error) {
	var ownTx bool
	var err error

	if tx == nil {
		tx, err = l.db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return nil, err
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
		return nil, err
	}

	// Process postings
	for _, p := range postings {
		if p.DrCr != "DR" && p.DrCr != "CR" {
			return nil, errors.New("invalid DR/CR value")
		}
		if p.Amount <= 0 {
			return nil, errors.New("posting amount must be positive")
		}

		// Ensure account exists (use tx)
		account, err := l.accountRepo.GetByAccountNumberTx(ctx, p.AccountData.AccountNumber, tx)
		if err != nil {
			return nil, err
		}

		p.JournalID = journal.ID
		p.AccountID = account.ID

		if p.Currency == "" {
			p.Currency = account.Currency
		}

		// Save posting
		if err := l.postingRepo.Create(ctx, p, tx); err != nil {
			return nil, err
		}

		// Update balances
		if err := l.balanceRepo.UpdateBalance(ctx, p.AccountID, p.DrCr, p.Amount, tx); err != nil {
			return nil, err
		}

		// Attach account data to posting
		p.AccountData = account
	}

	if ownTx {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}

	ledger := &domain.Ledger{
		Journal:  journal,
		Postings: postings,
	}

	return ledger, nil
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

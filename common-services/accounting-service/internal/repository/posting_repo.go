package repository

import (
	"context"
	"accounting-service/internal/domain"
	"time"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	//xerrors "x/shared/utils/errors"
)

type PostingRepository interface {
	Create(ctx context.Context, p *domain.Posting, tx pgx.Tx) error
	//ListByJournal(ctx context.Context, journalID int64) ([]*domain.Posting, error)
	//ListByAccount(ctx context.Context, accountID int64) ([]*domain.Posting, error)
}

type postingRepo struct {
	db *pgxpool.Pool
}

func NewPostingRepo(db *pgxpool.Pool) PostingRepository {
	return &postingRepo{db: db}
}

// Create inserts a new posting inside a transaction
func (r *postingRepo) Create(ctx context.Context, p *domain.Posting, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	err := tx.QueryRow(ctx, `
		INSERT INTO postings (journal_id, account_id, amount, dr_cr, currency, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id
	`, p.JournalID, p.AccountID, p.Amount, p.DrCr, p.Currency, time.Now()).Scan(&p.ID)

	return err
}

// // ListByJournal fetches all postings for a given journal
// func (r *postingRepo) ListByJournal(ctx context.Context, journalID int64) ([]*domain.Posting, error) {
// 	rows, err := r.db.Query(ctx, `
// 		SELECT id, journal_id, account_id, amount, dr_cr, currency, created_at
// 		FROM postings
// 		WHERE journal_id=$1
// 		ORDER BY created_at ASC
// 	`, journalID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var postings []*domain.Posting
// 	for rows.Next() {
// 		var p domain.Posting
// 		if err := rows.Scan(&p.ID, &p.JournalID, &p.AccountID, &p.Amount, &p.DrCr, &p.Currency, &p.CreatedAt); err != nil {
// 			return nil, err
// 		}
// 		postings = append(postings, &p)
// 	}

// 	if len(postings) == 0 {
// 		return nil, xerrors.ErrNotFound
// 	}

// 	return postings, nil
// }

// // ListByAccount fetches all postings for a given account
// func (r *postingRepo) ListByAccount(ctx context.Context, accountID int64) ([]*domain.Posting, error) {
// 	rows, err := r.db.Query(ctx, `
// 		SELECT id, journal_id, account_id, amount, dr_cr, currency, created_at
// 		FROM postings
// 		WHERE account_id=$1
// 		ORDER BY created_at ASC
// 	`, accountID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var postings []*domain.Posting
// 	for rows.Next() {
// 		var p domain.Posting
// 		if err := rows.Scan(&p.ID, &p.JournalID, &p.AccountID, &p.Amount, &p.DrCr, &p.Currency, &p.CreatedAt); err != nil {
// 			return nil, err
// 		}
// 		postings = append(postings, &p)
// 	}

// 	if len(postings) == 0 {
// 		return nil, xerrors.ErrNotFound
// 	}

// 	return postings, nil
// }

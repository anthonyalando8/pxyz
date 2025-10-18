package repository

import (
	"context"
	//"log"
	"time"

	"accounting-service/internal/domain"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatementRepository interface {
	// Account-level
	ListPostingsByAccount(ctx context.Context, accountNumber string, from, to time.Time) ([]*domain.Posting, error)
	GetCurrentBalance(ctx context.Context, accountNumber string) (*domain.Balance, error)
	GetCachedBalance(ctx context.Context, accountID int64) (*domain.Balance, error)

	// Owner-level (user / partner)
	ListPostingsByOwner(ctx context.Context, ownerType, ownerID string, from, to time.Time) ([]*domain.Posting, error)

	// Journal-level (delegates to PostingRepo)
	ListPostingsByJournal(ctx context.Context, journalID int64) ([]*domain.Posting, error)

	// Reports
	GetDailySummary(ctx context.Context, date time.Time) ([]*domain.DailyReport, error)

	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type statementRepo struct {
	db      *pgxpool.Pool
	posting PostingRepository
}

func NewStatementRepo(db *pgxpool.Pool, postingRepo PostingRepository) StatementRepository {
	return &statementRepo{
		db:      db,
		posting: postingRepo,
	}
}

func (r *statementRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.BeginTx(ctx, pgx.TxOptions{})
}

// ---- Delegations ----

// ListPostingsByJournal delegates to PostingRepository
func (r *statementRepo) ListPostingsByJournal(ctx context.Context, journalID int64) ([]*domain.Posting, error) {
	return r.posting.ListByJournal(ctx, journalID)
}

// Account-level, still needs JOIN with accounts (not covered in PostingRepo)
func (r *statementRepo) ListPostingsByAccount(ctx context.Context, accountNumber string, from, to time.Time) ([]*domain.Posting, error) {
	rows, err := r.db.Query(ctx, `
		SELECT l.id, l.journal_id, l.account_id, l.amount, l.dr_cr, l.currency, l.receipt_code, l.created_at
		FROM ledgers l
		JOIN accounts a ON l.account_id = a.id
		WHERE a.account_number = $1
		  AND l.created_at BETWEEN $2 AND $3
		ORDER BY l.created_at ASC
	`, accountNumber, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postings []*domain.Posting
	for rows.Next() {
		var p domain.Posting
		var amount int64
		if err := rows.Scan(
			&p.ID,
			&p.JournalID,
			&p.AccountID,
			&amount,
			&p.DrCr,
			&p.Currency,
			&p.ReceiptCode,
			&p.CreatedAt,
		); err != nil {
			return nil, err
		}
		p.Amount = float64(amount)
		postings = append(postings, &p)
	}

	return postings, nil
}

// ---- Reports & Balances ----

func (r *statementRepo) GetDailySummary(ctx context.Context, date time.Time) ([]*domain.DailyReport, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour).Add(-time.Nanosecond)

	rows, err := r.db.Query(ctx, `
		SELECT 
			a.id AS account_id,
			a.account_number,
			a.owner_type,
			a.owner_id,
			a.currency,
			COALESCE(SUM(CASE WHEN l.dr_cr='DR' THEN l.amount ELSE 0 END),0) AS total_debit,
			COALESCE(SUM(CASE WHEN l.dr_cr='CR' THEN l.amount ELSE 0 END),0) AS total_credit
		FROM accounts a
		LEFT JOIN ledgers l ON l.account_id=a.id AND l.created_at BETWEEN $1 AND $2
		GROUP BY a.id, a.account_number, a.owner_type, a.owner_id, a.currency
	`, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*domain.DailyReport
	for rows.Next() {
		var rpt domain.DailyReport
		var totalDebit, totalCredit int64

		if err := rows.Scan(
			&rpt.AccountID,
			&rpt.AccountNumber,
			&rpt.OwnerType,
			&rpt.OwnerID,
			&rpt.Currency,
			&totalDebit,
			&totalCredit,
		); err != nil {
			return nil, err
		}

		balance, err := r.GetCurrentBalance(ctx, rpt.AccountNumber)
		if err != nil && err != xerrors.ErrNotFound {
			return nil, err
		}

		rpt.TotalDebit = float64(totalDebit)
		rpt.TotalCredit = float64(totalCredit)
		if balance != nil {
			rpt.Balance = balance.Balance
		}
		rpt.NetChange = rpt.TotalCredit - rpt.TotalDebit
		rpt.Date = startOfDay

		reports = append(reports, &rpt)
	}

	if len(reports) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return reports, nil
}

func (r *statementRepo) GetCurrentBalance(ctx context.Context, accountNumber string) (*domain.Balance, error) {
	var accountID int64
	err := r.db.QueryRow(ctx, `SELECT id FROM accounts WHERE account_number = $1`, accountNumber).Scan(&accountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	row := r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN dr_cr='CR' THEN amount ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN dr_cr='DR' THEN amount ELSE 0 END), 0) AS balance
		FROM ledgers
		WHERE account_id=$1
	`, accountID)

	var b domain.Balance
	b.AccountID = accountID

	var balance int64
	if err := row.Scan(&balance); err != nil {
		if err == pgx.ErrNoRows {
			b.Balance = 0
			b.UpdatedAt = time.Now()
			return &b, nil
		}
		return nil, err
	}

	b.Balance = float64(balance)
	b.UpdatedAt = time.Now()
	return &b, nil
}

func (r *statementRepo) GetCachedBalance(ctx context.Context, accountID int64) (*domain.Balance, error) {
	row := r.db.QueryRow(ctx, `
		SELECT account_id, balance, updated_at
		FROM balances
		WHERE account_id=$1
	`, accountID)

	var b domain.Balance
	if err := row.Scan(&b.AccountID, &b.Balance, &b.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	return &b, nil
}

// Owner-level aggregation
func (r *statementRepo) ListPostingsByOwner(ctx context.Context, ownerType, ownerID string, from, to time.Time) ([]*domain.Posting, error) {
	rows, err := r.db.Query(ctx, `
		SELECT 
			l.id, 
			l.journal_id, 
			l.account_id,
			l.amount, 
			l.dr_cr, 
			l.currency, 
			l.receipt_code, 
			l.created_at
		FROM ledgers l
		JOIN accounts a ON a.id = l.account_id
		WHERE a.owner_type=$1 AND a.owner_id=$2 AND l.created_at BETWEEN $3 AND $4
		ORDER BY l.created_at ASC
	`, ownerType, ownerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postings []*domain.Posting
	for rows.Next() {
		var p domain.Posting
		var amount int64
		if err := rows.Scan(
			&p.ID,
			&p.JournalID,
			&p.AccountID,
			&amount,
			&p.DrCr,
			&p.Currency,
			&p.ReceiptCode,
			&p.CreatedAt,
		); err != nil {
			return nil, err
		}
		p.Amount = float64(amount)
		postings = append(postings, &p)
	}

	return postings, nil
}

package repository

import (
	"context"
	"time"

	"accounting-service/internal/domain"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatementRepository interface {
	// Account-level
	ListPostingsByAccount(ctx context.Context, accountID int64, from, to time.Time) ([]*domain.Posting, error)
	GetCurrentBalance(ctx context.Context, accountID int64) (*domain.Balance, error)
	GetCachedBalance(ctx context.Context, accountID int64) (*domain.Balance, error)

	// Owner-level (user / partner)
	ListPostingsByOwner(ctx context.Context, ownerType, ownerID string, from, to time.Time) ([]*domain.Posting, error)

	// Journal-level (optional drill-down)
	ListPostingsByJournal(ctx context.Context, journalID int64) ([]*domain.Posting, error)

	// Reports
	GetDailySummary(ctx context.Context, date time.Time) ([]*domain.DailyReport, error)

	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type statementRepo struct {
	db *pgxpool.Pool
}

func (r *statementRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return tx, nil
}


// NewStatementRepo initializes StatementRepository
func NewStatementRepo(db *pgxpool.Pool) StatementRepository {
	return &statementRepo{db: db}
}

// ListPostingsByAccount returns postings for a single account
func (r *statementRepo) ListPostingsByAccount(ctx context.Context, accountID int64, from, to time.Time) ([]*domain.Posting, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, journal_id, account_id, amount, dr_cr, currency, receipt_id, created_at
		FROM postings
		WHERE account_id=$1 AND created_at BETWEEN $2 AND $3
		ORDER BY created_at ASC
	`, accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postings []*domain.Posting
	for rows.Next() {
		var p domain.Posting
		if err := rows.Scan(
			&p.ID, &p.JournalID, &p.AccountID, &p.Amount, &p.DrCr,
			&p.Currency, &p.ReceiptID, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		postings = append(postings, &p)
	}

	return postings, nil
}

// GetDailySummary implements StatementRepository.
func (r *statementRepo) GetDailySummary(ctx context.Context, date time.Time) ([]*domain.DailyReport, error) {
	// Start and end of the day
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour).Add(-time.Nanosecond)

	// Fetch daily debit/credit totals per account
	rows, err := r.db.Query(ctx, `
		SELECT 
			a.id AS account_id,
			a.owner_type,
			a.owner_id,
			a.currency,
			COALESCE(SUM(CASE WHEN p.dr_cr='DR' THEN p.amount ELSE 0 END),0) AS total_debit,
			COALESCE(SUM(CASE WHEN p.dr_cr='CR' THEN p.amount ELSE 0 END),0) AS total_credit
		FROM accounts a
		LEFT JOIN postings p ON p.account_id=a.id AND p.created_at BETWEEN $1 AND $2
		GROUP BY a.id, a.owner_type, a.owner_id, a.currency
	`, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*domain.DailyReport
	for rows.Next() {
		var rpt domain.DailyReport
		var totalDebit, totalCredit float64

		if err := rows.Scan(
			&rpt.AccountID,
			&rpt.OwnerType,
			&rpt.OwnerID,
			&rpt.Currency,
			&totalDebit,
			&totalCredit,
		); err != nil {
			return nil, err
		}

		// Compute balance from all postings
		balance, err := r.GetCurrentBalance(ctx, rpt.AccountID)
		if err != nil && err != xerrors.ErrNotFound {
			return nil, err
		}

		rpt.TotalDebit = totalDebit
		rpt.TotalCredit = totalCredit
		rpt.Balance = 0
		if balance != nil {
			rpt.Balance = balance.Balance
		}
		rpt.NetChange = totalCredit - totalDebit
		rpt.Date = startOfDay

		reports = append(reports, &rpt)
	}

	if len(reports) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return reports, nil
}


// GetCurrentBalance returns current balance for an account
func (r *statementRepo) GetCurrentBalance(ctx context.Context, accountID int64) (*domain.Balance, error) {
	row := r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN dr_cr='CR' THEN amount ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN dr_cr='DR' THEN amount ELSE 0 END), 0) AS balance
		FROM postings
		WHERE account_id=$1
	`, accountID)

	var b domain.Balance
	b.AccountID = accountID

	if err := row.Scan(&b.Balance); err != nil {
		if err == pgx.ErrNoRows {
			// No postings yet → balance is zero
			b.Balance = 0
			b.UpdatedAt = time.Now()
			return &b, nil
		}
		return nil, err
	}

	b.UpdatedAt = time.Now()
	return &b, nil
}


// GetCachedBalance fetches the current balance from the balances table.
// Falls back to ErrNotFound if no balance exists yet.
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


// ListPostingsByOwner aggregates postings for all accounts of an owner
func (r *statementRepo) ListPostingsByOwner(ctx context.Context, ownerType, ownerID string, from, to time.Time) ([]*domain.Posting, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.journal_id, p.account_id, p.amount, p.dr_cr, p.currency, p.receipt_id, p.created_at
		FROM postings p
		JOIN accounts a ON a.id = p.account_id
		WHERE a.owner_type=$1 AND a.owner_id=$2 AND p.created_at BETWEEN $3 AND $4
		ORDER BY p.created_at ASC
	`, ownerType, ownerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postings []*domain.Posting
	for rows.Next() {
		var p domain.Posting
		if err := rows.Scan(
			&p.ID, &p.JournalID, &p.AccountID, &p.Amount, &p.DrCr,
			&p.Currency, &p.ReceiptID, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		postings = append(postings, &p)
	}

	return postings, nil
}

// ListPostingsByJournal fetches postings for a specific journal
func (r *statementRepo) ListPostingsByJournal(ctx context.Context, journalID int64) ([]*domain.Posting, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, journal_id, account_id, amount, dr_cr, currency, receipt_id, created_at
		FROM postings
		WHERE journal_id=$1
		ORDER BY created_at ASC
	`, journalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postings []*domain.Posting
	for rows.Next() {
		var p domain.Posting
		if err := rows.Scan(
			&p.ID, &p.JournalID, &p.AccountID, &p.Amount, &p.DrCr,
			&p.Currency, &p.ReceiptID, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		postings = append(postings, &p)
	}

	return postings, nil
}

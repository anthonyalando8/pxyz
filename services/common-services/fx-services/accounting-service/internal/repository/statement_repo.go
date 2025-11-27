package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"accounting-service/internal/domain"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatementRepository interface {
	// Account-level statements
	GetAccountStatement(ctx context.Context, accountNumber string, accountType domain.AccountType, from, to time.Time) (*domain.AccountStatement, error)
	ListLedgersByAccount(ctx context.Context, accountNumber string, accountType domain.AccountType, from, to time.Time) ([]*domain.Ledger, error)
	
	// Owner-level statements
	ListLedgersByOwner(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, from, to time.Time) ([]*domain.Ledger, error)
	GetOwnerSummary(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType) (*domain.OwnerSummary, error)
	
	// Balance queries
	GetCurrentBalance(ctx context.Context, accountNumber string, accountType domain.AccountType) (*domain.Balance, error)
	GetCachedBalance(ctx context.Context, accountID int64) (*domain.Balance, error)
	
	// Reports and analytics
	GetDailySummary(ctx context.Context, date time.Time, accountType domain.AccountType) ([]*domain.DailyReport, error)
	GetTransactionSummary(ctx context.Context, accountType domain.AccountType, from, to time.Time) ([]*domain.TransactionSummary, error)
	GetOwnerDailySummary(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, date time.Time) (*domain.DailyReport, error)
	
	// Materialized view queries (fast aggregates)
	GetSystemHoldings(ctx context.Context, accountType domain.AccountType) (map[string]int64, error) // Currency -> Balance
	GetDailyTransactionVolume(ctx context.Context, accountType domain.AccountType, date time.Time) ([]*domain.TransactionSummary, error)
	
	// Transaction management
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type statementRepo struct {
	db         *pgxpool.Pool
	ledgerRepo LedgerRepository
}

func NewStatementRepo(db *pgxpool.Pool, ledgerRepo LedgerRepository) StatementRepository {
	return &statementRepo{
		db:         db,
		ledgerRepo: ledgerRepo,
	}
}

func (r *statementRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// ===============================
// ACCOUNT-LEVEL STATEMENTS
// ===============================

// GetAccountStatement generates a detailed account statement for a period
func (r *statementRepo) GetAccountStatement(ctx context.Context, accountNumber string, accountType domain.AccountType, from, to time.Time) (*domain.AccountStatement, error) {
	// Get account info
	var stmt domain.AccountStatement
	query := `
		SELECT 
			a.id, a.account_number, a.account_type, a.owner_type, a.owner_id, a.currency
		FROM accounts a
		WHERE a.account_number = $1 AND a.account_type = $2
	`
	
	err := r.db.QueryRow(ctx, query, accountNumber, accountType).Scan(
		&stmt.AccountID,
		&stmt.AccountNumber,
		&stmt.AccountType,
		&stmt.OwnerType,
		&stmt.OwnerID,
		&stmt.Currency,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	stmt.PeriodStart = from
	stmt.PeriodEnd = to

	// Get opening balance (balance at start of period)
	openingBalance, err := r.ledgerRepo.CalculateBalance(ctx, stmt.AccountID, &from)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate opening balance: %w", err)
	}
	stmt.OpeningBalance = openingBalance.Balance

	// Get ledger entries for the period
	ledgers, err := r.ListLedgersByAccount(ctx, accountNumber, accountType, from, to)
	if err != nil && !errors.Is(err, xerrors.ErrNotFound) {
		return nil, fmt.Errorf("failed to get ledger entries: %w", err)
	}
	stmt.Ledgers = ledgers

	// Calculate totals
	for _, ledger := range ledgers {
		if ledger.IsDebit() {
			stmt.TotalDebits += ledger.Amount
		} else {
			stmt.TotalCredits += ledger.Amount
		}
	}

	// Get closing balance (current balance)
	closingBalance, err := r.GetCurrentBalance(ctx, accountNumber, accountType)
	if err != nil && !errors.Is(err, xerrors.ErrNotFound) {
		return nil, fmt.Errorf("failed to get closing balance: %w", err)
	}
	if closingBalance != nil {
		stmt.ClosingBalance = closingBalance.Balance
	} else {
		stmt.ClosingBalance = stmt.OpeningBalance + stmt.TotalCredits - stmt.TotalDebits
	}

	return &stmt, nil
}

// ListLedgersByAccount fetches ledger entries for an account in a date range
func (r *statementRepo) ListLedgersByAccount(ctx context.Context, accountNumber string, accountType domain.AccountType, from, to time.Time) ([]*domain.Ledger, error) {
	// Uses idx_ledgers_real or idx_ledgers_demo for optimal performance
	query := `
		SELECT 
			l.id, l.journal_id, l.account_id, l.account_type, l.amount, l.dr_cr, 
			l.currency, l.receipt_code, l.balance_after, l.description, l.metadata, l.created_at
		FROM ledgers l
		JOIN accounts a ON l.account_id = a.id
		WHERE a.account_number = $1 
		  AND l.account_type = $2
		  AND l.created_at >= $3 
		  AND l.created_at <= $4
		ORDER BY l.created_at ASC
	`

	rows, err := r.db.Query(ctx, query, accountNumber, accountType, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by account: %w", err)
	}
	defer rows.Close()

	var ledgers []*domain.Ledger
	for rows.Next() {
		var l domain.Ledger
		err := rows.Scan(
			&l.ID,
			&l.JournalID,
			&l.AccountID,
			&l.AccountType,
			&l.Amount,
			&l.DrCr,
			&l.Currency,
			&l.ReceiptCode,
			&l.BalanceAfter,
			&l.Description,
			&l.Metadata,
			&l.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ledger: %w", err)
		}
		ledgers = append(ledgers, &l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger rows: %w", err)
	}

	if len(ledgers) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return ledgers, nil
}

// ===============================
// OWNER-LEVEL STATEMENTS
// ===============================

// ListLedgersByOwner fetches all ledger entries for an owner across all their accounts
func (r *statementRepo) ListLedgersByOwner(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, from, to time.Time) ([]*domain.Ledger, error) {
	query := `
		SELECT 
			l.id, l.journal_id, l.account_id, l.account_type, l.amount, l.dr_cr,
			l.currency, l.receipt_code, l.balance_after, l.description, l.metadata, l.created_at
		FROM ledgers l
		JOIN accounts a ON a.id = l.account_id
		WHERE a.owner_type = $1 
		  AND a.owner_id = $2 
		  AND l.account_type = $3
		  AND l.created_at >= $4 
		  AND l.created_at <= $5
		ORDER BY l.created_at ASC
	`

	rows, err := r.db.Query(ctx, query, ownerType, ownerID, accountType, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by owner: %w", err)
	}
	defer rows.Close()

	var ledgers []*domain.Ledger
	for rows.Next() {
		var l domain.Ledger
		err := rows.Scan(
			&l.ID,
			&l.JournalID,
			&l.AccountID,
			&l.AccountType,
			&l.Amount,
			&l.DrCr,
			&l.Currency,
			&l.ReceiptCode,
			&l.BalanceAfter,
			&l.Description,
			&l.Metadata,
			&l.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ledger: %w", err)
		}
		ledgers = append(ledgers, &l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger rows: %w", err)
	}

	return ledgers, nil
}

// GetOwnerSummary returns aggregated summary for all accounts owned by an entity
func (r *statementRepo) GetOwnerSummary(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
) (*domain.OwnerSummary, error) {
	summary := &domain.OwnerSummary{
		OwnerType:   ownerType,
		OwnerID:     ownerID,
		AccountType: accountType,
		Balances:    []*domain.AccountBalanceSummary{},
	}

	// Get all accounts with balances for this owner
	query := `
		SELECT 
			a.id,
			a.account_number,
			a.currency,
			COALESCE(b.balance, 0) AS balance,
			COALESCE(b.available_balance, 0) AS available_balance
		FROM accounts a
		LEFT JOIN balances b ON b.account_id = a.id
		WHERE a.owner_type = $1 
		  AND a.owner_id = $2 
		  AND a.account_type = $3
		ORDER BY a.currency, a.account_number
	`

	rows, err := r.db.Query(ctx, query, ownerType, ownerID, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to query owner accounts: %w", err)
	}
	defer rows.Close()

	var totalBalanceUSD int64 // You may want to calculate USD equivalent here

	for rows.Next() {
		var balance domain.AccountBalanceSummary
		err := rows.Scan(
			&balance.AccountID,
			&balance.AccountNumber,
			&balance.Currency,
			&balance.Balance,
			&balance.AvailableBalance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account balance: %w", err)
		}

		summary.Balances = append(summary.Balances, &balance)
		
		// TODO: Convert to USD equivalent if currency != USD
		// For now, just sum all balances (assumes USD or add conversion logic)
		if balance.Currency == "USD" {
			totalBalanceUSD += balance.Balance
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating account rows: %w", err)
	}

	if len(summary.Balances) == 0 {
		return nil, xerrors.ErrNotFound
	}

	summary.TotalBalance = totalBalanceUSD

	return summary, nil
}

// ===============================
// BALANCE QUERIES
// ===============================

// GetCurrentBalance calculates current balance from ledger entries
func (r *statementRepo) GetCurrentBalance(ctx context.Context, accountNumber string, accountType domain.AccountType) (*domain.Balance, error) {
	var accountID int64
	query := `SELECT id FROM accounts WHERE account_number = $1 AND account_type = $2`
	err := r.db.QueryRow(ctx, query, accountNumber, accountType).Scan(&accountID)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	// Calculate balance from ledger
	balanceQuery := `
		SELECT 
			COALESCE(SUM(CASE WHEN dr_cr='CR' THEN amount ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN dr_cr='DR' THEN amount ELSE 0 END), 0) AS balance
		FROM ledgers
		WHERE account_id = $1 AND account_type = $2
	`

	var balance domain.Balance
	balance.AccountID = accountID

	var balanceAmount int64
	err = r.db.QueryRow(ctx, balanceQuery, accountID, accountType).Scan(&balanceAmount)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			balance.Balance = 0
			balance.UpdatedAt = time.Now()
			return &balance, nil
		}
		return nil, fmt.Errorf("failed to calculate balance: %w", err)
	}

	balance.Balance = balanceAmount
	balance.UpdatedAt = time.Now()
	
	return &balance, nil
}

// GetCachedBalance fetches balance from cached balances table (fast)
func (r *statementRepo) GetCachedBalance(ctx context.Context, accountID int64) (*domain.Balance, error) {
	query := `
		SELECT 
			account_id, balance, available_balance, pending_debit, pending_credit,
			last_ledger_id, version, updated_at
		FROM balances
		WHERE account_id = $1
	`

	var b domain.Balance
	err := r.db.QueryRow(ctx, query, accountID).Scan(
		&b.AccountID,
		&b.Balance,
		&b.AvailableBalance,
		&b.PendingDebit,
		&b.PendingCredit,
		&b.LastLedgerID,
		&b.Version,
		&b.UpdatedAt,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get cached balance: %w", err)
	}

	return &b, nil
}

// ===============================
// REPORTS AND ANALYTICS
// ===============================

// GetDailySummary generates daily summary report for all accounts
func (r *statementRepo) GetDailySummary(ctx context.Context, date time.Time, accountType domain.AccountType) ([]*domain.DailyReport, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour).Add(-time.Nanosecond)

	query := `
		SELECT 
			a.id AS account_id,
			a.account_number,
			a.account_type,
			a.owner_type,
			a.owner_id,
			a.currency,
			COALESCE(SUM(CASE WHEN l.dr_cr='DR' THEN l.amount ELSE 0 END), 0) AS total_debit,
			COALESCE(SUM(CASE WHEN l.dr_cr='CR' THEN l.amount ELSE 0 END), 0) AS total_credit,
			COALESCE(b.balance, 0) AS balance
		FROM accounts a
		LEFT JOIN ledgers l ON l.account_id = a.id 
			AND l.created_at >= $1 
			AND l.created_at <= $2
			AND l.account_type = $3
		LEFT JOIN balances b ON b.account_id = a.id
		WHERE a.account_type = $3
		GROUP BY a.id, a.account_number, a.account_type, a.owner_type, a.owner_id, a.currency, b.balance
		ORDER BY a.owner_type, a.owner_id, a.currency
	`

	rows, err := r.db.Query(ctx, query, startOfDay, endOfDay, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summary: %w", err)
	}
	defer rows.Close()

	var reports []*domain.DailyReport
	for rows.Next() {
		var rpt domain.DailyReport
		err := rows.Scan(
			&rpt.AccountID,
			&rpt.AccountNumber,
			&rpt.AccountType,
			&rpt.OwnerType,
			&rpt.OwnerID,
			&rpt.Currency,
			&rpt.TotalDebit,
			&rpt.TotalCredit,
			&rpt.Balance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily report: %w", err)
		}

		rpt.NetChange = rpt.TotalCredit - rpt.TotalDebit
		rpt.Date = startOfDay

		reports = append(reports, &rpt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily report rows: %w", err)
	}

	if len(reports) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return reports, nil
}

// GetOwnerDailySummary generates daily summary for a specific owner
func (r *statementRepo) GetOwnerDailySummary(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, date time.Time) (*domain.DailyReport, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour).Add(-time.Nanosecond)

	query := `
		SELECT 
			a.owner_type,
			a.owner_id,
			a.currency,
			COALESCE(SUM(CASE WHEN l.dr_cr='DR' THEN l.amount ELSE 0 END), 0) AS total_debit,
			COALESCE(SUM(CASE WHEN l.dr_cr='CR' THEN l.amount ELSE 0 END), 0) AS total_credit,
			COALESCE(SUM(b.balance), 0) AS total_balance
		FROM accounts a
		LEFT JOIN ledgers l ON l.account_id = a.id 
			AND l.created_at >= $3 
			AND l.created_at <= $4
			AND l.account_type = $5
		LEFT JOIN balances b ON b.account_id = a.id
		WHERE a.owner_type = $1 
		  AND a.owner_id = $2
		  AND a.account_type = $5
		GROUP BY a.owner_type, a.owner_id, a.currency
	`

	var rpt domain.DailyReport
	err := r.db.QueryRow(ctx, query, ownerType, ownerID, startOfDay, endOfDay, accountType).Scan(
		&rpt.OwnerType,
		&rpt.OwnerID,
		&rpt.Currency,
		&rpt.TotalDebit,
		&rpt.TotalCredit,
		&rpt.Balance,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get owner daily summary: %w", err)
	}

	rpt.AccountType = accountType
	rpt.NetChange = rpt.TotalCredit - rpt.TotalDebit
	rpt.Date = startOfDay

	return &rpt, nil
}

// GetTransactionSummary returns transaction statistics for a period
func (r *statementRepo) GetTransactionSummary(ctx context.Context, accountType domain.AccountType, from, to time.Time) ([]*domain.TransactionSummary, error) {
	query := `
		SELECT 
			j.transaction_type,
			l.currency,
			COUNT(*) AS count,
			SUM(l.amount) AS total_volume,
			AVG(l.amount) AS avg_amount,
			MIN(l.amount) AS min_amount,
			MAX(l.amount) AS max_amount
		FROM ledgers l
		JOIN journals j ON j.id = l.journal_id
		WHERE l.account_type = $1
		  AND l.created_at >= $2
		  AND l.created_at <= $3
		GROUP BY j.transaction_type, l.currency
		ORDER BY total_volume DESC
	`

	rows, err := r.db.Query(ctx, query, accountType, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction summary: %w", err)
	}
	defer rows.Close()

	var summaries []*domain.TransactionSummary
	for rows.Next() {
		var s domain.TransactionSummary
		s.AccountType = accountType
		s.PeriodStart = from
		s.PeriodEnd = to

		err := rows.Scan(
			&s.TransactionType,
			&s.Currency,
			&s.Count,
			&s.TotalVolume,
			&s.AverageAmount,
			&s.MinAmount,
			&s.MaxAmount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction summary: %w", err)
		}

		summaries = append(summaries, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transaction summary rows: %w", err)
	}

	return summaries, nil
}

// ===============================
// MATERIALIZED VIEW QUERIES
// ===============================

// GetSystemHoldings returns total holdings by currency (uses materialized view)
func (r *statementRepo) GetSystemHoldings(ctx context.Context, accountType domain.AccountType) (map[string]int64, error) {
	// Uses materialized view: system_holdings_real
	// Note: Schema only has system_holdings_real, not separate for demo
	// For demo, we'll query accounts directly
	
	var query string
	if accountType == domain.AccountTypeReal {
		query = `
			SELECT currency, total_balance
			FROM system_holdings_real
			ORDER BY total_balance DESC
		`
	} else {
		// For demo accounts, query directly
		query = `
			SELECT a.currency, SUM(COALESCE(b.balance, 0)) AS total_balance
			FROM accounts a
			LEFT JOIN balances b ON b.account_id = a.id
			WHERE a.account_type = 'demo' AND a.is_active = true
			GROUP BY a.currency
			ORDER BY total_balance DESC
		`
	}

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get system holdings: %w", err)
	}
	defer rows.Close()

	holdings := make(map[string]int64)
	for rows.Next() {
		var currency string
		var balance int64
		
		err := rows.Scan(&currency, &balance)
		if err != nil {
			return nil, fmt.Errorf("failed to scan holding: %w", err)
		}

		holdings[currency] = balance
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating holdings rows: %w", err)
	}

	return holdings, nil
}

// GetDailyTransactionVolume returns transaction volume for a specific date (uses materialized view)
func (r *statementRepo) GetDailyTransactionVolume(ctx context.Context, accountType domain.AccountType, date time.Time) ([]*domain.TransactionSummary, error) {
	// Uses materialized view: daily_transaction_volume_real
	// Note: Schema only has real version
	
	if accountType != domain.AccountTypeReal {
		// Demo accounts don't have materialized view, return empty
		return []*domain.TransactionSummary{}, nil
	}

	query := `
		SELECT 
			transaction_type,
			currency,
			transaction_count,
			total_volume,
			avg_transaction_size
		FROM daily_transaction_volume_real
		WHERE transaction_date = $1
		ORDER BY total_volume DESC
	`

	rows, err := r.db.Query(ctx, query, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily transaction volume: %w", err)
	}
	defer rows.Close()

	var summaries []*domain.TransactionSummary
	for rows.Next() {
		var s domain.TransactionSummary
		s.AccountType = accountType
		s.PeriodStart = date
		s.PeriodEnd = date.Add(24 * time.Hour)

		err := rows.Scan(
			&s.TransactionType,
			&s.Currency,
			&s.Count,
			&s.TotalVolume,
			&s.AverageAmount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily volume: %w", err)
		}

		summaries = append(summaries, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily volume rows: %w", err)
	}

	return summaries, nil
}
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"accounting-service/internal/domain"

	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LedgerRepository interface {
	// Basic CRUD
	Create(ctx context.Context, tx pgx.Tx, ledger *domain.LedgerCreate) (*domain.Ledger, error)
	CreateBatch(ctx context.Context, tx pgx.Tx, ledgers []*domain.LedgerCreate) ([]*domain.Ledger, map[int]error)
	CreatePairedEntry(ctx context.Context, tx pgx.Tx, debit, credit *domain.LedgerCreate) (*domain.Ledger, *domain.Ledger, error)
	GetByID(ctx context.Context, id int64) (*domain.Ledger, error)
	
	// Query operations
	List(ctx context.Context, filter *domain.LedgerFilter) ([]*domain.Ledger, error)
	ListByJournal(ctx context.Context, journalID int64) ([]*domain.Ledger, error)
	ListByAccount(ctx context.Context, accountNumber string, accountType domain.AccountType, from, to *time.Time, limit, offset int) ([]*domain.Ledger, int, error)
	ListByReceipt(ctx context.Context, receiptCode string) ([]*domain.Ledger, error)
	ListByOwner(ctx context.Context, ownerType domain.OwnerType, ownerID string, accountType domain.AccountType, from, to time.Time) ([]*domain.Ledger, error)
	
	// Balance operations
	CalculateBalance(ctx context.Context, accountID int64, upToTime *time.Time) (*domain.LedgerBalance, error)
	GetLastLedgerID(ctx context.Context, accountID int64) (*int64, error)
	
	// Statistics
	GetAccountActivity(ctx context.Context, accountID int64, startDate, endDate time.Time) (debits, credits int64, err error)
	GetTransactionVolume(ctx context.Context, accountType domain.AccountType, startDate, endDate time.Time) (int64, error)
}

type ledgerRepo struct {
	db *pgxpool.Pool
}

func NewLedgerRepo(db *pgxpool.Pool) LedgerRepository {
	return &ledgerRepo{db: db}
}

// Create inserts a new ledger entry inside a transaction
func (r *ledgerRepo) Create(ctx context.Context, tx pgx.Tx, ledger *domain.LedgerCreate) (*domain.Ledger, error) {
	if tx == nil {
		return nil, errors.New("transaction cannot be nil")
	}

	// Validate currency code length
	if len(ledger.Currency) > 8 {
		return nil, errors.New("currency code must be 8 characters or less")
	}

	// Validate amount
	if ledger.Amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	// Validate DR/CR
	if ledger.DrCr != domain.DrCrDebit && ledger.DrCr != domain.DrCrCredit {
		return nil, domain.ErrInvalidDrCr
	}

	query := `
		INSERT INTO ledgers (
			journal_id, account_id, account_type, amount, dr_cr, currency,
			receipt_code, balance_after, description, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`

	now := time.Now()
	var l domain.Ledger
	l.JournalID = ledger.JournalID
	l.AccountID = ledger.AccountID
	l.AccountType = ledger.AccountType
	l.Amount = ledger.Amount
	l.DrCr = ledger.DrCr
	l.Currency = ledger.Currency
	l.ReceiptCode = ledger.ReceiptCode
	l.BalanceAfter = ledger.BalanceAfter
	l.Description = ledger.Description
	l.Metadata = ledger.Metadata

	err := tx.QueryRow(ctx, query,
		ledger.JournalID,
		ledger.AccountID,
		ledger.AccountType,
		ledger.Amount,
		ledger.DrCr,
		ledger.Currency,
		ledger.ReceiptCode,
		ledger.BalanceAfter,
		ledger.Description,
		ledger.Metadata,
		now,
	).Scan(&l.ID, &l.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create ledger entry: %w", err)
	}

	return &l, nil
}

// CreateBatch creates multiple ledger entries in a single batch (bulk insert)
func (r *ledgerRepo) CreateBatch(ctx context.Context, tx pgx.Tx, ledgers []*domain.LedgerCreate) ([]*domain.Ledger, map[int]error) {
	if tx == nil {
		return nil, map[int]error{0: errors.New("transaction cannot be nil")}
	}

	if len(ledgers) == 0 {
		return []*domain.Ledger{}, nil
	}

	errs := make(map[int]error)
	results := make([]*domain.Ledger, 0, len(ledgers))
	
	batch := &pgx.Batch{}
	query := `
		INSERT INTO ledgers (
			journal_id, account_id, account_type, amount, dr_cr, currency,
			receipt_code, balance_after, description, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`

	now := time.Now()
	validLedgers := make([]*domain.LedgerCreate, 0, len(ledgers))
	indexMap := make(map[int]int) // Maps batch index to original index

	for i, ledger := range ledgers {
		// Validate
		if len(ledger.Currency) > 8 {
			errs[i] = errors.New("currency code must be 8 characters or less")
			continue
		}
		if ledger.Amount <= 0 {
			errs[i] = errors.New("amount must be positive")
			continue
		}
		if ledger.DrCr != domain.DrCrDebit && ledger.DrCr != domain.DrCrCredit {
			errs[i] = domain.ErrInvalidDrCr
			continue
		}

		batch.Queue(query,
			ledger.JournalID,
			ledger.AccountID,
			ledger.AccountType,
			ledger.Amount,
			ledger.DrCr,
			ledger.Currency,
			ledger.ReceiptCode,
			ledger.BalanceAfter,
			ledger.Description,
			ledger.Metadata,
			now,
		)

		indexMap[len(validLedgers)] = i
		validLedgers = append(validLedgers, ledger)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for batchIdx := 0; batchIdx < len(validLedgers); batchIdx++ {
		originalIdx := indexMap[batchIdx]
		ledger := validLedgers[batchIdx]

		var l domain.Ledger
		l.JournalID = ledger.JournalID
		l.AccountID = ledger.AccountID
		l.AccountType = ledger.AccountType
		l.Amount = ledger.Amount
		l.DrCr = ledger.DrCr
		l.Currency = ledger.Currency
		l.ReceiptCode = ledger.ReceiptCode
		l.BalanceAfter = ledger.BalanceAfter
		l.Description = ledger.Description
		l.Metadata = ledger.Metadata

		err := br.QueryRow().Scan(&l.ID, &l.CreatedAt)
		if err != nil {
			errs[originalIdx] = fmt.Errorf("failed to create ledger entry: %w", err)
			continue
		}

		results = append(results, &l)
	}

	return results, errs
}

// CreatePairedEntry creates a balanced debit/credit pair
func (r *ledgerRepo) CreatePairedEntry(ctx context.Context, tx pgx.Tx, debit, credit *domain.LedgerCreate) (*domain.Ledger, *domain.Ledger, error) {
	if tx == nil {
		return nil, nil, errors.New("transaction cannot be nil")
	}

	// Validate paired entry
	if err := domain.ValidatePairedEntry(debit, credit); err != nil {
		return nil, nil, err
	}

	// Create debit
	debitLedger, err := r.Create(ctx, tx, debit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create debit entry: %w", err)
	}

	// Create credit
	creditLedger, err := r.Create(ctx, tx, credit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create credit entry: %w", err)
	}

	return debitLedger, creditLedger, nil
}

// GetByID fetches a ledger entry by its ID
func (r *ledgerRepo) GetByID(ctx context.Context, id int64) (*domain.Ledger, error) {
	query := `
		SELECT 
			id, journal_id, account_id, account_type, amount, dr_cr, currency,
			receipt_code, balance_after, description, metadata, created_at
		FROM ledgers
		WHERE id = $1
	`

	var l domain.Ledger
	err := r.db.QueryRow(ctx, query, id).Scan(
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get ledger entry: %w", err)
	}

	return &l, nil
}

// List fetches ledger entries based on filter criteria
func (r *ledgerRepo) List(ctx context.Context, filter *domain.LedgerFilter) ([]*domain.Ledger, error) {
	query := `
		SELECT 
			id, journal_id, account_id, account_type, amount, dr_cr, currency,
			receipt_code, balance_after, description, metadata, created_at
		FROM ledgers
		WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	if filter.AccountID != nil {
		query += fmt.Sprintf(" AND account_id = $%d", argPos)
		args = append(args, *filter.AccountID)
		argPos++
	}

	if filter.JournalID != nil {
		query += fmt.Sprintf(" AND journal_id = $%d", argPos)
		args = append(args, *filter.JournalID)
		argPos++
	}

	if filter.AccountType != nil {
		query += fmt.Sprintf(" AND account_type = $%d", argPos)
		args = append(args, *filter.AccountType)
		argPos++
	}

	if filter.Currency != nil {
		query += fmt.Sprintf(" AND currency = $%d", argPos)
		args = append(args, *filter.Currency)
		argPos++
	}

	if filter.DrCr != nil {
		query += fmt.Sprintf(" AND dr_cr = $%d", argPos)
		args = append(args, *filter.DrCr)
		argPos++
	}

	if filter.ReceiptCode != nil {
		query += fmt.Sprintf(" AND receipt_code = $%d", argPos)
		args = append(args, *filter.ReceiptCode)
		argPos++
	}

	if filter.StartDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argPos)
		args = append(args, *filter.StartDate)
		argPos++
	}

	if filter.EndDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argPos)
		args = append(args, *filter.EndDate)
		argPos++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
		argPos++
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledger entries: %w", err)
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
			return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		ledgers = append(ledgers, &l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger rows: %w", err)
	}

	return ledgers, nil
}

// ListByJournal fetches all ledger entries for a given journal
func (r *ledgerRepo) ListByJournal(ctx context.Context, journalID int64) ([]*domain.Ledger, error) {
	query := `
		SELECT 
			id, journal_id, account_id, account_type, amount, dr_cr, currency,
			receipt_code, balance_after, description, metadata, created_at
		FROM ledgers
		WHERE journal_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, journalID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by journal: %w", err)
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
			return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		ledgers = append(ledgers, &l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger rows: %w", err)
	}

	return ledgers, nil
}

// ListByAccount fetches all ledger entries for a given account (uses indexed query)
func (r *ledgerRepo) ListByAccount(
	ctx context.Context,
	accountNumber string,
	accountType domain.AccountType,
	from, to *time.Time,
	limit, offset int,
) ([]*domain.Ledger, int, error) {
	// Set defaults
	if limit <= 0 {
		limit = 1000
	}
	if limit > 1000 {
		limit = 1000 // Max limit
	}
	if offset < 0 {
		offset = 0
	}

	// First, get account ID from account number
	var accountID int64
	accountQuery := `SELECT id FROM accounts WHERE account_number = $1 AND account_type = $2`
	err := r.db.QueryRow(ctx, accountQuery, accountNumber, accountType).Scan(&accountID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find account: %w", err)
	}

	// Build query with optional date filters
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT 
			id, journal_id, account_id, account_type, amount, dr_cr, currency,
			receipt_code, balance_after, description, metadata, created_at
		FROM ledgers
		WHERE account_id = $1 AND account_type = $2
	`)

	// Count query for pagination
	countBuilder := strings.Builder{}
	countBuilder.WriteString(`
		SELECT COUNT(*) 
		FROM ledgers
		WHERE account_id = $1 AND account_type = $2
	`)

	args := []interface{}{accountID, accountType}
	argPos := 3

	// Add date filters if provided
	if from != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at >= $%d", argPos))
		countBuilder.WriteString(fmt.Sprintf(" AND created_at >= $%d", argPos))
		args = append(args, *from)
		argPos++
	}

	if to != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at <= $%d", argPos))
		countBuilder.WriteString(fmt.Sprintf(" AND created_at <= $%d", argPos))
		args = append(args, *to)
		argPos++
	}

	// Add ordering and pagination
	queryBuilder.WriteString(` ORDER BY created_at DESC`)
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1))

	// Get total count
	var total int
	countArgs := args[:argPos-1] // Exclude limit and offset for count
	err = r.db.QueryRow(ctx, countBuilder.String(), countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count ledgers: %w", err)
	}

	// Get paginated results
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list ledgers by account: %w", err)
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
			return nil, 0, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		ledgers = append(ledgers, &l)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating ledger rows: %w", err)
	}

	return ledgers, total, nil
}

func (r *ledgerRepo) ListByOwner(
	ctx context.Context,
	ownerType domain.OwnerType,
	ownerID string,
	accountType domain.AccountType,
	from, to time.Time,
) ([]*domain.Ledger, error) {
	// First, get all account IDs for this owner
	accountQuery := `
		SELECT id 
		FROM accounts 
		WHERE owner_type = $1 AND owner_id = $2 AND account_type = $3
	`
	
	rows, err := r.db.Query(ctx, accountQuery, ownerType, ownerID, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get owner accounts: %w", err)
	}
	defer rows.Close()

	var accountIDs []int64
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, fmt.Errorf("failed to scan account ID: %w", err)
		}
		accountIDs = append(accountIDs, accountID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating account rows: %w", err)
	}

	// If no accounts found, return empty result
	if len(accountIDs) == 0 {
		return []*domain.Ledger{}, nil
	}

	// Build query for ledgers with date range
	// Uses ANY() for efficient IN clause with array
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT 
			l.id, l.journal_id, l.account_id, l.account_type, l.amount, l.dr_cr, l.currency,
			l.receipt_code, l.balance_after, l.description, l.metadata, l.created_at
		FROM ledgers l
		WHERE l.account_id = ANY($1)
			AND l.account_type = $2
			AND l.created_at >= $3
			AND l.created_at <= $4
		ORDER BY l.created_at DESC
	`)

	ledgerRows, err := r.db.Query(
		ctx,
		queryBuilder.String(),
		accountIDs,
		accountType,
		from,
		to,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by owner: %w", err)
	}
	defer ledgerRows.Close()

	var ledgers []*domain.Ledger
	for ledgerRows.Next() {
		var l domain.Ledger
		err := ledgerRows.Scan(
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
			return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		ledgers = append(ledgers, &l)
	}

	if err := ledgerRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger rows: %w", err)
	}

	return ledgers, nil
}

// ListByReceipt fetches all ledger entries linked to a specific receipt (uses indexed query)
func (r *ledgerRepo) ListByReceipt(ctx context.Context, receiptCode string) ([]*domain.Ledger, error) {
	// Uses idx_ledgers_receipt index
	query := `
		SELECT 
			id, journal_id, account_id, account_type, amount, dr_cr, currency,
			receipt_code, balance_after, description, metadata, created_at
		FROM ledgers
		WHERE receipt_code = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, receiptCode)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledgers by receipt: %w", err)
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
			return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		ledgers = append(ledgers, &l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger rows: %w", err)
	}

	return ledgers, nil
}

// CalculateBalance calculates account balance from ledger entries
func (r *ledgerRepo) CalculateBalance(ctx context.Context, accountID int64, upToTime *time.Time) (*domain.LedgerBalance, error) {
	query := `
		SELECT 
			account_id,
			currency,
			COALESCE(SUM(CASE WHEN dr_cr = 'DR' THEN amount ELSE 0 END), 0) AS total_debits,
			COALESCE(SUM(CASE WHEN dr_cr = 'CR' THEN amount ELSE 0 END), 0) AS total_credits,
			MAX(id) AS last_ledger_id,
			MAX(created_at) AS last_updated
		FROM ledgers
		WHERE account_id = $1
	`

	args := []interface{}{accountID}
	
	if upToTime != nil {
		query += " AND created_at <= $2"
		args = append(args, *upToTime)
	}

	query += " GROUP BY account_id, currency"

	var balance domain.LedgerBalance
	var currency string
	var lastUpdated time.Time

	err := r.db.QueryRow(ctx, query, args...).Scan(
		&balance.AccountID,
		&currency,
		&balance.TotalDebits,
		&balance.TotalCredits,
		&balance.LastLedgerID,
		&lastUpdated,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No ledger entries for this account
			return &domain.LedgerBalance{
				AccountID:    accountID,
				TotalDebits:  0,
				TotalCredits: 0,
				Balance:      0,
				LastUpdated:  time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to calculate balance: %w", err)
	}

	balance.Currency = currency
	balance.Balance = balance.TotalCredits - balance.TotalDebits
	balance.LastUpdated = lastUpdated

	return &balance, nil
}

// GetLastLedgerID fetches the ID of the most recent ledger entry for an account
func (r *ledgerRepo) GetLastLedgerID(ctx context.Context, accountID int64) (*int64, error) {
	query := `
		SELECT id
		FROM ledgers
		WHERE account_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`

	var id int64
	err := r.db.QueryRow(ctx, query, accountID).Scan(&id)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No ledger entries yet
		}
		return nil, fmt.Errorf("failed to get last ledger ID: %w", err)
	}

	return &id, nil
}

// GetAccountActivity returns total debits and credits for an account in a date range
func (r *ledgerRepo) GetAccountActivity(ctx context.Context, accountID int64, startDate, endDate time.Time) (debits, credits int64, err error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN dr_cr = 'DR' THEN amount ELSE 0 END), 0) AS total_debits,
			COALESCE(SUM(CASE WHEN dr_cr = 'CR' THEN amount ELSE 0 END), 0) AS total_credits
		FROM ledgers
		WHERE account_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`

	err = r.db.QueryRow(ctx, query, accountID, startDate, endDate).Scan(&debits, &credits)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get account activity: %w", err)
	}

	return debits, credits, nil
}

// GetTransactionVolume returns total transaction volume for an account type in a date range
func (r *ledgerRepo) GetTransactionVolume(ctx context.Context, accountType domain.AccountType, startDate, endDate time.Time) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0) AS total_volume
		FROM ledgers
		WHERE account_type = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`

	var volume int64
	err := r.db.QueryRow(ctx, query, accountType, startDate, endDate).Scan(&volume)
	
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction volume: %w", err)
	}

	return volume, nil
}
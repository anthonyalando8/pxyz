package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"accounting-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	xerrors "x/shared/utils/errors"
)

type JournalRepository interface {
	// Basic CRUD
	Create(ctx context.Context, tx pgx.Tx, journal *domain.JournalCreate) (*domain.Journal, error)
	CreateBatch(ctx context.Context, tx pgx.Tx, journals []*domain.JournalCreate) ([]*domain.Journal, map[int]error)
	GetByID(ctx context.Context, id int64) (*domain.Journal, error)
	GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Journal, error)
	
	// Query operations
	List(ctx context.Context, filter *domain.JournalFilter) ([]*domain.Journal, error)
	ListByAccount(ctx context.Context, accountID int64, accountType domain.AccountType) ([]*domain.Journal, error)
	ListByTransactionType(ctx context.Context, transactionType domain.TransactionType, accountType domain.AccountType, limit int) ([]*domain.Journal, error)
	ListByExternalRef(ctx context.Context, externalRef string) ([]*domain.Journal, error)
	
	// Statistics
	CountByType(ctx context.Context, accountType domain.AccountType, startDate, endDate time.Time) (map[domain.TransactionType]int64, error)
}

type journalRepo struct {
	db *pgxpool.Pool
}

func NewJournalRepo(db *pgxpool.Pool) JournalRepository {
	return &journalRepo{db: db}
}

// Create inserts a new journal entry inside a transaction
func (r *journalRepo) Create(ctx context.Context, tx pgx.Tx, journal *domain.JournalCreate) (*domain.Journal, error) {
	if tx == nil {
		return nil, errors.New("transaction cannot be nil")
	}

	// Validate transaction type for account type
	if journal.AccountType == domain.AccountTypeDemo {
		restrictedTypes := []domain.TransactionType{
			domain.TransactionTypeDeposit,
			domain.TransactionTypeWithdrawal,
			domain.TransactionTypeTransfer,
			domain.TransactionTypeFee,
			domain.TransactionTypeCommission,
		}
		
		for _, restricted := range restrictedTypes {
			if journal.TransactionType == restricted {
				return nil, xerrors.ErrInvalidTransactionType
			}
		}
	}

	query := `
		INSERT INTO journals (
			idempotency_key, transaction_type, account_type, external_ref,
			description, created_by_external_id, created_by_type,
			ip_address, user_agent, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	now := time.Now()
	var j domain.Journal
	j.IdempotencyKey = journal.IdempotencyKey
	j.TransactionType = journal.TransactionType
	j.AccountType = journal.AccountType
	j.ExternalRef = journal.ExternalRef
	j.Description = journal.Description
	j.CreatedByExternalID = journal.CreatedByExternalID
	j.CreatedByType = journal.CreatedByType
	j.IPAddress = journal.IPAddress
	j.UserAgent = journal.UserAgent

	err := tx.QueryRow(ctx, query,
		journal.IdempotencyKey,
		journal.TransactionType,
		journal.AccountType,
		journal.ExternalRef,
		journal.Description,
		journal.CreatedByExternalID,
		journal.CreatedByType,
		journal.IPAddress,
		journal.UserAgent,
		now,
	).Scan(&j.ID, &j.CreatedAt)

	if err != nil {
		// Check for idempotency key violation
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			if pgErr.ConstraintName == "journals_idempotency_key_key" {
				return nil, xerrors.ErrDuplicateIdempotencyKey
			}
		}
		return nil, fmt.Errorf("failed to create journal: %w", err)
	}

	return &j, nil
}

// CreateBatch creates multiple journals in a single batch (bulk insert)
func (r *journalRepo) CreateBatch(ctx context.Context, tx pgx.Tx, journals []*domain.JournalCreate) ([]*domain.Journal, map[int]error) {
	if tx == nil {
		return nil, map[int]error{0: errors.New("transaction cannot be nil")}
	}

	if len(journals) == 0 {
		return []*domain.Journal{}, nil
	}

	errs := make(map[int]error)
	results := make([]*domain.Journal, 0, len(journals))
	
	batch := &pgx.Batch{}
	query := `
		INSERT INTO journals (
			idempotency_key, transaction_type, account_type, external_ref,
			description, created_by_external_id, created_by_type,
			ip_address, user_agent, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	now := time.Now()
	validJournals := make([]*domain.JournalCreate, 0, len(journals))
	indexMap := make(map[int]int) // Maps batch index to original index

	for i, j := range journals {
		// Validate transaction type
		if j.AccountType == domain.AccountTypeDemo {
			restrictedTypes := []domain.TransactionType{
				domain.TransactionTypeDeposit,
				domain.TransactionTypeWithdrawal,
				domain.TransactionTypeTransfer,
				domain.TransactionTypeFee,
				domain.TransactionTypeCommission,
			}
			
			isRestricted := false
			for _, restricted := range restrictedTypes {
				if j.TransactionType == restricted {
					errs[i] = xerrors.ErrInvalidTransactionType
					isRestricted = true
					break
				}
			}
			
			if isRestricted {
				continue
			}
		}

		batch.Queue(query,
			j.IdempotencyKey,
			j.TransactionType,
			j.AccountType,
			j.ExternalRef,
			j.Description,
			j.CreatedByExternalID,
			j.CreatedByType,
			j.IPAddress,
			j.UserAgent,
			now,
		)

		indexMap[len(validJournals)] = i
		validJournals = append(validJournals, j)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for batchIdx := 0; batchIdx < len(validJournals); batchIdx++ {
		originalIdx := indexMap[batchIdx]
		j := validJournals[batchIdx]

		var journal domain.Journal
		journal.IdempotencyKey = j.IdempotencyKey
		journal.TransactionType = j.TransactionType
		journal.AccountType = j.AccountType
		journal.ExternalRef = j.ExternalRef
		journal.Description = j.Description
		journal.CreatedByExternalID = j.CreatedByExternalID
		journal.CreatedByType = j.CreatedByType
		journal.IPAddress = j.IPAddress
		journal.UserAgent = j.UserAgent

		err := br.QueryRow().Scan(&journal.ID, &journal.CreatedAt)
		if err != nil {
			// Check for idempotency key violation
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				if pgErr.ConstraintName == "journals_idempotency_key_key" {
					errs[originalIdx] = xerrors.ErrDuplicateIdempotencyKey
				} else {
					errs[originalIdx] = fmt.Errorf("unique constraint violation: %w", err)
				}
			} else {
				errs[originalIdx] = fmt.Errorf("failed to create journal: %w", err)
			}
			continue
		}

		results = append(results, &journal)
	}

	return results, errs
}

// GetByID fetches a journal by its ID
func (r *journalRepo) GetByID(ctx context.Context, id int64) (*domain.Journal, error) {
	query := `
		SELECT 
			id, idempotency_key, transaction_type, account_type, external_ref,
			description, created_by_external_id, created_by_type,
			ip_address, user_agent, created_at
		FROM journals
		WHERE id = $1
	`

	var j domain.Journal
	err := r.db.QueryRow(ctx, query, id).Scan(
		&j.ID,
		&j.IdempotencyKey,
		&j.TransactionType,
		&j.AccountType,
		&j.ExternalRef,
		&j.Description,
		&j.CreatedByExternalID,
		&j.CreatedByType,
		&j.IPAddress,
		&j.UserAgent,
		&j.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get journal: %w", err)
	}

	return &j, nil
}

// GetByIdempotencyKey fetches a journal by idempotency key (for duplicate detection)
func (r *journalRepo) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Journal, error) {
	query := `
		SELECT 
			id, idempotency_key, transaction_type, account_type, external_ref,
			description, created_by_external_id, created_by_type,
			ip_address, user_agent, created_at
		FROM journals
		WHERE idempotency_key = $1
	`

	var j domain.Journal
	err := r.db.QueryRow(ctx, query, idempotencyKey).Scan(
		&j.ID,
		&j.IdempotencyKey,
		&j.TransactionType,
		&j.AccountType,
		&j.ExternalRef,
		&j.Description,
		&j.CreatedByExternalID,
		&j.CreatedByType,
		&j.IPAddress,
		&j.UserAgent,
		&j.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get journal by idempotency key: %w", err)
	}

	return &j, nil
}

// List fetches journals based on filter criteria
func (r *journalRepo) List(ctx context.Context, filter *domain.JournalFilter) ([]*domain.Journal, error) {
	query := `
		SELECT 
			id, idempotency_key, transaction_type, account_type, external_ref,
			description, created_by_external_id, created_by_type,
			ip_address, user_agent, created_at
		FROM journals
		WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	if filter.TransactionType != nil {
		query += fmt.Sprintf(" AND transaction_type = $%d", argPos)
		args = append(args, *filter.TransactionType)
		argPos++
	}

	if filter.AccountType != nil {
		query += fmt.Sprintf(" AND account_type = $%d", argPos)
		args = append(args, *filter.AccountType)
		argPos++
	}

	if filter.ExternalRef != nil {
		query += fmt.Sprintf(" AND external_ref = $%d", argPos)
		args = append(args, *filter.ExternalRef)
		argPos++
	}

	if filter.CreatedByID != nil {
		query += fmt.Sprintf(" AND created_by_external_id = $%d", argPos)
		args = append(args, *filter.CreatedByID)
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
		return nil, fmt.Errorf("failed to list journals: %w", err)
	}
	defer rows.Close()

	var journals []*domain.Journal
	for rows.Next() {
		var j domain.Journal
		err := rows.Scan(
			&j.ID,
			&j.IdempotencyKey,
			&j.TransactionType,
			&j.AccountType,
			&j.ExternalRef,
			&j.Description,
			&j.CreatedByExternalID,
			&j.CreatedByType,
			&j.IPAddress,
			&j.UserAgent,
			&j.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan journal: %w", err)
		}
		journals = append(journals, &j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating journal rows: %w", err)
	}

	return journals, nil
}

// ListByAccount fetches all journals linked to ledgers of a specific account
func (r *journalRepo) ListByAccount(ctx context.Context, accountID int64, accountType domain.AccountType) ([]*domain.Journal, error) {
	query := `
		SELECT DISTINCT 
			j.id, j.idempotency_key, j.transaction_type, j.account_type, j.external_ref,
			j.description, j.created_by_external_id, j.created_by_type,
			j.ip_address, j.user_agent, j.created_at
		FROM journals j
		JOIN ledgers l ON l.journal_id = j.id
		WHERE l.account_id = $1 AND j.account_type = $2
		ORDER BY j.created_at DESC
		LIMIT 1000
	`

	rows, err := r.db.Query(ctx, query, accountID, accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to list journals by account: %w", err)
	}
	defer rows.Close()

	var journals []*domain.Journal
	for rows.Next() {
		var j domain.Journal
		err := rows.Scan(
			&j.ID,
			&j.IdempotencyKey,
			&j.TransactionType,
			&j.AccountType,
			&j.ExternalRef,
			&j.Description,
			&j.CreatedByExternalID,
			&j.CreatedByType,
			&j.IPAddress,
			&j.UserAgent,
			&j.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan journal: %w", err)
		}
		journals = append(journals, &j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating journal rows: %w", err)
	}

	return journals, nil
}

// ListByTransactionType fetches journals by transaction type (uses index idx_journals_type_real/demo)
func (r *journalRepo) ListByTransactionType(ctx context.Context, transactionType domain.TransactionType, accountType domain.AccountType, limit int) ([]*domain.Journal, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT 
			id, idempotency_key, transaction_type, account_type, external_ref,
			description, created_by_external_id, created_by_type,
			ip_address, user_agent, created_at
		FROM journals
		WHERE transaction_type = $1 AND account_type = $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, transactionType, accountType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list journals by transaction type: %w", err)
	}
	defer rows.Close()

	var journals []*domain.Journal
	for rows.Next() {
		var j domain.Journal
		err := rows.Scan(
			&j.ID,
			&j.IdempotencyKey,
			&j.TransactionType,
			&j.AccountType,
			&j.ExternalRef,
			&j.Description,
			&j.CreatedByExternalID,
			&j.CreatedByType,
			&j.IPAddress,
			&j.UserAgent,
			&j.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan journal: %w", err)
		}
		journals = append(journals, &j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating journal rows: %w", err)
	}

	return journals, nil
}

// ListByExternalRef fetches all journals with a specific external reference
func (r *journalRepo) ListByExternalRef(ctx context.Context, externalRef string) ([]*domain.Journal, error) {
	query := `
		SELECT 
			id, idempotency_key, transaction_type, account_type, external_ref,
			description, created_by_external_id, created_by_type,
			ip_address, user_agent, created_at
		FROM journals
		WHERE external_ref = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, externalRef)
	if err != nil {
		return nil, fmt.Errorf("failed to list journals by external ref: %w", err)
	}
	defer rows.Close()

	var journals []*domain.Journal
	for rows.Next() {
		var j domain.Journal
		err := rows.Scan(
			&j.ID,
			&j.IdempotencyKey,
			&j.TransactionType,
			&j.AccountType,
			&j.ExternalRef,
			&j.Description,
			&j.CreatedByExternalID,
			&j.CreatedByType,
			&j.IPAddress,
			&j.UserAgent,
			&j.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan journal: %w", err)
		}
		journals = append(journals, &j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating journal rows: %w", err)
	}

	return journals, nil
}

// CountByType returns count of journals grouped by transaction type for a date range
func (r *journalRepo) CountByType(ctx context.Context, accountType domain.AccountType, startDate, endDate time.Time) (map[domain.TransactionType]int64, error) {
	query := `
		SELECT transaction_type, COUNT(*) as count
		FROM journals
		WHERE account_type = $1
		  AND created_at >= $2
		  AND created_at <= $3
		GROUP BY transaction_type
		ORDER BY count DESC
	`

	rows, err := r.db.Query(ctx, query, accountType, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to count journals by type: %w", err)
	}
	defer rows.Close()

	result := make(map[domain.TransactionType]int64)
	for rows.Next() {
		var txType domain.TransactionType
		var count int64
		
		err := rows.Scan(&txType, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		
		result[txType] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating count rows: %w", err)
	}

	return result, nil
}
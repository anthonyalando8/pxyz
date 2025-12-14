package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"accounting-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	xerrors "x/shared/utils/errors"
)

type TransactionFeeRuleRepository interface {
	// Basic CRUD
	Create(ctx context.Context, tx pgx.Tx, rule *domain.FeeRuleCreate) (*domain.TransactionFeeRule, error)
	CreateBatch(ctx context.Context, tx pgx.Tx, rules []*domain.FeeRuleCreate) ([]*domain.TransactionFeeRule, map[int]error)
	Update(ctx context.Context, tx pgx.Tx, rule *domain.TransactionFeeRule) error
	GetByID(ctx context.Context, id int64) (*domain.TransactionFeeRule, error)
	Delete(ctx context.Context, tx pgx.Tx, id int64) error
	
	// Query operations
	List(ctx context.Context, filter *domain.FeeRuleFilter) ([]*domain.TransactionFeeRule, error)
	ListActive(ctx context.Context) ([]*domain.TransactionFeeRule, error)
	
	// Priority-based matching (critical for fee calculation)
	FindBestMatch(ctx context.Context, transactionType domain.TransactionType, sourceCurrency, targetCurrency *string, accountType *domain.AccountType, ownerType *domain.OwnerType) (*domain.TransactionFeeRule, error)
	FindAllMatches(ctx context.Context, transactionType domain.TransactionType, sourceCurrency, targetCurrency *string, accountType *domain.AccountType, ownerType *domain.OwnerType) ([]*domain.TransactionFeeRule, error)
	
	// Expiration
	ExpireRule(ctx context.Context, tx pgx.Tx, id int64, validTo time.Time) error
	
	// Transaction management
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type transactionFeeRuleRepo struct {
	db *pgxpool.Pool
}

func NewTransactionFeeRuleRepo(db *pgxpool.Pool) TransactionFeeRuleRepository {
	return &transactionFeeRuleRepo{db: db}
}

func (r *transactionFeeRuleRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// ===============================
// BASIC CRUD
// ===============================

// Create inserts a new fee rule
// Create inserts a new fee rule
func (r *transactionFeeRuleRepo) Create(ctx context.Context, tx pgx. Tx, rule *domain.FeeRuleCreate) (*domain.TransactionFeeRule, error) {
	if tx == nil {
		return nil, errors.New("transaction cannot be nil")
	}

	// Validate currency code lengths
	if rule.SourceCurrency != nil && len(*rule.SourceCurrency) > 8 {
		return nil, errors.New("source currency code must be 8 characters or less")
	}
	if rule.TargetCurrency != nil && len(*rule.TargetCurrency) > 8 {
		return nil, errors. New("target currency code must be 8 characters or less")
	}

	query := `
		INSERT INTO transaction_fee_rules (
			rule_name, transaction_type, source_currency, target_currency,
			account_type, owner_type, fee_type, calculation_method,
			fee_value, min_fee, max_fee, tiers, tariffs,
			valid_from, valid_to, is_active, priority, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id, created_at, updated_at
	`

	now := time.Now()
	if rule.ValidFrom. IsZero() {
		rule.ValidFrom = now
	}

	var feeRule domain.TransactionFeeRule
	feeRule.RuleName = rule.RuleName
	feeRule.TransactionType = rule. TransactionType
	feeRule.SourceCurrency = rule. SourceCurrency
	feeRule.TargetCurrency = rule.TargetCurrency
	feeRule.AccountType = rule.AccountType
	feeRule.OwnerType = rule.OwnerType
	feeRule.FeeType = rule.FeeType
	feeRule.CalculationMethod = rule.CalculationMethod
	feeRule.FeeValue = rule.FeeValue
	feeRule.MinFee = rule. MinFee
	feeRule.MaxFee = rule.MaxFee
	feeRule. Tiers = rule.Tiers
	feeRule. Tariffs = rule.Tariffs // ✅ NEW
	feeRule.ValidFrom = rule.ValidFrom
	feeRule.ValidTo = rule.ValidTo
	feeRule.IsActive = rule.IsActive
	feeRule.Priority = rule. Priority

	err := tx.QueryRow(ctx, query,
		rule.RuleName,
		rule.TransactionType,
		rule.SourceCurrency,
		rule.TargetCurrency,
		rule.AccountType,
		rule.OwnerType,
		rule.FeeType,
		rule. CalculationMethod,
		rule. FeeValue,
		rule. MinFee,
		rule. MaxFee,
		rule. Tiers,
		rule. Tariffs, // ✅ NEW
		rule.ValidFrom,
		rule.ValidTo,
		rule. IsActive,
		rule.Priority,
		now,
		now,
	).Scan(&feeRule.ID, &feeRule.CreatedAt, &feeRule.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create fee rule: %w", err)
	}

	return &feeRule, nil
}

// CreateBatch creates multiple fee rules (bulk insert)
// CreateBatch creates multiple fee rules (bulk insert)
func (r *transactionFeeRuleRepo) CreateBatch(ctx context.Context, tx pgx.Tx, rules []*domain.FeeRuleCreate) ([]*domain.TransactionFeeRule, map[int]error) {
	if tx == nil {
		return nil, map[int]error{0: errors.New("transaction cannot be nil")}
	}

	if len(rules) == 0 {
		return []*domain.TransactionFeeRule{}, nil
	}

	errs := make(map[int]error)
	results := make([]*domain.TransactionFeeRule, 0, len(rules))

	batch := &pgx.Batch{}
	query := `
		INSERT INTO transaction_fee_rules (
			rule_name, transaction_type, source_currency, target_currency,
			account_type, owner_type, fee_type, calculation_method,
			fee_value, min_fee, max_fee, tiers, tariffs,
			valid_from, valid_to, is_active, priority, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id, created_at, updated_at
	`

	now := time.Now()
	validRules := make([]*domain.FeeRuleCreate, 0, len(rules))
	indexMap := make(map[int]int)

	for i, rule := range rules {
		// Validate currency codes
		if rule.SourceCurrency != nil && len(*rule. SourceCurrency) > 8 {
			errs[i] = errors.New("source currency code must be 8 characters or less")
			continue
		}
		if rule.TargetCurrency != nil && len(*rule.TargetCurrency) > 8 {
			errs[i] = errors.New("target currency code must be 8 characters or less")
			continue
		}

		if rule.ValidFrom.IsZero() {
			rule.ValidFrom = now
		}

		batch.Queue(query,
			rule.RuleName,
			rule.TransactionType,
			rule.SourceCurrency,
			rule.TargetCurrency,
			rule. AccountType,
			rule. OwnerType,
			rule. FeeType,
			rule. CalculationMethod,
			rule.FeeValue,
			rule.MinFee,
			rule.MaxFee,
			rule.Tiers,
			rule.Tariffs, // ✅ NEW
			rule.ValidFrom,
			rule.ValidTo,
			rule. IsActive,
			rule.Priority,
			now,
			now,
		)

		indexMap[len(validRules)] = i
		validRules = append(validRules, rule)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for batchIdx := 0; batchIdx < len(validRules); batchIdx++ {
		originalIdx := indexMap[batchIdx]
		rule := validRules[batchIdx]

		var feeRule domain.TransactionFeeRule
		feeRule. RuleName = rule.RuleName
		feeRule.TransactionType = rule.TransactionType
		feeRule.SourceCurrency = rule.SourceCurrency
		feeRule. TargetCurrency = rule. TargetCurrency
		feeRule.AccountType = rule.AccountType
		feeRule. OwnerType = rule.OwnerType
		feeRule. FeeType = rule.FeeType
		feeRule. CalculationMethod = rule.CalculationMethod
		feeRule. FeeValue = rule.FeeValue
		feeRule.MinFee = rule.MinFee
		feeRule.MaxFee = rule.MaxFee
		feeRule.Tiers = rule.Tiers
		feeRule.Tariffs = rule.Tariffs // ✅ NEW
		feeRule.ValidFrom = rule.ValidFrom
		feeRule.ValidTo = rule.ValidTo
		feeRule.IsActive = rule. IsActive
		feeRule. Priority = rule.Priority

		err := br.QueryRow().Scan(&feeRule.ID, &feeRule.CreatedAt, &feeRule.UpdatedAt)
		if err != nil {
			errs[originalIdx] = fmt.Errorf("failed to create fee rule: %w", err)
			continue
		}

		results = append(results, &feeRule)
	}

	return results, errs
}

// Update updates an existing fee rule
// Update updates an existing fee rule
func (r *transactionFeeRuleRepo) Update(ctx context. Context, tx pgx.Tx, rule *domain.TransactionFeeRule) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	query := `
		UPDATE transaction_fee_rules
		SET 
			rule_name = $2,
			fee_value = $3,
			min_fee = $4,
			max_fee = $5,
			tiers = $6,
			tariffs = $7,
			valid_to = $8,
			is_active = $9,
			priority = $10,
			updated_at = $11
		WHERE id = $1
	`

	cmdTag, err := tx. Exec(ctx, query,
		rule.ID,
		rule. RuleName,
		rule. FeeValue,
		rule. MinFee,
		rule. MaxFee,
		rule. Tiers,
		rule. Tariffs, // ✅ NEW
		rule.ValidTo,
		rule.IsActive,
		rule. Priority,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update fee rule: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// GetByID fetches a fee rule by ID
func (r *transactionFeeRuleRepo) GetByID(ctx context.Context, id int64) (*domain.TransactionFeeRule, error) {
	query := `
		SELECT 
			id, rule_name, transaction_type, source_currency, target_currency,
			account_type, owner_type, fee_type, calculation_method,
			fee_value, min_fee, max_fee, tiers, tariffs,
			valid_from, valid_to, is_active, priority, created_at, updated_at
		FROM transaction_fee_rules
		WHERE id = $1
	`

	var rule domain.TransactionFeeRule
	err := r.db. QueryRow(ctx, query, id).Scan(
		&rule.ID,
		&rule. RuleName,
		&rule.TransactionType,
		&rule.SourceCurrency,
		&rule.TargetCurrency,
		&rule.AccountType,
		&rule. OwnerType,
		&rule.FeeType,
		&rule.CalculationMethod,
		&rule.FeeValue,
		&rule.MinFee,
		&rule.MaxFee,
		&rule. Tiers,
		&rule. Tariffs, // ✅ NEW
		&rule.ValidFrom,
		&rule.ValidTo,
		&rule.IsActive,
		&rule.Priority,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors. ErrNotFound
		}
		return nil, fmt.Errorf("failed to get fee rule:  %w", err)
	}

	return &rule, nil
}

// Delete deletes a fee rule
func (r *transactionFeeRuleRepo) Delete(ctx context.Context, tx pgx.Tx, id int64) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	cmdTag, err := tx.Exec(ctx, `DELETE FROM transaction_fee_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete fee rule: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// ===============================
// QUERY OPERATIONS
// ===============================

// List fetches fee rules based on filter criteria
func (r *transactionFeeRuleRepo) List(ctx context.Context, filter *domain.FeeRuleFilter) ([]*domain.TransactionFeeRule, error) {
	query := `
		SELECT 
			id, rule_name, transaction_type, source_currency, target_currency,
			account_type, owner_type, fee_type, calculation_method,
			fee_value, min_fee, max_fee, tiers, tariffs,
			valid_from, valid_to, is_active, priority, created_at, updated_at
		FROM transaction_fee_rules
		WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	if filter.TransactionType != nil {
		query += fmt.Sprintf(" AND transaction_type = $%d", argPos)
		args = append(args, *filter.TransactionType)
		argPos++
	}

	if filter.SourceCurrency != nil {
		query += fmt.Sprintf(" AND source_currency = $%d", argPos)
		args = append(args, *filter.SourceCurrency)
		argPos++
	}

	if filter.TargetCurrency != nil {
		query += fmt.Sprintf(" AND target_currency = $%d", argPos)
		args = append(args, *filter.TargetCurrency)
		argPos++
	}

	if filter.AccountType != nil {
		query += fmt.Sprintf(" AND account_type = $%d", argPos)
		args = append(args, *filter.AccountType)
		argPos++
	}

	if filter.OwnerType != nil {
		query += fmt.Sprintf(" AND owner_type = $%d", argPos)
		args = append(args, *filter.OwnerType)
		argPos++
	}

	if filter.FeeType != nil {
		query += fmt.Sprintf(" AND fee_type = $%d", argPos)
		args = append(args, *filter.FeeType)
		argPos++
	}

	if filter.IsActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argPos)
		args = append(args, *filter.IsActive)
		argPos++
	}

	if filter.ValidAt != nil {
		query += fmt.Sprintf(" AND valid_from <= $%d AND (valid_to IS NULL OR valid_to > $%d)", argPos, argPos)
		args = append(args, *filter.ValidAt)
		argPos++
	}

	query += " ORDER BY priority DESC, created_at DESC"

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
		return nil, fmt. Errorf("failed to list fee rules: %w", err)
	}
	defer rows.Close()

	var rules []*domain. TransactionFeeRule
	for rows.Next() {
		var rule domain.TransactionFeeRule
		err := rows.Scan(
			&rule.ID,
			&rule.RuleName,
			&rule.TransactionType,
			&rule. SourceCurrency,
			&rule.TargetCurrency,
			&rule.AccountType,
			&rule.OwnerType,
			&rule.FeeType,
			&rule. CalculationMethod,
			&rule.FeeValue,
			&rule.MinFee,
			&rule.MaxFee,
			&rule.Tiers,
			&rule.Tariffs, // ✅ NEW
			&rule.ValidFrom,
			&rule.ValidTo,
			&rule.IsActive,
			&rule.Priority,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan fee rule: %w", err)
		}
		rules = append(rules, &rule)
	}

	return rules, rows.Err()
}

// ListActive fetches all active fee rules (uses idx_fee_rules_lookup)
func (r *transactionFeeRuleRepo) ListActive(ctx context. Context) ([]*domain.TransactionFeeRule, error) {
	query := `
		SELECT 
			id, rule_name, transaction_type, source_currency, target_currency,
			account_type, owner_type, fee_type, calculation_method,
			fee_value, min_fee, max_fee, tiers, tariffs,
			valid_from, valid_to, is_active, priority, created_at, updated_at
		FROM transaction_fee_rules
		WHERE is_active = true AND valid_to IS NULL
		ORDER BY priority DESC, transaction_type
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt. Errorf("failed to list active fee rules: %w", err)
	}
	defer rows. Close()

	var rules []*domain.TransactionFeeRule
	for rows.Next() {
		var rule domain.TransactionFeeRule
		err := rows. Scan(
			&rule. ID,
			&rule.RuleName,
			&rule. TransactionType,
			&rule.SourceCurrency,
			&rule.TargetCurrency,
			&rule.AccountType,
			&rule.OwnerType,
			&rule. FeeType,
			&rule.CalculationMethod,
			&rule.FeeValue,
			&rule.MinFee,
			&rule.MaxFee,
			&rule.Tiers,
			&rule. Tariffs, // ✅ NEW
			&rule.ValidFrom,
			&rule.ValidTo,
			&rule.IsActive,
			&rule.Priority,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan fee rule: %w", err)
		}
		rules = append(rules, &rule)
	}

	return rules, rows.Err()
}

// ===============================
// PRIORITY-BASED MATCHING
// ===============================

// FindBestMatch finds the best matching fee rule based on priority
// Returns the FIRST match (highest priority) or nil if no match
func (r *transactionFeeRuleRepo) FindBestMatch(ctx context.Context, transactionType domain.TransactionType, sourceCurrency, targetCurrency *string, accountType *domain.AccountType, ownerType *domain.OwnerType) (*domain.TransactionFeeRule, error) {
	query := `
		SELECT 
			id, rule_name, transaction_type, source_currency, target_currency,
			account_type, owner_type, fee_type, calculation_method,
			fee_value, min_fee, max_fee, tiers, tariffs,
			valid_from, valid_to, is_active, priority, created_at, updated_at
		FROM transaction_fee_rules
		WHERE is_active = true 
		  AND valid_to IS NULL
		  AND transaction_type = $1
		  AND (source_currency IS NULL OR source_currency = $2)
		  AND (target_currency IS NULL OR target_currency = $3)
		  AND (account_type IS NULL OR account_type = $4)
		  AND (owner_type IS NULL OR owner_type = $5)
		ORDER BY 
			priority DESC,
			(source_currency IS NOT NULL):: int DESC,
			(target_currency IS NOT NULL)::int DESC,
			(account_type IS NOT NULL)::int DESC,
			(owner_type IS NOT NULL)::int DESC
		LIMIT 1
	`

	var rule domain.TransactionFeeRule
	err := r. db.QueryRow(ctx, query, transactionType, sourceCurrency, targetCurrency, accountType, ownerType).Scan(
		&rule.ID,
		&rule.RuleName,
		&rule.TransactionType,
		&rule.SourceCurrency,
		&rule. TargetCurrency,
		&rule.AccountType,
		&rule.OwnerType,
		&rule.FeeType,
		&rule.CalculationMethod,
		&rule.FeeValue,
		&rule. MinFee,
		&rule.MaxFee,
		&rule.Tiers,
		&rule.Tariffs, // ✅ NEW
		&rule.ValidFrom,
		&rule.ValidTo,
		&rule.IsActive,
		&rule.Priority,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find best match:  %w", err)
	}

	return &rule, nil
}
// FindAllMatches finds all matching fee rules (for debugging or multi-fee scenarios)
func (r *transactionFeeRuleRepo) FindAllMatches(ctx context.Context, transactionType domain.TransactionType, sourceCurrency, targetCurrency *string, accountType *domain.AccountType, ownerType *domain.OwnerType) ([]*domain.TransactionFeeRule, error) {
	query := `
		SELECT 
			id, rule_name, transaction_type, source_currency, target_currency,
			account_type, owner_type, fee_type, calculation_method,
			fee_value, min_fee, max_fee, tiers, tariffs,
			valid_from, valid_to, is_active, priority, created_at, updated_at
		FROM transaction_fee_rules
		WHERE is_active = true 
		  AND valid_to IS NULL
		  AND transaction_type = $1
		  AND (source_currency IS NULL OR source_currency = $2)
		  AND (target_currency IS NULL OR target_currency = $3)
		  AND (account_type IS NULL OR account_type = $4)
		  AND (owner_type IS NULL OR owner_type = $5)
		ORDER BY priority DESC
	`

	rows, err := r.db.Query(ctx, query, transactionType, sourceCurrency, targetCurrency, accountType, ownerType)
	if err != nil {
		return nil, fmt.Errorf("failed to find all matches: %w", err)
	}
	defer rows.Close()

	var rules []*domain.TransactionFeeRule
	for rows. Next() {
		var rule domain.TransactionFeeRule
		err := rows.Scan(
			&rule.ID,
			&rule.RuleName,
			&rule.TransactionType,
			&rule.SourceCurrency,
			&rule. TargetCurrency,
			&rule.AccountType,
			&rule.OwnerType,
			&rule.FeeType,
			&rule.CalculationMethod,
			&rule.FeeValue,
			&rule. MinFee,
			&rule.MaxFee,
			&rule.Tiers,
			&rule.Tariffs, // ✅ NEW
			&rule.ValidFrom,
			&rule.ValidTo,
			&rule.IsActive,
			&rule.Priority,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt. Errorf("failed to scan fee rule: %w", err)
		}
		rules = append(rules, &rule)
	}

	return rules, rows.Err()
}
// ===============================
// EXPIRATION
// ===============================

// ExpireRule marks a fee rule as expired by setting valid_to
func (r *transactionFeeRuleRepo) ExpireRule(ctx context.Context, tx pgx.Tx, id int64, validTo time.Time) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	query := `
		UPDATE transaction_fee_rules
		SET valid_to = $2, updated_at = $3
		WHERE id = $1 AND valid_to IS NULL
	`

	cmdTag, err := tx.Exec(ctx, query, id, validTo, time.Now())
	if err != nil {
		return fmt.Errorf("failed to expire fee rule: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}
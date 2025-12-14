// // repository/tariff_repository.go
 package repository

// import (
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"time"

// 	"accounting-service/internal/domain"

// 	"github.com/jackc/pgx/v5"
// 	"github.com/jackc/pgx/v5/pgxpool"
// 	xerrors "x/shared/utils/errors"
// )

// type TariffRepository interface {
// 	// Tariff CRUD
// 	CreateTariff(ctx context.Context, tx pgx. Tx, tariff *domain.TariffCreate) (*domain.Tariff, error)
// 	UpdateTariff(ctx context.Context, tx pgx.Tx, id int64, update *domain.TariffUpdate) (*domain.Tariff, error)
// 	GetTariffByID(ctx context.Context, id int64) (*domain.Tariff, error)
// 	GetTariffByCode(ctx context. Context, tariffCode string) (*domain.Tariff, error)
// 	ListTariffs(ctx context.Context, filter *domain.TariffFilter) ([]*domain.Tariff, error)
// 	DeleteTariff(ctx context.Context, tx pgx.Tx, id int64) error
	
// 	// Tariff Fee Rules
// 	AddFeeRuleToTariff(ctx context.Context, tx pgx. Tx, rule *domain.TariffFeeRuleCreate) (*domain.TariffFeeRule, error)
// 	RemoveFeeRuleFromTariff(ctx context.Context, tx pgx.Tx, tariffID, feeRuleID int64) error
// 	UpdateTariffFeeRule(ctx context.Context, tx pgx. Tx, id int64, overrideFeeValue, overrideMinFee, overrideMaxFee *float64) error
// 	ListFeeRulesForTariff(ctx context.Context, tariffID int64) ([]*domain.TariffFeeRule, error)
	
// 	// Get tariff with fee rules (includes overrides)
// 	GetTariffWithFeeRules(ctx context. Context, tariffID int64) (*domain.TariffWithFeeRules, error)
	
// 	// Get default tariff for a type
// 	GetDefaultTariff(ctx context.Context, tariffType string) (*domain.Tariff, error)
	
// 	// Transaction management
// 	BeginTx(ctx context.Context) (pgx.Tx, error)
// }

// type tariffRepo struct {
// 	db *pgxpool.Pool
// }

// func NewTariffRepository(db *pgxpool.Pool) TariffRepository {
// 	return &tariffRepo{db: db}
// }

// func (r *tariffRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
// 	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to begin transaction: %w", err)
// 	}
// 	return tx, nil
// }

// // ============================================================================
// // TARIFF CRUD
// // ============================================================================

// func (r *tariffRepo) CreateTariff(ctx context. Context, tx pgx.Tx, tariff *domain.TariffCreate) (*domain.Tariff, error) {
// 	if tx == nil {
// 		return nil, errors.New("transaction cannot be nil")
// 	}

// 	metadataJSON, _ := json.Marshal(tariff. Metadata)
// 	if tariff. Metadata == nil {
// 		metadataJSON = []byte("{}")
// 	}

// 	query := `
// 		INSERT INTO tariffs (
// 			tariff_code, tariff_name, description, tariff_type,
// 			is_default, is_active, priority, valid_from, valid_to,
// 			metadata, created_by, created_at, updated_at
// 		)
// 		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
// 		RETURNING id, created_at, updated_at
// 	`

// 	now := time.Now()
// 	if tariff.ValidFrom.IsZero() {
// 		tariff.ValidFrom = now
// 	}

// 	var result domain.Tariff
// 	result.TariffCode = tariff.TariffCode
// 	result.TariffName = tariff.TariffName
// 	result.Description = tariff.Description
// 	result.TariffType = tariff.TariffType
// 	result.IsDefault = tariff.IsDefault
// 	result.IsActive = tariff.IsActive
// 	result.Priority = tariff.Priority
// 	result.ValidFrom = tariff.ValidFrom
// 	result.ValidTo = tariff.ValidTo
// 	result.Metadata = tariff.Metadata
// 	result. CreatedBy = tariff.CreatedBy

// 	err := tx.QueryRow(ctx, query,
// 		tariff.TariffCode,
// 		tariff.TariffName,
// 		tariff.Description,
// 		tariff.TariffType,
// 		tariff. IsDefault,
// 		tariff. IsActive,
// 		tariff. Priority,
// 		tariff.ValidFrom,
// 		tariff.ValidTo,
// 		metadataJSON,
// 		tariff.CreatedBy,
// 		now,
// 		now,
// 	).Scan(&result.ID, &result.CreatedAt, &result.UpdatedAt)

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create tariff: %w", err)
// 	}

// 	return &result, nil
// }

// func (r *tariffRepo) UpdateTariff(ctx context.Context, tx pgx. Tx, id int64, update *domain.TariffUpdate) (*domain.Tariff, error) {
// 	if tx == nil {
// 		return nil, errors.New("transaction cannot be nil")
// 	}

// 	// Build dynamic update query
// 	query := `UPDATE tariffs SET updated_at = $1`
// 	args := []interface{}{time.Now()}
// 	argPos := 2

// 	if update.TariffName != nil {
// 		query += fmt.Sprintf(", tariff_name = $%d", argPos)
// 		args = append(args, *update. TariffName)
// 		argPos++
// 	}

// 	if update.Description != nil {
// 		query += fmt.Sprintf(", description = $%d", argPos)
// 		args = append(args, *update.Description)
// 		argPos++
// 	}

// 	if update.IsActive != nil {
// 		query += fmt.Sprintf(", is_active = $%d", argPos)
// 		args = append(args, *update.IsActive)
// 		argPos++
// 	}

// 	if update.Priority != nil {
// 		query += fmt.Sprintf(", priority = $%d", argPos)
// 		args = append(args, *update.Priority)
// 		argPos++
// 	}

// 	if update.ValidTo != nil {
// 		query += fmt. Sprintf(", valid_to = $%d", argPos)
// 		args = append(args, *update.ValidTo)
// 		argPos++
// 	}

// 	if update.Metadata != nil {
// 		metadataJSON, _ := json. Marshal(update.Metadata)
// 		query += fmt.Sprintf(", metadata = $%d", argPos)
// 		args = append(args, metadataJSON)
// 		argPos++
// 	}

// 	query += fmt.Sprintf(" WHERE id = $%d", argPos)
// 	args = append(args, id)

// 	cmdTag, err := tx.Exec(ctx, query, args...)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to update tariff: %w", err)
// 	}

// 	if cmdTag.RowsAffected() == 0 {
// 		return nil, xerrors.ErrNotFound
// 	}

// 	// Fetch updated tariff
// 	return r.GetTariffByID(ctx, id)
// }

// func (r *tariffRepo) GetTariffByID(ctx context.Context, id int64) (*domain.Tariff, error) {
// 	query := `
// 		SELECT 
// 			id, tariff_code, tariff_name, description, tariff_type,
// 			is_default, is_active, priority, valid_from, valid_to,
// 			metadata, created_at, updated_at, created_by
// 		FROM tariffs
// 		WHERE id = $1
// 	`

// 	return r.scanTariff(r.db. QueryRow(ctx, query, id))
// }

// func (r *tariffRepo) GetTariffByCode(ctx context.Context, tariffCode string) (*domain.Tariff, error) {
// 	query := `
// 		SELECT 
// 			id, tariff_code, tariff_name, description, tariff_type,
// 			is_default, is_active, priority, valid_from, valid_to,
// 			metadata, created_at, updated_at, created_by
// 		FROM tariffs
// 		WHERE tariff_code = $1
// 	`

// 	return r.scanTariff(r.db. QueryRow(ctx, query, tariffCode))
// }

// func (r *tariffRepo) ListTariffs(ctx context.Context, filter *domain.TariffFilter) ([]*domain.Tariff, error) {
// 	query := `
// 		SELECT 
// 			id, tariff_code, tariff_name, description, tariff_type,
// 			is_default, is_active, priority, valid_from, valid_to,
// 			metadata, created_at, updated_at, created_by
// 		FROM tariffs
// 		WHERE 1=1
// 	`

// 	args := []interface{}{}
// 	argPos := 1

// 	if filter.TariffType != nil {
// 		query += fmt.Sprintf(" AND tariff_type = $%d", argPos)
// 		args = append(args, *filter. TariffType)
// 		argPos++
// 	}

// 	if filter.IsDefault != nil {
// 		query += fmt.Sprintf(" AND is_default = $%d", argPos)
// 		args = append(args, *filter.IsDefault)
// 		argPos++
// 	}

// 	if filter.IsActive != nil {
// 		query += fmt.Sprintf(" AND is_active = $%d", argPos)
// 		args = append(args, *filter.IsActive)
// 		argPos++
// 	}

// 	if filter.ValidAt != nil {
// 		query += fmt.Sprintf(" AND valid_from <= $%d AND (valid_to IS NULL OR valid_to > $%d)", argPos, argPos)
// 		args = append(args, *filter.ValidAt)
// 		argPos++
// 	}

// 	query += " ORDER BY priority DESC, created_at DESC"

// 	if filter.Limit > 0 {
// 		query += fmt. Sprintf(" LIMIT $%d", argPos)
// 		args = append(args, filter.Limit)
// 		argPos++
// 	}

// 	if filter.Offset > 0 {
// 		query += fmt.Sprintf(" OFFSET $%d", argPos)
// 		args = append(args, filter. Offset)
// 	}

// 	rows, err := r.db. Query(ctx, query, args...)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to list tariffs:  %w", err)
// 	}
// 	defer rows.Close()

// 	var tariffs []*domain.Tariff
// 	for rows.Next() {
// 		tariff, err := r.scanTariffFromRows(rows)
// 		if err != nil {
// 			return nil, err
// 		}
// 		tariffs = append(tariffs, tariff)
// 	}

// 	return tariffs, rows.Err()
// }

// func (r *tariffRepo) DeleteTariff(ctx context.Context, tx pgx.Tx, id int64) error {
// 	if tx == nil {
// 		return errors.New("transaction cannot be nil")
// 	}

// 	cmdTag, err := tx.Exec(ctx, `DELETE FROM tariffs WHERE id = $1`, id)
// 	if err != nil {
// 		return fmt. Errorf("failed to delete tariff: %w", err)
// 	}

// 	if cmdTag. RowsAffected() == 0 {
// 		return xerrors.ErrNotFound
// 	}

// 	return nil
// }

// func (r *tariffRepo) GetDefaultTariff(ctx context. Context, tariffType string) (*domain.Tariff, error) {
// 	query := `
// 		SELECT 
// 			id, tariff_code, tariff_name, description, tariff_type,
// 			is_default, is_active, priority, valid_from, valid_to,
// 			metadata, created_at, updated_at, created_by
// 		FROM tariffs
// 		WHERE tariff_type = $1 AND is_default = true AND is_active = true
// 		LIMIT 1
// 	`

// 	return r.scanTariff(r.db.QueryRow(ctx, query, tariffType))
// }

// // ============================================================================
// // TARIFF FEE RULES
// // ============================================================================

// func (r *tariffRepo) AddFeeRuleToTariff(ctx context. Context, tx pgx.Tx, rule *domain.TariffFeeRuleCreate) (*domain.TariffFeeRule, error) {
// 	if tx == nil {
// 		return nil, errors.New("transaction cannot be nil")
// 	}

// 	query := `
// 		INSERT INTO tariff_fee_rules (
// 			tariff_id, fee_rule_id, override_fee_value, override_min_fee, override_max_fee,
// 			priority, is_active, created_at, updated_at
// 		)
// 		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
// 		RETURNING id, created_at, updated_at
// 	`

// 	now := time.Now()
// 	var result domain.TariffFeeRule
// 	result. TariffID = rule.TariffID
// 	result.FeeRuleID = rule.FeeRuleID
// 	result. OverrideFeeValue = rule.OverrideFeeValue
// 	result.OverrideMinFee = rule.OverrideMinFee
// 	result. OverrideMaxFee = rule.OverrideMaxFee
// 	result.Priority = rule.Priority
// 	result.IsActive = rule.IsActive

// 	err := tx.QueryRow(ctx, query,
// 		rule. TariffID,
// 		rule.FeeRuleID,
// 		rule.OverrideFeeValue,
// 		rule.OverrideMinFee,
// 		rule.OverrideMaxFee,
// 		rule.Priority,
// 		rule.IsActive,
// 		now,
// 		now,
// 	).Scan(&result.ID, &result.CreatedAt, &result.UpdatedAt)

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to add fee rule to tariff: %w", err)
// 	}

// 	return &result, nil
// }

// func (r *tariffRepo) RemoveFeeRuleFromTariff(ctx context.Context, tx pgx. Tx, tariffID, feeRuleID int64) error {
// 	if tx == nil {
// 		return errors.New("transaction cannot be nil")
// 	}

// 	cmdTag, err := tx.Exec(ctx, 
// 		`DELETE FROM tariff_fee_rules WHERE tariff_id = $1 AND fee_rule_id = $2`, 
// 		tariffID, feeRuleID)
// 	if err != nil {
// 		return fmt.Errorf("failed to remove fee rule from tariff: %w", err)
// 	}

// 	if cmdTag. RowsAffected() == 0 {
// 		return xerrors.ErrNotFound
// 	}

// 	return nil
// }

// func (r *tariffRepo) UpdateTariffFeeRule(ctx context.Context, tx pgx. Tx, id int64, overrideFeeValue, overrideMinFee, overrideMaxFee *float64) error {
// 	if tx == nil {
// 		return errors.New("transaction cannot be nil")
// 	}

// 	query := `
// 		UPDATE tariff_fee_rules
// 		SET override_fee_value = $2, override_min_fee = $3, override_max_fee = $4, updated_at = $5
// 		WHERE id = $1
// 	`

// 	cmdTag, err := tx.Exec(ctx, query, id, overrideFeeValue, overrideMinFee, overrideMaxFee, time.Now())
// 	if err != nil {
// 		return fmt.Errorf("failed to update tariff fee rule: %w", err)
// 	}

// 	if cmdTag.RowsAffected() == 0 {
// 		return xerrors.ErrNotFound
// 	}

// 	return nil
// }

// func (r *tariffRepo) ListFeeRulesForTariff(ctx context. Context, tariffID int64) ([]*domain.TariffFeeRule, error) {
// 	query := `
// 		SELECT 
// 			id, tariff_id, fee_rule_id, override_fee_value, override_min_fee, override_max_fee,
// 			priority, is_active, created_at, updated_at
// 		FROM tariff_fee_rules
// 		WHERE tariff_id = $1 AND is_active = true
// 		ORDER BY priority DESC
// 	`

// 	rows, err := r.db.Query(ctx, query, tariffID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to list fee rules for tariff: %w", err)
// 	}
// 	defer rows.Close()

// 	var rules []*domain. TariffFeeRule
// 	for rows.Next() {
// 		var rule domain.TariffFeeRule
// 		err := rows.Scan(
// 			&rule.ID,
// 			&rule.TariffID,
// 			&rule.FeeRuleID,
// 			&rule.OverrideFeeValue,
// 			&rule.OverrideMinFee,
// 			&rule.OverrideMaxFee,
// 			&rule.Priority,
// 			&rule.IsActive,
// 			&rule.CreatedAt,
// 			&rule.UpdatedAt,
// 		)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to scan tariff fee rule: %w", err)
// 		}
// 		rules = append(rules, &rule)
// 	}

// 	return rules, rows.Err()
// }

// func (r *tariffRepo) GetTariffWithFeeRules(ctx context. Context, tariffID int64) (*domain.TariffWithFeeRules, error) {
// 	// Get tariff
// 	tariff, err := r.GetTariffByID(ctx, tariffID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Get fee rules with details
// 	query := `
// 		SELECT 
// 			tfr.id, tfr.tariff_id, tfr.fee_rule_id, 
// 			tfr.override_fee_value, tfr.override_min_fee, tfr.override_max_fee,
// 			tfr.priority, tfr.is_active, tfr.created_at, tfr.updated_at,
// 			fr.id, fr.rule_name, fr.transaction_type, fr.source_currency, fr.target_currency,
// 			fr.account_type, fr.owner_type, fr.fee_type, fr.calculation_method,
// 			fr.fee_value, fr.min_fee, fr.max_fee, fr.tiers,
// 			fr.valid_from, fr.valid_to, fr.is_active, fr.priority, fr.created_at, fr.updated_at
// 		FROM tariff_fee_rules tfr
// 		JOIN transaction_fee_rules fr ON fr.id = tfr.fee_rule_id
// 		WHERE tfr.tariff_id = $1 AND tfr.is_active = true
// 		ORDER BY tfr.priority DESC
// 	`

// 	rows, err := r.db.Query(ctx, query, tariffID)
// 	if err != nil {
// 		return nil, fmt. Errorf("failed to get tariff with fee rules: %w", err)
// 	}
// 	defer rows.Close()

// 	var feeRules []*domain.TariffFeeRuleWithDetails
// 	for rows. Next() {
// 		var tfr domain.TariffFeeRule
// 		var fr domain.TransactionFeeRule

// 		err := rows.Scan(
// 			&tfr.ID, &tfr.TariffID, &tfr.FeeRuleID,
// 			&tfr.OverrideFeeValue, &tfr.OverrideMinFee, &tfr. OverrideMaxFee,
// 			&tfr.Priority, &tfr.IsActive, &tfr.CreatedAt, &tfr.UpdatedAt,
// 			&fr.ID, &fr.RuleName, &fr.TransactionType, &fr. SourceCurrency, &fr. TargetCurrency,
// 			&fr.AccountType, &fr.OwnerType, &fr.FeeType, &fr. CalculationMethod,
// 			&fr.FeeValue, &fr. MinFee, &fr.MaxFee, &fr.Tiers,
// 			&fr.ValidFrom, &fr.ValidTo, &fr.IsActive, &fr.Priority, &fr.CreatedAt, &fr.UpdatedAt,
// 		)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to scan tariff fee rule with details: %w", err)
// 		}

// 		// Calculate effective values
// 		var effectiveFeeValue *float64
// 		if tfr.OverrideFeeValue != nil {
// 			effectiveFeeValue = tfr.OverrideFeeValue
// 		} else {
// 			effectiveFeeValue = stringToFloat64Ptr(fr.FeeValue)
// 		}

// 		var effectiveMinFee *float64
// 		if tfr.OverrideMinFee != nil {
// 			effectiveMinFee = tfr.OverrideMinFee
// 		} else {
// 			effectiveMinFee = fr.MinFee
// 		}

// 		var effectiveMaxFee *float64
// 		if tfr.OverrideMaxFee != nil {
// 			effectiveMaxFee = tfr.OverrideMaxFee
// 		} else {
// 			effectiveMaxFee = fr.MaxFee
// 		}

// 		feeRules = append(feeRules, &domain.TariffFeeRuleWithDetails{
// 			TariffFeeRule:      &tfr,
// 			BaseFeeRule:       &fr,
// 			EffectiveFeeValue: *effectiveFeeValue,
// 			EffectiveMinFee:   effectiveMinFee,
// 			EffectiveMaxFee:   effectiveMaxFee,
// 		})
// 	}

// 	return &domain.TariffWithFeeRules{
// 		Tariff:   tariff,
// 		FeeRules: feeRules,
// 	}, rows.Err()
// }

// // ============================================================================
// // HELPER SCAN FUNCTIONS
// // ============================================================================

// func (r *tariffRepo) scanTariff(row pgx.Row) (*domain.Tariff, error) {
// 	var tariff domain.Tariff
// 	var metadataJSON []byte

// 	err := row.Scan(
// 		&tariff.ID,
// 		&tariff.TariffCode,
// 		&tariff.TariffName,
// 		&tariff.Description,
// 		&tariff.TariffType,
// 		&tariff. IsDefault,
// 		&tariff.IsActive,
// 		&tariff.Priority,
// 		&tariff.ValidFrom,
// 		&tariff.ValidTo,
// 		&metadataJSON,
// 		&tariff.CreatedAt,
// 		&tariff.UpdatedAt,
// 		&tariff. CreatedBy,
// 	)

// 	if err != nil {
// 		if errors.Is(err, pgx. ErrNoRows) {
// 			return nil, xerrors.ErrNotFound
// 		}
// 		return nil, fmt. Errorf("failed to scan tariff: %w", err)
// 	}

// 	if len(metadataJSON) > 0 {
// 		json.Unmarshal(metadataJSON, &tariff. Metadata)
// 	}

// 	return &tariff, nil
// }

// func (r *tariffRepo) scanTariffFromRows(rows pgx.Rows) (*domain.Tariff, error) {
// 	var tariff domain. Tariff
// 	var metadataJSON []byte

// 	err := rows.Scan(
// 		&tariff.ID,
// 		&tariff.TariffCode,
// 		&tariff.TariffName,
// 		&tariff.Description,
// 		&tariff.TariffType,
// 		&tariff.IsDefault,
// 		&tariff.IsActive,
// 		&tariff.Priority,
// 		&tariff.ValidFrom,
// 		&tariff. ValidTo,
// 		&metadataJSON,
// 		&tariff.CreatedAt,
// 		&tariff.UpdatedAt,
// 		&tariff.CreatedBy,
// 	)

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to scan tariff: %w", err)
// 	}

// 	if len(metadataJSON) > 0 {
// 		json.Unmarshal(metadataJSON, &tariff. Metadata)
// 	}

// 	return &tariff, nil
// }

// func stringToFloat64Ptr(s string) *float64 {
// 	if s == "" {
// 		return nil
// 	}
// 	f := 0.0
// 	// Add parsing logic here if needed
// 	return &f
// }
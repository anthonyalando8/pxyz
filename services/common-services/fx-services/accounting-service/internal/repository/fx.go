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

type CurrencyRepository interface {
	// Currency operations
	GetCurrency(ctx context.Context, code string) (*domain.Currency, error)
	GetCurrencies(ctx context.Context, codes []string) (map[string]*domain.Currency, error)
	ListCurrencies(ctx context.Context, activeOnly bool) ([]*domain.Currency, error)
	ListDemoCurrencies(ctx context.Context) ([]*domain.Currency, error)
	CreateCurrency(ctx context.Context, tx pgx.Tx, currency *domain.Currency) error
	CreateCurrencies(ctx context.Context, tx pgx.Tx, currencies []*domain.Currency) map[int]error
	UpdateCurrency(ctx context.Context, tx pgx.Tx, currency *domain.Currency) error
	
	// FX Rate operations
	GetFXRate(ctx context.Context, base, quote string, validAt time.Time) (*domain.FXRate, error)
	GetCurrentFXRate(ctx context.Context, base, quote string) (*domain.FXRate, error)
	GetFXRates(ctx context.Context, query *domain.FXRateQuery) ([]*domain.FXRate, error)
	ListFXRates(ctx context.Context, base string) ([]*domain.FXRate, error)
	CreateFXRate(ctx context.Context, tx pgx.Tx, rate *domain.FXRate) error
	CreateFXRates(ctx context.Context, tx pgx.Tx, rates []*domain.FXRate) map[int]error
	ExpireFXRate(ctx context.Context, tx pgx.Tx, id int64, validTo time.Time) error
	
	// Transaction management
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type currencyRepo struct {
	db *pgxpool.Pool
}

func NewCurrencyRepo(db *pgxpool.Pool) CurrencyRepository {
	return &currencyRepo{db: db}
}

// BeginTx starts a new transaction
func (r *currencyRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// ===============================
// CURRENCY OPERATIONS
// ===============================

// GetCurrency fetches a single currency by code
func (r *currencyRepo) GetCurrency(ctx context.Context, code string) (*domain.Currency, error) {
	query := `
		SELECT 
			code, name, symbol, decimals, is_fiat, is_active,
			demo_enabled, demo_initial_balance, min_amount, max_amount,
			created_at, updated_at
		FROM currencies
		WHERE code = $1
	`

	var c domain.Currency
	err := r.db.QueryRow(ctx, query, code).Scan(
		&c.Code,
		&c.Name,
		&c.Symbol,
		&c.Decimals,
		&c.IsFiat,
		&c.IsActive,
		&c.DemoEnabled,
		&c.DemoInitialBalance,
		&c.MinAmount,
		&c.MaxAmount,
		&c.CreatedAt,
		&c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get currency: %w", err)
	}

	return &c, nil
}

// GetCurrencies fetches multiple currencies by codes (bulk read)
func (r *currencyRepo) GetCurrencies(ctx context.Context, codes []string) (map[string]*domain.Currency, error) {
	if len(codes) == 0 {
		return make(map[string]*domain.Currency), nil
	}

	query := `
		SELECT 
			code, name, symbol, decimals, is_fiat, is_active,
			demo_enabled, demo_initial_balance, min_amount, max_amount,
			created_at, updated_at
		FROM currencies
		WHERE code = ANY($1)
	`

	rows, err := r.db.Query(ctx, query, codes)
	if err != nil {
		return nil, fmt.Errorf("failed to query currencies: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*domain.Currency, len(codes))
	for rows.Next() {
		var c domain.Currency
		err := rows.Scan(
			&c.Code,
			&c.Name,
			&c.Symbol,
			&c.Decimals,
			&c.IsFiat,
			&c.IsActive,
			&c.DemoEnabled,
			&c.DemoInitialBalance,
			&c.MinAmount,
			&c.MaxAmount,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan currency: %w", err)
		}
		result[c.Code] = &c
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating currency rows: %w", err)
	}

	return result, nil
}

// ListCurrencies fetches all currencies
func (r *currencyRepo) ListCurrencies(ctx context.Context, activeOnly bool) ([]*domain.Currency, error) {
	query := `
		SELECT 
			code, name, symbol, decimals, is_fiat, is_active,
			demo_enabled, demo_initial_balance, min_amount, max_amount,
			created_at, updated_at
		FROM currencies
	`
	
	if activeOnly {
		query += " WHERE is_active = true"
	}
	
	query += " ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list currencies: %w", err)
	}
	defer rows.Close()

	var currencies []*domain.Currency
	for rows.Next() {
		var c domain.Currency
		err := rows.Scan(
			&c.Code,
			&c.Name,
			&c.Symbol,
			&c.Decimals,
			&c.IsFiat,
			&c.IsActive,
			&c.DemoEnabled,
			&c.DemoInitialBalance,
			&c.MinAmount,
			&c.MaxAmount,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan currency: %w", err)
		}
		currencies = append(currencies, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating currency rows: %w", err)
	}

	return currencies, nil
}

// ListDemoCurrencies fetches all currencies that support demo accounts
func (r *currencyRepo) ListDemoCurrencies(ctx context.Context) ([]*domain.Currency, error) {
	query := `
		SELECT 
			code, name, symbol, decimals, is_fiat, is_active,
			demo_enabled, demo_initial_balance, min_amount, max_amount,
			created_at, updated_at
		FROM currencies
		WHERE demo_enabled = true AND is_active = true
		ORDER BY name ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list demo currencies: %w", err)
	}
	defer rows.Close()

	var currencies []*domain.Currency
	for rows.Next() {
		var c domain.Currency
		err := rows.Scan(
			&c.Code,
			&c.Name,
			&c.Symbol,
			&c.Decimals,
			&c.IsFiat,
			&c.IsActive,
			&c.DemoEnabled,
			&c.DemoInitialBalance,
			&c.MinAmount,
			&c.MaxAmount,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan demo currency: %w", err)
		}
		currencies = append(currencies, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating demo currency rows: %w", err)
	}

	return currencies, nil
}

// CreateCurrency creates a single currency
func (r *currencyRepo) CreateCurrency(ctx context.Context, tx pgx.Tx, currency *domain.Currency) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	if !currency.IsValid() {
		return errors.New("invalid currency code length")
	}

	query := `
		INSERT INTO currencies (
			code, name, symbol, decimals, is_fiat, is_active,
			demo_enabled, demo_initial_balance, min_amount, max_amount,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (code) DO UPDATE 
		SET 
			name = EXCLUDED.name,
			symbol = EXCLUDED.symbol,
			decimals = EXCLUDED.decimals,
			is_fiat = EXCLUDED.is_fiat,
			is_active = EXCLUDED.is_active,
			demo_enabled = EXCLUDED.demo_enabled,
			demo_initial_balance = EXCLUDED.demo_initial_balance,
			min_amount = EXCLUDED.min_amount,
			max_amount = EXCLUDED.max_amount,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	_, err := tx.Exec(ctx, query,
		currency.Code,
		currency.Name,
		currency.Symbol,
		currency.Decimals,
		currency.IsFiat,
		currency.IsActive,
		currency.DemoEnabled,
		currency.DemoInitialBalance,
		currency.MinAmount,
		currency.MaxAmount,
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to create currency: %w", err)
	}

	return nil
}

// CreateCurrencies creates multiple currencies (bulk insert with error tracking)
func (r *currencyRepo) CreateCurrencies(ctx context.Context, tx pgx.Tx, currencies []*domain.Currency) map[int]error {
	if tx == nil {
		return map[int]error{0: errors.New("transaction cannot be nil")}
	}

	errs := make(map[int]error)
	now := time.Now()

	// Use batch for better performance
	batch := &pgx.Batch{}
	query := `
		INSERT INTO currencies (
			code, name, symbol, decimals, is_fiat, is_active,
			demo_enabled, demo_initial_balance, min_amount, max_amount,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (code) DO UPDATE 
		SET 
			name = EXCLUDED.name,
			symbol = EXCLUDED.symbol,
			decimals = EXCLUDED.decimals,
			is_fiat = EXCLUDED.is_fiat,
			is_active = EXCLUDED.is_active,
			demo_enabled = EXCLUDED.demo_enabled,
			demo_initial_balance = EXCLUDED.demo_initial_balance,
			min_amount = EXCLUDED.min_amount,
			max_amount = EXCLUDED.max_amount,
			updated_at = EXCLUDED.updated_at
	`

	for i, c := range currencies {
		if !c.IsValid() {
			errs[i] = errors.New("invalid currency code length")
			continue
		}

		batch.Queue(query,
			c.Code,
			c.Name,
			c.Symbol,
			c.Decimals,
			c.IsFiat,
			c.IsActive,
			c.DemoEnabled,
			c.DemoInitialBalance,
			c.MinAmount,
			c.MaxAmount,
			now,
			now,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	batchIndex := 0
	for i := range currencies {
		if _, hasError := errs[i]; hasError {
			continue // Skip currencies that already have errors
		}

		_, err := br.Exec()
		if err != nil {
			errs[i] = fmt.Errorf("failed to create currency at index %d: %w", i, err)
		}
		batchIndex++
	}

	return errs
}

// UpdateCurrency updates an existing currency
func (r *currencyRepo) UpdateCurrency(ctx context.Context, tx pgx.Tx, currency *domain.Currency) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	query := `
		UPDATE currencies
		SET 
			name = $2,
			symbol = $3,
			is_active = $4,
			demo_enabled = $5,
			demo_initial_balance = $6,
			min_amount = $7,
			max_amount = $8,
			updated_at = $9
		WHERE code = $1
	`

	cmdTag, err := tx.Exec(ctx, query,
		currency.Code,
		currency.Name,
		currency.Symbol,
		currency.IsActive,
		currency.DemoEnabled,
		currency.DemoInitialBalance,
		currency.MinAmount,
		currency.MaxAmount,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update currency: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// ===============================
// FX RATE OPERATIONS
// ===============================

// GetFXRate fetches FX rate valid at a specific time
func (r *currencyRepo) GetFXRate(ctx context.Context, base, quote string, validAt time.Time) (*domain.FXRate, error) {
	query := `
		SELECT 
			id, base_currency, quote_currency, rate, bid_rate, ask_rate,
			spread, source, valid_from, valid_to, created_at
		FROM fx_rates
		WHERE base_currency = $1 
		  AND quote_currency = $2 
		  AND valid_from <= $3
		  AND (valid_to IS NULL OR valid_to > $3)
		ORDER BY valid_from DESC
		LIMIT 1
	`

	var fx domain.FXRate
	err := r.db.QueryRow(ctx, query, base, quote, validAt).Scan(
		&fx.ID,
		&fx.BaseCurrency,
		&fx.QuoteCurrency,
		&fx.Rate,
		&fx.BidRate,
		&fx.AskRate,
		&fx.Spread,
		&fx.Source,
		&fx.ValidFrom,
		&fx.ValidTo,
		&fx.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get FX rate: %w", err)
	}

	return &fx, nil
}

// GetCurrentFXRate fetches the currently active FX rate (optimized index usage)
func (r *currencyRepo) GetCurrentFXRate(ctx context.Context, base, quote string) (*domain.FXRate, error) {
	// Uses idx_fx_rates_current for fast lookup
	query := `
		SELECT 
			id, base_currency, quote_currency, rate, bid_rate, ask_rate,
			spread, source, valid_from, valid_to, created_at
		FROM fx_rates
		WHERE base_currency = $1 
		  AND quote_currency = $2 
		  AND valid_to IS NULL
		ORDER BY valid_from DESC
		LIMIT 1
	`

	var fx domain.FXRate
	err := r.db.QueryRow(ctx, query, base, quote).Scan(
		&fx.ID,
		&fx.BaseCurrency,
		&fx.QuoteCurrency,
		&fx.Rate,
		&fx.BidRate,
		&fx.AskRate,
		&fx.Spread,
		&fx.Source,
		&fx.ValidFrom,
		&fx.ValidTo,
		&fx.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get current FX rate: %w", err)
	}

	return &fx, nil
}

// GetFXRates fetches FX rates based on query parameters
func (r *currencyRepo) GetFXRates(ctx context.Context, query *domain.FXRateQuery) ([]*domain.FXRate, error) {
	sql := `
		SELECT 
			id, base_currency, quote_currency, rate, bid_rate, ask_rate,
			spread, source, valid_from, valid_to, created_at
		FROM fx_rates
		WHERE base_currency = $1 
		  AND quote_currency = $2
	`

	args := []interface{}{query.BaseCurrency, query.QuoteCurrency}

	if !query.ValidAt.IsZero() {
		sql += " AND valid_from <= $3"
		args = append(args, query.ValidAt)

		if !query.IncludeExpired {
			sql += " AND (valid_to IS NULL OR valid_to > $3)"
		}
	} else if !query.IncludeExpired {
		sql += " AND valid_to IS NULL"
	}

	sql += " ORDER BY valid_from DESC"

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query FX rates: %w", err)
	}
	defer rows.Close()

	var rates []*domain.FXRate
	for rows.Next() {
		var fx domain.FXRate
		err := rows.Scan(
			&fx.ID,
			&fx.BaseCurrency,
			&fx.QuoteCurrency,
			&fx.Rate,
			&fx.BidRate,
			&fx.AskRate,
			&fx.Spread,
			&fx.Source,
			&fx.ValidFrom,
			&fx.ValidTo,
			&fx.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan FX rate: %w", err)
		}
		rates = append(rates, &fx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating FX rate rows: %w", err)
	}

	return rates, nil
}

// ListFXRates fetches all current FX rates for a base currency
func (r *currencyRepo) ListFXRates(ctx context.Context, base string) ([]*domain.FXRate, error) {
	query := `
		SELECT 
			id, base_currency, quote_currency, rate, bid_rate, ask_rate,
			spread, source, valid_from, valid_to, created_at
		FROM fx_rates
		WHERE base_currency = $1 AND valid_to IS NULL
		ORDER BY valid_from DESC
	`

	rows, err := r.db.Query(ctx, query, base)
	if err != nil {
		return nil, fmt.Errorf("failed to list FX rates: %w", err)
	}
	defer rows.Close()

	var rates []*domain.FXRate
	for rows.Next() {
		var fx domain.FXRate
		err := rows.Scan(
			&fx.ID,
			&fx.BaseCurrency,
			&fx.QuoteCurrency,
			&fx.Rate,
			&fx.BidRate,
			&fx.AskRate,
			&fx.Spread,
			&fx.Source,
			&fx.ValidFrom,
			&fx.ValidTo,
			&fx.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan FX rate: %w", err)
		}
		rates = append(rates, &fx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating FX rate rows: %w", err)
	}

	return rates, nil
}

// CreateFXRate creates a single FX rate
func (r *currencyRepo) CreateFXRate(ctx context.Context, tx pgx.Tx, rate *domain.FXRate) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	// Validate currency code lengths
	if len(rate.BaseCurrency) > 8 || len(rate.QuoteCurrency) > 8 {
		return errors.New("currency codes must be 8 characters or less")
	}

	query := `
		INSERT INTO fx_rates (
			base_currency, quote_currency, rate, bid_rate, ask_rate,
			spread, source, valid_from, valid_to, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	now := time.Now()
	if rate.ValidFrom.IsZero() {
		rate.ValidFrom = now
	}

	err := tx.QueryRow(ctx, query,
		rate.BaseCurrency,
		rate.QuoteCurrency,
		rate.Rate,
		rate.BidRate,
		rate.AskRate,
		rate.Spread,
		rate.Source,
		rate.ValidFrom,
		rate.ValidTo,
		now,
	).Scan(&rate.ID)

	if err != nil {
		return fmt.Errorf("failed to create FX rate: %w", err)
	}

	return nil
}

// CreateFXRates creates multiple FX rates (bulk insert with error tracking)
func (r *currencyRepo) CreateFXRates(ctx context.Context, tx pgx.Tx, rates []*domain.FXRate) map[int]error {
	if tx == nil {
		return map[int]error{0: errors.New("transaction cannot be nil")}
	}

	errs := make(map[int]error)
	now := time.Now()

	// Use batch for better performance
	batch := &pgx.Batch{}
	query := `
		INSERT INTO fx_rates (
			base_currency, quote_currency, rate, bid_rate, ask_rate,
			spread, source, valid_from, valid_to, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	for i, fx := range rates {
		// Validate currency code lengths
		if len(fx.BaseCurrency) > 8 || len(fx.QuoteCurrency) > 8 {
			errs[i] = errors.New("currency codes must be 8 characters or less")
			continue
		}

		if fx.ValidFrom.IsZero() {
			fx.ValidFrom = now
		}

		batch.Queue(query,
			fx.BaseCurrency,
			fx.QuoteCurrency,
			fx.Rate,
			fx.BidRate,
			fx.AskRate,
			fx.Spread,
			fx.Source,
			fx.ValidFrom,
			fx.ValidTo,
			now,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	batchIndex := 0
	for i := range rates {
		if _, hasError := errs[i]; hasError {
			continue // Skip rates that already have errors
		}

		_, err := br.Exec()
		if err != nil {
			errs[i] = fmt.Errorf("failed to create FX rate at index %d: %w", i, err)
		}
		batchIndex++
	}

	return errs
}

// ExpireFXRate marks an FX rate as expired by setting valid_to
func (r *currencyRepo) ExpireFXRate(ctx context.Context, tx pgx.Tx, id int64, validTo time.Time) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	query := `
		UPDATE fx_rates
		SET valid_to = $2
		WHERE id = $1 AND valid_to IS NULL
	`

	cmdTag, err := tx.Exec(ctx, query, id, validTo)
	if err != nil {
		return fmt.Errorf("failed to expire FX rate: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}
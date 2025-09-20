package repository

import (
	"context"
	"time"

	"accounting-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CurrencyRepository interface {
	// Currency operations
	GetCurrency(ctx context.Context, code string) (*domain.Currency, error)
	ListCurrencies(ctx context.Context) ([]*domain.Currency, error)
	CreateCurrencies(ctx context.Context, currencies []*domain.Currency, tx pgx.Tx) map[int]error

	// FX Rate operations
	GetFXRate(ctx context.Context, base, quote string, asOf time.Time) (*domain.FXRate, error)
	ListFXRates(ctx context.Context, base string) ([]*domain.FXRate, error)
	CreateFXRates(ctx context.Context, rates []*domain.FXRate, tx pgx.Tx) map[int]error

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
		return nil, err
	}
	return tx, nil
}


// --- Currency ---
// --- Currencies ---
func (r *currencyRepo) CreateCurrencies(ctx context.Context, currencies []*domain.Currency, tx pgx.Tx) map[int]error {
	if tx == nil {
		return map[int]error{0: pgx.ErrTxClosed}
	}

	errs := make(map[int]error)
	now := time.Now()
	for i, c := range currencies {
		_, err := tx.Exec(ctx, `
			INSERT INTO currencies (code, name, decimals, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (code) DO UPDATE 
			SET name = EXCLUDED.name,
			    decimals = EXCLUDED.decimals,
			    updated_at = EXCLUDED.updated_at
		`, c.Code, c.Name, c.Decimals, now, now)
		if err != nil {
			errs[i] = err
		}
	}
	return errs
}

// --- FX Rates ---
func (r *currencyRepo) CreateFXRates(ctx context.Context, rates []*domain.FXRate, tx pgx.Tx) map[int]error {
	if tx == nil {
		return map[int]error{0: pgx.ErrTxClosed}
	}

	errs := make(map[int]error)
	now := time.Now()
	for i, fx := range rates {
		_, err := tx.Exec(ctx, `
			INSERT INTO fx_rates (base_currency, quote_currency, rate, as_of, created_at)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (base_currency, quote_currency, as_of) DO UPDATE 
			SET rate = EXCLUDED.rate,
			    created_at = EXCLUDED.created_at
		`, fx.BaseCurrency, fx.QuoteCurrency, fx.Rate, fx.AsOf, now)
		if err != nil {
			errs[i] = err
		}
	}
	return errs
}



func (r *currencyRepo) GetCurrency(ctx context.Context, code string) (*domain.Currency, error) {
	row := r.db.QueryRow(ctx, `
		SELECT code, name, decimals, created_at, updated_at
		FROM currencies
		WHERE code=$1
	`, code)

	var c domain.Currency
	if err := row.Scan(&c.Code, &c.Name, &c.Decimals, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *currencyRepo) ListCurrencies(ctx context.Context) ([]*domain.Currency, error) {
	rows, err := r.db.Query(ctx, `
		SELECT code, name, decimals, created_at, updated_at
		FROM currencies
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var currencies []*domain.Currency
	for rows.Next() {
		var c domain.Currency
		if err := rows.Scan(&c.Code, &c.Name, &c.Decimals, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		currencies = append(currencies, &c)
	}
	return currencies, nil
}
func (r *currencyRepo) GetFXRate(ctx context.Context, base, quote string, asOf time.Time) (*domain.FXRate, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, base_currency, quote_currency, rate, as_of, created_at
		FROM fx_rates
		WHERE base_currency=$1 AND quote_currency=$2 AND as_of <= $3
		ORDER BY as_of DESC
		LIMIT 1
	`, base, quote, asOf)

	var fx domain.FXRate
	if err := row.Scan(&fx.ID, &fx.BaseCurrency, &fx.QuoteCurrency, &fx.Rate, &fx.AsOf, &fx.CreatedAt); err != nil {
		return nil, err
	}
	return &fx, nil
}

func (r *currencyRepo) ListFXRates(ctx context.Context, base string) ([]*domain.FXRate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, base_currency, quote_currency, rate, as_of, created_at
		FROM fx_rates
		WHERE base_currency=$1
		ORDER BY as_of DESC
	`, base)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []*domain.FXRate
	for rows.Next() {
		var fx domain.FXRate
		if err := rows.Scan(&fx.ID, &fx.BaseCurrency, &fx.QuoteCurrency, &fx.Rate, &fx.AsOf, &fx.CreatedAt); err != nil {
			return nil, err
		}
		rates = append(rates, &fx)
	}
	return rates, nil
}
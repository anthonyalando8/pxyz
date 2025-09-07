package repository

import (
	"context"
	"core-service/internal/domain"
	"x/shared/utils/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CountryRepository interface {
	Upsert(ctx context.Context, c *domain.Country) error
	GetAll(ctx context.Context) ([]*domain.Country, error)
	GetByISO2(ctx context.Context, iso2 string) (*domain.Country, error)
}

type countryRepo struct {
	db *pgxpool.Pool
}

func NewCountryRepo(db *pgxpool.Pool) CountryRepository {
	return &countryRepo{db: db}
}

func (r *countryRepo) Upsert(ctx context.Context, c *domain.Country) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO countries (
			iso2, iso3, name, phone_code, currency_code, currency_name, region, subregion, flag_url
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (iso2) DO UPDATE SET
			iso3=EXCLUDED.iso3,
			name=EXCLUDED.name,
			phone_code=EXCLUDED.phone_code,
			currency_code=EXCLUDED.currency_code,
			currency_name=EXCLUDED.currency_name,
			region=EXCLUDED.region,
			subregion=EXCLUDED.subregion,
			flag_url=EXCLUDED.flag_url,
			updated_at=now()
	`, c.ISO2, c.ISO3, c.Name, c.PhoneCode, c.CurrencyCode, c.CurrencyName, c.Region, c.Subregion, c.FlagURL)

	return err
}

func (r *countryRepo) GetAll(ctx context.Context) ([]*domain.Country, error) {
	rows, err := r.db.Query(ctx, `
		SELECT iso2, iso3, name, phone_code, currency_code, currency_name, region, subregion, flag_url
		FROM countries
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var countries []*domain.Country
	for rows.Next() {
		c := &domain.Country{}
		if err := rows.Scan(
			&c.ISO2,
			&c.ISO3,
			&c.Name,
			&c.PhoneCode,
			&c.CurrencyCode,
			&c.CurrencyName,
			&c.Region,
			&c.Subregion,
			&c.FlagURL,
		); err != nil {
			return nil, err
		}
		countries = append(countries, c)
	}

	return countries, rows.Err()
}
func (r *countryRepo) GetByISO2(ctx context.Context, iso2 string) (*domain.Country, error) {
	row := r.db.QueryRow(ctx, `
        SELECT id, iso2, iso3, name, phone_code, currency_code, currency_name, region, subregion, flag_url
        FROM countries
        WHERE iso2 = $1
    `, iso2)

	c := &domain.Country{}
	err := row.Scan(&c.ID, &c.ISO2, &c.ISO3, &c.Name, &c.PhoneCode, &c.CurrencyCode, &c.CurrencyName, &c.Region, &c.Subregion, &c.FlagURL)
	if err != nil {
		if err.Error() == "no rows in result set" { // pgx returns this string
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

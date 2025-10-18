package usecase

import (
	"context"
	"core-service/internal/domain"
	"core-service/internal/repository"
	"x/shared/utils/id"
	"time"
	"fmt"
)

type CountryUsecase struct {
	countryRepo repository.CountryRepository
	sf          *id.Snowflake
}

// NewCountryUsecase initializes a new CountryUsecase
func NewCountryUsecase(countryRepo repository.CountryRepository, sf *id.Snowflake) *CountryUsecase {
	return &CountryUsecase{
		countryRepo: countryRepo,
		sf:          sf,
	}
}

// RefreshCountries fetches from external API and updates DB
func (u *CountryUsecase) RefreshCountries(ctx context.Context) error {
	// TODO: replace with actual API call to public country endpoint
	countries, err := fetchCountriesFromAPI()
	if err != nil {
		return fmt.Errorf("failed to fetch countries: %w", err)
	}

	for _, c := range countries {
		// if c.ID == "" {
		// 	c.ID = u.sf.Generate()
		// }
		c.UpdatedAt = time.Now()
		if err := u.countryRepo.Upsert(ctx, c); err != nil {
			return fmt.Errorf("failed to upsert country %s: %w", c.Name, err)
		}
	}
	return nil
}

// GetAllCountries returns all countries from the DB
func (u *CountryUsecase) GetAllCountries(ctx context.Context) ([]*domain.Country, error) {
	return u.countryRepo.GetAll(ctx)
}

// GetCountryByISO2 returns a single country by ISO2 code
func (u *CountryUsecase) GetCountryByISO2(ctx context.Context, iso2 string) (*domain.Country, error) {
	return u.countryRepo.GetByISO2(ctx, iso2)
}

// --- mock fetcher until we wire real API ---
func fetchCountriesFromAPI() ([]*domain.Country, error) {
	// TODO: Replace with HTTP client fetching from public API
	return []*domain.Country{
		{
			ISO2:         "KE",
			ISO3:         "KEN",
			Name:         "Kenya",
			PhoneCode:    "+254",
			CurrencyCode: "KES",
			CurrencyName: "Kenyan Shilling",
			Region:       "Africa",
			Subregion:    "Eastern Africa",
			FlagURL:      "https://flagcdn.com/w320/ke.png",
		},
	}, nil
}

package service

import (
	"context"
	//"encoding/json"
	//"fmt"
	"net/http"
	"time"
	//"log"

	//"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	"github.com/jackc/pgx/v5"
)

type FXService struct {
	repo repository.CurrencyRepository
	client *http.Client
}

// NewFXService creates a new FXService instance
func NewFXService(repo repository.CurrencyRepository) *FXService {
	return &FXService{
		repo: repo,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchCommonCurrencies fetches common currencies from a public URL
func (s *FXService) FetchCommonCurrencies(ctx context.Context, tx pgx.Tx) map[int]error {
	// if tx == nil {
	// 	return map[int]error{0: fmt.Errorf("transaction cannot be nil")}
	// }

	// // Use static defaults instead of fetching online
	// currencies := domain.DefaultCurrencies()
	// log.Printf("➡️ Seeding %d default currencies (USD, BTC, USDT)", len(currencies))

	// errMap := s.repo.CreateCurrencies(ctx, currencies, tx)
	// if len(errMap) > 0 {
	// 	for i, e := range errMap {
	// 		log.Printf("⚠️ currency insert error #%d: %v", i, e)
	// 	}
	// } else {
	// 	log.Println("✅ all default currencies inserted successfully")
	// }

	// return errMap
	return map[int]error{0: nil}
}


// FetchFXRates fetches conversion rates for a base currency and saves to DB
func (s *FXService) FetchFXRates(ctx context.Context, base string, tx pgx.Tx) map[int]error {
	// if tx == nil {
	// 	return map[int]error{0: fmt.Errorf("transaction cannot be nil")}
	// }

	// url := fmt.Sprintf("https://openexchangerates.org/api/latest.json?base=%s", base)
	// log.Printf("➡️ Fetching FX rates for base=%s from %s", base, url)

	// resp, err := s.client.Get(url)
	// if err != nil {
	// 	log.Printf("❌ failed HTTP request for FX rates: %v", err)
	// 	return map[int]error{0: fmt.Errorf("failed to fetch FX rates: %w", err)}
	// }
	// defer resp.Body.Close()
	// log.Printf("✅ received response for FX rates, status=%s", resp.Status)

	// var data struct {
	// 	Rates     map[string]float64 `json:"rates"`
	// 	Timestamp int64              `json:"timestamp"`
	// }
	// if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
	// 	log.Printf("❌ failed to decode FX rates JSON: %v", err)
	// 	return map[int]error{0: fmt.Errorf("failed to decode FX rates: %w", err)}
	// }
	// log.Printf("✅ decoded %d FX rates (timestamp=%d)", len(data.Rates), data.Timestamp)

	// rates := []*domain.FXRate{}
	// asOf := time.Unix(data.Timestamp, 0)
	// for quote, rate := range data.Rates {
	// 	rates = append(rates, &domain.FXRate{
	// 		BaseCurrency:  base,
	// 		QuoteCurrency: quote,
	// 		Rate:          rate,
	// 		AsOf:          asOf,
	// 	})
	// }
	// log.Printf("➡️ inserting %d FX rates into DB (as_of=%s)", len(rates), asOf)

	// errMap := s.repo.CreateFXRates(ctx, rates, tx)
	// if len(errMap) > 0 {
	// 	for i, e := range errMap {
	// 		log.Printf("⚠️ FX rate insert error #%d: %v", i, e)
	// 	}
	// } else {
	// 	log.Println("✅ all FX rates inserted successfully")
	// }

	// return errMap
	return map[int]error{0: nil}
}


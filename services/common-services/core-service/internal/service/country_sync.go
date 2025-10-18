package service

import (
	"context"
	"core-service/internal/domain"
	"core-service/internal/repository"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const restCountriesAPI = "https://restcountries.com/v3.1/all?fields=cca2,cca3,name,region,subregion,idd,currencies,flags"

type CountrySync struct {
    repo repository.CountryRepository
}

func NewCountrySync(repo repository.CountryRepository) *CountrySync {
    return &CountrySync{repo: repo}
}

func (s *CountrySync) Sync(ctx context.Context) error {
	log.Println("[CountrySync] Fetching data from", restCountriesAPI)

	resp, err := http.Get(restCountriesAPI)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Debug: dump raw response (up to 2KB)
	body, _ := io.ReadAll(resp.Body)
	snippet := string(body)
	if len(snippet) > 2000 {
		snippet = snippet[:2000] + "...[truncated]"
		_ = snippet
	}
	//log.Printf("[CountrySync] API response snippet: %s", snippet)

	// Decode response into array or object
	var rawArr []map[string]interface{}
	if err := json.Unmarshal(body, &rawArr); err != nil {
		var rawObj map[string]interface{}
		if err2 := json.Unmarshal(body, &rawObj); err2 != nil {
			return fmt.Errorf("failed to decode API response: %w", err)
		}
		rawArr = append(rawArr, rawObj)
	}

	count := 0
	for _, item := range rawArr {
		country := domain.Country{}

		if v, ok := item["cca2"].(string); ok {
			country.ISO2 = v
		}
		if v, ok := item["cca3"].(string); ok {
			country.ISO3 = v
		}
		if name, ok := item["name"].(map[string]interface{}); ok {
			if common, ok := name["common"].(string); ok {
				country.Name = common
			}
		}
		if v, ok := item["region"].(string); ok {
			country.Region = v
		}
		if v, ok := item["subregion"].(string); ok {
			country.Subregion = v
		}

		// Phone code
		if idd, ok := item["idd"].(map[string]interface{}); ok {
			if root, ok := idd["root"].(string); ok && root != "" {
				if suffixes, ok := idd["suffixes"].([]interface{}); ok && len(suffixes) > 0 {
					if sfx, ok := suffixes[0].(string); ok {
						country.PhoneCode = root + sfx
					}
				}
			}
		}

		// Currency (first entry only)
		if currencies, ok := item["currencies"].(map[string]interface{}); ok {
			for code, val := range currencies {
				if cur, ok := val.(map[string]interface{}); ok {
					if cname, ok := cur["name"].(string); ok {
						country.CurrencyCode = code
						country.CurrencyName = cname
					}
				}
				break
			}
		}

		// Flag
		if flags, ok := item["flags"].(map[string]interface{}); ok {
			if png, ok := flags["png"].(string); ok {
				country.FlagURL = png
			}
		}

		// Skip if no ISO2 or Name (invalid country entry)
		if country.ISO2 == "" || country.Name == "" {
			log.Printf("[CountrySync] Skipping invalid entry: %+v", item)
			continue
		}

		if err := s.repo.Upsert(ctx, &country); err != nil {
			log.Printf("[CountrySync] ❌ failed to upsert country %s (%s): %v",
				country.Name, country.ISO2, err)
		} else {
			count++
			//log.Printf("[CountrySync] ✅ upserted country: %s (%s)", country.Name, country.ISO2)
		}
	}

	log.Printf("[CountrySync] Sync complete | Total countries upserted: %d", count)
	return nil
}


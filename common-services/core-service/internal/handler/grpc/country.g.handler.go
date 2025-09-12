package hgrpc

import (
	"context"
	"log"

	"core-service/internal/usecase"
	"core-service/internal/domain"
	corepb "x/shared/genproto/corepb"
)

type CountryGRPCHandler struct {
	corepb.UnimplementedCoreServiceServer
	countryUC *usecase.CountryUsecase
}

func NewCountryGRPCHandler(uc *usecase.CountryUsecase) *CountryGRPCHandler {
	return &CountryGRPCHandler{countryUC: uc}
}

// GetAllCountries returns all countries
func (h *CountryGRPCHandler) GetAllCountries(ctx context.Context, req *corepb.GetAllCountriesRequest) (*corepb.GetAllCountriesResponse, error) {
	countries, err := h.countryUC.GetAllCountries(ctx)
	if err != nil {
		log.Printf("[gRPC] GetAllCountries failed: %v", err)
		return nil, err
	}

	var result []*corepb.Country
	for _, c := range countries {
		result = append(result, toProtoCountry(c))
	}

	return &corepb.GetAllCountriesResponse{
		Countries: result,
	}, nil
}

// GetCountry returns a single country by ISO2
func (h *CountryGRPCHandler) GetCountry(ctx context.Context, req *corepb.GetCountryRequest) (*corepb.GetCountryResponse, error) {
	country, err := h.countryUC.GetCountryByISO2(ctx, req.GetIso2())
	if err != nil {
		log.Printf("[gRPC] GetCountry(%s) failed: %v", req.GetIso2(), err)
		return nil, err
	}

	return &corepb.GetCountryResponse{
		Country: toProtoCountry(country),
	}, nil
}

// --- Helpers ---
func toProtoCountry(c *domain.Country) *corepb.Country {
	if c == nil {
		return nil
	}
	return &corepb.Country{
		Id:           c.ID,
		Iso2:         c.ISO2,
		Iso3:         c.ISO3,
		Name:         c.Name,
		PhoneCode:    c.PhoneCode,
		CurrencyCode: c.CurrencyCode,
		CurrencyName: c.CurrencyName,
		Region:       c.Region,
		Subregion:    c.Subregion,
		FlagUrl:      c.FlagURL,
	}
}
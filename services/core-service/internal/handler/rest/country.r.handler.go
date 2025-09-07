package hrest

import (
	"core-service/internal/usecase"
	"net/http"
	"x/shared/response"
	"x/shared/utils/errors"
	"errors"
)

// CountryHandler holds usecases and dependencies
type CountryHandler struct {
	uc *usecase.CountryUsecase
}

// NewCountryHandler initializes a new handler
func NewCountryHandler(uc *usecase.CountryUsecase) *CountryHandler {
	return &CountryHandler{
		uc: uc,
	}
}

// HandleRefreshCountries triggers a refresh from remote API
func (h *CountryHandler) HandleRefreshCountries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := h.uc.RefreshCountries(ctx)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to refresh countries")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Countries updated successfully",
	})
}

func (h *CountryHandler) HandleGetCountries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.URL.Query().Get("code")

	if code != "" {
		// Single country case
		country, err := h.uc.GetCountryByISO2(ctx, code)
		if err != nil {
			if errors.Is(err, xerrors.ErrNotFound) {
				response.Error(w, http.StatusNotFound, "Country not found")
			} else {
				response.Error(w, http.StatusInternalServerError, "Failed to fetch country")
			}
			return
		}
		response.JSON(w, http.StatusOK, country)
		return
	}

	// List all countries
	countries, err := h.uc.GetAllCountries(ctx)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch countries")
		return
	}
	response.JSON(w, http.StatusOK, countries)
}


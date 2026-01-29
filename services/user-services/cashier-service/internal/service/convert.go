// internal/service/currency.go
package service

import (
	"context"
	"fmt"
	"math"

	partnersvcpb "x/shared/genproto/partner/svcpb"
	partnerclient "x/shared/partner"
)

type CurrencyService struct {
	partnerClient *partnerclient.PartnerService
}

func NewCurrencyService(partnerClient *partnerclient.PartnerService) *CurrencyService {
	return &CurrencyService{
		partnerClient: partnerClient,
	}
}

// ConvertToUSD converts local currency to USD using partner rate
// amount: 1000 KES, partner.Rate: 129.50 → returns 7.72 USD
func (s *CurrencyService) ConvertToUSD(
	ctx context.Context,
	partner *partnersvcpb.Partner,
	amountInLocal float64,
) (amountInUSD float64, rate float64) {

	// Partner rate:  1 USD = X LocalCurrency
	// To convert:  LocalCurrency / Rate = USD
	rate = partner.Rate
	amountInUSD = amountInLocal / rate

	//  Round to 2 decimal places
	amountInUSD = roundTo2Decimals(amountInUSD)

	return amountInUSD, rate
}

// ConvertFromUSD converts USD to local currency using partner rate
// amount: 10 USD, partner.Rate: 129.50 → returns 1295.00 KES
func (s *CurrencyService) ConvertFromUSD(
	ctx context.Context,
	partner *partnersvcpb.Partner,
	amountInUSD float64,
) (amountInLocal float64, rate float64) {

	// Partner rate:   1 USD = X LocalCurrency
	// To convert: USD * Rate = LocalCurrency
	rate = partner.Rate
	amountInLocal = amountInUSD * rate

	//  Round to 2 decimal places
	amountInLocal = roundTo2Decimals(amountInLocal)

	return amountInLocal, rate
}

// ValidateAmount checks if amount is within valid range for NUMERIC(20,2)
// Max value: 99,999,999,999,999,999.99 (18 digits before decimal + 2 after)
func (s *CurrencyService) ValidateAmount(amount float64) error {
	//  Check for negative
	if amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}

	//  Check for zero
	if amount <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}

	//  Check maximum value for NUMERIC(20,2)
	// 20 total digits:  18 before decimal, 2 after
	// Maximum: 99,999,999,999,999,999.99
	const maxAmount = 99999999999999999.99
	if amount > maxAmount {
		return fmt.Errorf("amount %.2f exceeds maximum allowed value %.2f", amount, maxAmount)
	}

	//  Check minimum value (1 cent)
	if amount < 0.01 {
		return fmt.Errorf("amount must be at least 0.01")
	}

	return nil
}

// ConvertToUSDWithValidation converts and validates the result
func (s *CurrencyService) ConvertToUSDWithValidation(
	ctx context.Context,
	partner *partnersvcpb.Partner,
	amountInLocal float64,
) (amountInUSD float64, rate float64, err error) {

	//  Validate input amount
	if err := s.ValidateAmount(amountInLocal); err != nil {
		return 0, 0, fmt.Errorf("invalid input amount: %w", err)
	}

	//  Validate partner rate
	if partner.Rate <= 0 {
		return 0, 0, fmt.Errorf("invalid exchange rate: %f", partner.Rate)
	}

	// Convert
	amountInUSD, rate = s.ConvertToUSD(ctx, partner, amountInLocal)

	//  Validate output amount
	if err := s.ValidateAmount(amountInUSD); err != nil {
		return 0, 0, fmt.Errorf("conversion resulted in invalid amount: %w", err)
	}

	return amountInUSD, rate, nil
}

// ConvertFromUSDWithValidation converts and validates the result
func (s *CurrencyService) ConvertFromUSDWithValidation(
	ctx context.Context,
	partner *partnersvcpb.Partner,
	amountInUSD float64,
) (amountInLocal float64, rate float64, err error) {

	//  Validate input amount
	if err := s.ValidateAmount(amountInUSD); err != nil {
		return 0, 0, fmt.Errorf("invalid input amount:   %w", err)
	}

	//  Validate partner rate
	if partner.Rate <= 0 {
		return 0, 0, fmt.Errorf("invalid exchange rate: %f", partner.Rate)
	}

	// Convert
	amountInLocal, rate = s.ConvertFromUSD(ctx, partner, amountInUSD)

	//  Validate output amount
	if err := s.ValidateAmount(amountInLocal); err != nil {
		return 0, 0, fmt.Errorf("conversion resulted in invalid amount: %w", err)
	}

	return amountInLocal, rate, nil
}

// GetPartnerForAgent gets the partner associated with an agent (if any)
func (s *CurrencyService) GetPartnerForAgent(
	ctx context.Context,
	agentID string,
) (*partnersvcpb.Partner, error) {
	// TODO: Implement agent-to-partner mapping
	return nil, fmt.Errorf("agent currency conversion not yet implemented")
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// roundTo2Decimals rounds a float64 to 2 decimal places
func roundTo2Decimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// roundTo8Decimals rounds a float64 to 8 decimal places (for crypto)
func roundTo8Decimals(value float64) float64 {
	return math.Round(value*100000000) / 100000000
}

// FormatAmount formats amount with 2 decimal places
func FormatAmount(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

// ParseAmount parses and validates amount string
func ParseAmount(amountStr string) (float64, error) {
	var amount float64
	_, err := fmt.Sscanf(amountStr, "%f", &amount)
	if err != nil {
		return 0, fmt.Errorf("invalid amount format: %w", err)
	}

	// Round to 2 decimals
	amount = roundTo2Decimals(amount)

	return amount, nil
}

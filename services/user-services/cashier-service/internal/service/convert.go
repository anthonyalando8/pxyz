// internal/service/currency. go
package service

import (
    "context"
    "fmt"
	 partnerclient "x/shared/partner"
	//accountingclient "x/shared/common/accounting"
    partnersvcpb "x/shared/genproto/partner/svcpb"
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
// amount: 1000 KES, partner. Rate: 129.50 → returns 7.72 USD
func (s *CurrencyService) ConvertToUSD(
    ctx context.Context,
    partner *partnersvcpb.Partner,
    amountInLocal float64,
) (amountInUSD float64, rate float64) {
    
    // Partner rate:   1 USD = X LocalCurrency
    // To convert:   LocalCurrency / Rate = USD
    rate = partner.Rate
    amountInUSD = amountInLocal / rate
    
    return amountInUSD, rate
}

// ConvertFromUSD converts USD to local currency using partner rate
// amount: 10 USD, partner.Rate: 129.50 → returns 1295 KES
func (s *CurrencyService) ConvertFromUSD(
    ctx context.Context,
    partner *partnersvcpb.Partner,
    amountInUSD float64,
) (amountInLocal float64, rate float64) {
    
    // Partner rate:  1 USD = X LocalCurrency
    // To convert:  USD * Rate = LocalCurrency
    rate = partner.Rate
    amountInLocal = amountInUSD * rate
    
    return amountInLocal, rate
}

// GetPartnerForAgent gets the partner associated with an agent (if any)
// This is a placeholder - implement based on your business logic
func (s *CurrencyService) GetPartnerForAgent(
    ctx context.Context,
    agentID string,
) (*partnersvcpb.Partner, error) {
    // TODO: Implement agent-to-partner mapping
    // For now, return error
    return nil, fmt.Errorf("agent currency conversion not yet implemented")
}
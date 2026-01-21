package service
// accounting-service/internal/service/fee_calculator.go

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"accounting-service/internal/domain"

	cryptopb "x/shared/genproto/shared/accounting/cryptopb"
)


// ✅ NEW: Get network fee from crypto service
func (uc *TransactionFeeCalculator) getNetworkFee(
	ctx context.Context,
	currency string,
	amount float64,
	toAddress string,
) (*domain.NetworkFeeCalculation, error) {
	
	if uc.cryptoClient == nil {
		return nil, fmt. Errorf("crypto client not available")
	}

	// Check cache first (network fees change but not too frequently)
	cacheKey := fmt.Sprintf("network_fee:%s: %.0f", currency, amount)
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var cached domain.NetworkFeeCalculation
		if jsonErr := json.Unmarshal([]byte(val), &cached); jsonErr == nil {
			return &cached, nil
		}
	}

	// Map currency to chain and asset
	chain, asset := uc.getCurrencyChainMapping(currency)
	if chain == cryptopb.Chain_CHAIN_UNSPECIFIED {
		return nil, fmt.Errorf("unsupported crypto currency: %s", currency)
	}

	// Convert amount to smallest unit (satoshis, SUN, wei)
	amountInSmallestUnit := uc.convertToSmallestUnit(amount, currency)

	// Call crypto service to estimate network fee
	resp, err := uc.cryptoClient.TransactionClient.EstimateNetworkFee(ctx, &cryptopb.EstimateNetworkFeeRequest{
		Chain:     chain,
		Asset:     asset,
		Amount:    fmt.Sprintf("%d", amountInSmallestUnit),
		ToAddress: toAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("crypto service estimate fee failed: %w", err)
	}

	// Convert fee back to standard units
	feeInStandardUnits := uc.convertFromSmallestUnit(resp.FeeAmount, resp. FeeCurrency)

	networkFeeCalc := &domain.NetworkFeeCalculation{
		Amount:        feeInStandardUnits,
		Currency:      resp.FeeCurrency,
		EstimatedAt:   time.Now(),
		ValidFor:      time.Duration(resp.ValidFor) * time.Second,
		Explanation:   resp.Explanation,
	}

	// Cache for 2 minutes (network fees can change)
	if data, err := json.Marshal(networkFeeCalc); err == nil {
		_ = uc.redisClient. Set(ctx, cacheKey, data, 2*time. Minute).Err()
	}

	return networkFeeCalc, nil
}

// ✅ NEW: Check if currency is crypto
func (uc *TransactionFeeCalculator) isCryptoCurrency(currency string) bool {
	cryptoCurrencies := map[string]bool{
		"BTC":  true,
		"USDT": true,
		"TRX":  true,
		"ETH":  true,
		"USDC": true,
		// Add more as needed
	}
	return cryptoCurrencies[currency]
}

// ✅ NEW: Map currency to chain and asset
func (uc *TransactionFeeCalculator) getCurrencyChainMapping(currency string) (cryptopb.Chain, string) {
	mappings := map[string]struct {
		Chain cryptopb. Chain
		Asset string
	}{
		"BTC":  {cryptopb.Chain_CHAIN_BITCOIN, "BTC"},
		"USDT": {cryptopb.Chain_CHAIN_TRON, "USDT"}, // USDT on TRON by default
		"TRX":   {cryptopb.Chain_CHAIN_TRON, "TRX"},
		"ETH":  {cryptopb. Chain_CHAIN_ETHEREUM, "ETH"},
		"USDC": {cryptopb. Chain_CHAIN_ETHEREUM, "USDC"},
		// Add more mappings
	}

	if mapping, ok := mappings[currency]; ok {
		return mapping.Chain, mapping.Asset
	}

	return cryptopb.Chain_CHAIN_UNSPECIFIED, ""
}

// ✅ NEW: Convert to smallest unit (satoshis, SUN, wei)
func (uc *TransactionFeeCalculator) convertToSmallestUnit(amount float64, currency string) int64 {
	decimals := map[string]int{
		"BTC":  8,  // satoshis
		"TRX":  6,  // SUN
		"USDT":  6,  // SUN (on TRON)
		"ETH":  18, // wei
		"USDC": 6,  // (on Ethereum)
	}

	if dec, ok := decimals[currency]; ok {
		multiplier := math.Pow(10, float64(dec))
		return int64(amount * multiplier)
	}

	return int64(amount * 1e8) // Default to 8 decimals
}

// ✅ NEW: Convert from smallest unit back to standard
func (uc *TransactionFeeCalculator) convertFromSmallestUnit(amountStr, currency string) float64 {
	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		return 0
	}

	decimals := map[string]int{
		"BTC":  8,
		"TRX":  6,
		"USDT":  6,
		"ETH":  18,
		"USDC": 6,
	}

	if dec, ok := decimals[currency]; ok {
		divisor := math.Pow(10, float64(dec))
		return float64(amount) / divisor
	}

	return float64(amount) / 1e8
}

// ✅ NEW: Get estimated network fee (fallback when crypto service unavailable)
func (uc *TransactionFeeCalculator) getEstimatedNetworkFee(currency string) *domain.NetworkFeeCalculation {
	// Conservative estimates for common crypto currencies
	estimates := map[string]struct {
		Amount   float64
		Currency string
	}{
		"BTC":  {0.00005, "BTC"},   // ~5000 satoshis
		"TRX":  {0.1, "TRX"},        // 0.1 TRX for native transfer
		"USDT": {5.0, "TRX"},        // ~5 TRX for TRC20 transfer
		"ETH":  {0.001, "ETH"},      // ~0.001 ETH (varies with gas)
		"USDC": {0.002, "ETH"},      // ~0.002 ETH for ERC20
	}

	if est, ok := estimates[currency]; ok {
		return &domain.NetworkFeeCalculation{
			Amount:      est.Amount,
			Currency:    est.Currency,
			EstimatedAt: time.Now(),
			ValidFor:    5 * time.Minute,
			Explanation: fmt.Sprintf("Estimated %s network fee (crypto service unavailable)", currency),
		}
	}

	return nil
}

// handler/deposit_validators.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	// "strconv"

	// "cashier-service/internal/usecase/transaction"
	"go.uber.org/zap"
)

// parseDepositRequest parses and returns the deposit request
func (h *PaymentHandler) parseDepositRequest(data json.RawMessage) (*DepositRequest, error) {
	var req DepositRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request format")
	}

	// Round amount to 8 decimal places (supports crypto precision)
	// Only if amount is provided
	if req.Amount > 0 {
		req.Amount = roundTo8Decimals(req.Amount)
	}

	return &req, nil
}

// validateDepositRequest validates the deposit request data
func (h *PaymentHandler) validateDepositRequest(req *DepositRequest) error {
	//  Service is always required
	if req.Service == "" {
		return fmt.Errorf("service is required")
	}

	// Validate service types
	validServices := map[string]bool{
		"mpesa":  true,
		"agent":  true,
		"crypto": true,
	}

	if !validServices[req.Service] {
		return fmt.Errorf("invalid service: %s (supported: mpesa, agent, crypto)", req.Service)
	}

	//  For crypto deposits: amount is optional, but currency is required
	if req.Service == "crypto" {
		if req.LocalCurrency == "" {
			return fmt.Errorf("local_currency is required for crypto deposits (e.g., BTC, USDT, TRX)")
		}

		// Amount is optional for crypto - user deposits externally
		// If amount is provided, validate it
		if req.Amount > 0 {
			if err := h.validateCryptoAmount(req.Amount, req.LocalCurrency); err != nil {
				return err
			}
		}

		// No other validations needed for crypto
		return nil
	}

	//  For non-crypto deposits: amount is required
	if req.Amount <= 0 {
		return fmt.Errorf("amount is required and must be greater than zero")
	}

	// Service-specific minimum amounts (non-crypto)
	minAmount := 5.0 // Default minimum for M-Pesa and Agent

	if req.Amount < minAmount {
		return fmt.Errorf("amount must be at least %.2f", minAmount)
	}

	// Maximum amount validation
	if req.Amount > 999999999999999999.99 {
		return fmt.Errorf("amount exceeds maximum allowed value")
	}

	// Service-specific validations
	if req.Service == "agent" && (req.AgentID == nil || *req.AgentID == "") {
		return fmt.Errorf("agent_id is required for agent deposits")
	}

	return nil
}

//  NEW: Validate crypto amount if provided
func (h *PaymentHandler) validateCryptoAmount(amount float64, currency string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be greater than zero if provided")
	}

	// Crypto-specific minimum amounts (if user provides amount)
	var minAmount float64
	switch currency {
	case "BTC":
		minAmount = 0.0001 // 0.0001 BTC minimum
	case "USDT", "USDC":
		minAmount = 1.0 // 1 USDT/USDC minimum
	case "TRX":
		minAmount = 10.0 // 10 TRX minimum
	case "ETH":
		minAmount = 0.001 // 0.001 ETH minimum
	default:
		minAmount = 0.00000001 // Very small minimum for unknown assets
	}

	if amount < minAmount {
		return fmt.Errorf("if amount is provided, it must be at least %.8f %s", minAmount, currency)
	}

	// Maximum amount validation
	if amount > 999999999999999999.99 {
		return fmt.Errorf("amount exceeds maximum allowed value")
	}

	return nil
}

// validateUserProfile validates user profile and returns phone and bank account
func (h *PaymentHandler) validateUserProfile(ctx context.Context, userID, service string) (string, string, error) {
	// Crypto doesn't need phone/bank validation
	if service == "crypto" {
		return "", "", nil
	}

	// Fetch user profile
	profile, err := h.profileFetcher.FetchProfile(ctx, "user", userID)
	if err != nil {
		h.logger.Error("failed to fetch user profile",
			zap.String("user_id", userID),
			zap.String("service", service),
			zap.Error(err))
		return "", "", fmt.Errorf("failed to fetch user profile")
	}

	var phoneNumber, bankAccount string

	// Check phone number requirement
	if servicesRequiringPhone[service] {
		if profile.Phone == "" {
			h.logger.Warn("phone number required but not set",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt.Errorf("phone number is required for this payment method. Please add your phone number in profile settings")
		}
		phoneNumber = profile.Phone
		h.logger.Info("phone number validated",
			zap.String("user_id", userID),
			zap.String("service", service),
			zap.String("phone", phoneNumber))
	}

	// Check bank account requirement (not used yet, but kept for future)
	if servicesRequiringBank[service] {
		if profile.BankAccount == "" {
			h.logger.Warn("bank account required but not set",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt.Errorf("bank account is required for this payment method. Please add your bank account in profile settings")
		}
		bankAccount = profile.BankAccount
		h.logger.Info("bank account validated",
			zap.String("user_id", userID),
			zap.String("service", service),
			zap.String("bank_account", bankAccount))
	}

	return phoneNumber, bankAccount, nil
}

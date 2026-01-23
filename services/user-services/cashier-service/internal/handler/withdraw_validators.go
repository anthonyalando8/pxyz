// handler/withdrawal_validators.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"

	//"strconv"

	"go.uber.org/zap"
)

// parseWithdrawalRequest parses and returns the withdrawal request
func (h *PaymentHandler) parseWithdrawalRequest(data json.RawMessage) (*WithdrawalRequest, error) {
	var req WithdrawalRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request format")
	}

	// Round amount to 8 decimal places (supports crypto precision)
	req.Amount = roundTo8Decimals(req.Amount)

	return &req, nil
}

// validateWithdrawalVerification validates the verification token
func (h *PaymentHandler) validateWithdrawalVerification(ctx context.Context, client *Client, req *WithdrawalRequest) error {
	if req.VerificationToken == "" {
		return fmt.Errorf("verification_token is required. Please complete verification first")
	}

	if !req.Consent {
		return fmt.Errorf("you must consent before processing a withdrawal")
	}

	tokenUserID, err := h.validateVerificationToken(ctx, req.VerificationToken, VerificationPurposeWithdrawal)
	if err != nil {
		h.logger.Warn("invalid verification token",
			zap.String("user_id", client.UserID),
			zap.Error(err))
		return fmt.Errorf("invalid or expired verification token. Please verify again")
	}

	if tokenUserID != client.UserID {
		h.logger.Warn("token user mismatch",
			zap.String("client_user_id", client.UserID),
			zap.String("token_user_id", tokenUserID))
		return fmt.Errorf("verification token does not belong to you")
	}

	return nil
}

// validateWithdrawalRequest validates the withdrawal request data
func (h *PaymentHandler) validateWithdrawalRequest(req *WithdrawalRequest) error {
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}

	// Minimum amount validation
	minAmount := 5.0
	if req.LocalCurrency != "" {
		// Crypto has different minimums
		switch req.LocalCurrency {
		case "BTC":
			minAmount = 0.0001 // 0.0001 BTC minimum
		case "USDT", "USDC":
			minAmount = 1.0 // 1 USDT/USDC minimum
		case "TRX":
			minAmount = 10.0 // 10 TRX minimum
		case "ETH":
			minAmount = 0.001 // 0.001 ETH minimum
		}
	}

	if req.Amount < minAmount {
		return fmt.Errorf("amount must be at least %.8f %s", minAmount, req.LocalCurrency)
	}

	// Maximum amount validation
	if req.Amount > 999999999999999999.99 {
		return fmt.Errorf("amount exceeds maximum allowed value")
	}

	if req.Destination == "" {
		return fmt.Errorf("destination is required")
	}

	return nil
}

// validateUserProfileForWithdrawal fetches profile and validates payment method requirements
// Priority: Use provided destination, then fall back to profile phone/bank account
func (h *PaymentHandler) validateUserProfileForWithdrawal(ctx context.Context, userID string, req *WithdrawalRequest) (string, string, error) {
	// Fetch user profile
	profile, err := h.profileFetcher.FetchProfile(ctx, "user", userID)
	if err != nil {
		h.logger.Error("failed to fetch user profile",
			zap.String("user_id", userID),
			zap.Error(err))
		return "", "", fmt.Errorf("failed to fetch user profile")
	}

	// Determine service
	service := "mpesa" // default
	if req.Service != nil {
		service = *req.Service
	}

	// ✅ Crypto doesn't need phone/bank validation
	if service == "crypto" {
		return "", "", nil
	}

	var phoneNumber, bankAccount string

	// Check phone number requirement
	if servicesRequiringPhone[service] {
		// Priority: Use provided destination, then fall back to profile phone
		if req.Destination != "" {
			phoneNumber = req.Destination
			h.logger.Info("using provided destination for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("destination", phoneNumber))
		} else if profile.Phone != "" {
			phoneNumber = profile.Phone
			h.logger.Info("using profile phone number for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("phone", phoneNumber))
		} else {
			h.logger.Warn("phone number required but not provided",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt.Errorf("phone number is required for this payment method. Please provide a destination or add your phone number in profile settings")
		}
	}

	// Check bank account requirement
	if servicesRequiringBank[service] {
		// Priority: Use provided destination, then fall back to profile bank account
		if req.Destination != "" {
			bankAccount = req.Destination
			h.logger.Info("using provided destination for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("destination", bankAccount))
		} else if profile.BankAccount != "" {
			bankAccount = profile.BankAccount
			h.logger.Info("using profile bank account for withdrawal",
				zap.String("user_id", userID),
				zap.String("service", service),
				zap.String("bank_account", bankAccount))
		} else {
			h.logger.Warn("bank account required but not provided",
				zap.String("user_id", userID),
				zap.String("service", service))
			return "", "", fmt.Errorf("bank account is required for this payment method. Please provide a destination or add your bank account in profile settings")
		}
	}

	return phoneNumber, bankAccount, nil
}

// Service requirement maps
var (
	servicesRequiringPhone = map[string]bool{
		"mpesa":        true,
		"airtel_money": true,
		"mtn_momo":     true,
		"tigopesa":     true,
		"halopesa":     true,
	}

	servicesRequiringBank = map[string]bool{
		"bank_transfer": true,
		"bank":   true,
		"ach":           true,
		"wire":          true,
	}
)
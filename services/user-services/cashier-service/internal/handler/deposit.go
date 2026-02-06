// handler/deposit.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	cryptopb "x/shared/genproto/shared/accounting/cryptopb"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"go.uber.org/zap"
	// "cashier-service/internal/domain"
	// "go.uber.org/zap"
)

// DepositRequest represents the unified deposit request
type DepositRequest struct {
	Amount         float64 `json:"amount"`
	LocalCurrency  string  `json:"local_currency"`  // User's input currency
	TargetCurrency *string `json:"target_currency"` // Optional, defaults to USD
	Service        string  `json:"service"`         // "mpesa", "agent", "crypto"
	PartnerID      *string `json:"partner_id,omitempty"`
	AgentID        *string `json:"agent_id,omitempty"`
}

// DepositType determines the type of deposit
type DepositType string

const (
	DepositTypePartner DepositType = "partner" // M-Pesa (with conversion)
	DepositTypeAgent   DepositType = "agent"   // Agent (USD, no conversion)
	DepositTypeCrypto  DepositType = "crypto"  // Crypto (external, return address)
)

// DepositContext holds all deposit processing data
type DepositContext struct {
	Request   *DepositRequest
	UserID    string
	UserIDInt int64
	Type      DepositType

	// Currency conversion (partner only)
	AmountInUSD   float64
	ExchangeRate  float64
	LocalCurrency string

	// Partner flow
	Partner     *partnersvcpb.Partner
	PhoneNumber string

	// Agent flow
	Agent *accountingpb.Agent

	// Crypto flow
	CryptoWallet *cryptopb.Wallet
	CryptoChain  string
	CryptoAsset  string

	// Target
	TargetCurrency string
}

// handleDepositRequest orchestrates deposit flow
// handler/deposit.go

// handleDepositRequest orchestrates deposit flow
func (h *PaymentHandler) handleDepositRequest(ctx context.Context, client *Client, data json.RawMessage) {
	// Step 1: Parse and validate request
	req, err := h.parseDepositRequest(data)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 2: Basic validation
	if err := h.validateDepositRequest(req); err != nil {
		client.SendError(err.Error())
		return
	}

	userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

	//  Step 3: Validate wallet exists UPFRONT
	walletCurrency := h.determineWalletCurrency(req)
	walletExists, walletErr := h.validateWalletExists(ctx, client.UserID, walletCurrency)

	if walletErr != nil {
		h.logger.Error("failed to validate wallet existence",
			zap.String("user_id", client.UserID),
			zap.String("currency", walletCurrency),
			zap.Error(walletErr))
		client.SendError(fmt.Sprintf("Failed to validate wallet: %v", walletErr))
		return
	}

	if !walletExists {
		h.logger.Warn("wallet does not exist for deposit",
			zap.String("user_id", client.UserID),
			zap.String("currency", walletCurrency),
			zap.String("service", req.Service))

		//  Send wallet creation required response
		h.sendWalletCreationRequired(client, walletCurrency, req.Service)
		return
	}

	h.logger.Info("wallet validation passed",
		zap.String("user_id", client.UserID),
		zap.String("currency", walletCurrency),
		zap.String("service", req.Service))

	// Step 4: Determine deposit type
	depositType := h.determineDepositType(req)

	// Step 5: Build deposit context based on type
	dctx, err := h.buildDepositContext(ctx, client.UserID, userIDInt, req, depositType)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 6: Process deposit based on type
	h.processDepositByType(ctx, client, dctx)
}

// determineWalletCurrency determines which currency wallet to check
func (h *PaymentHandler) determineWalletCurrency(req *DepositRequest) string {
	// If local_currency is provided, use it
	if req.LocalCurrency != "" {
		return req.LocalCurrency
	}

	// If target_currency is provided, use it
	if req.TargetCurrency != nil && *req.TargetCurrency != "" {
		return *req.TargetCurrency
	}

	// Default to USD
	return "USD"
}

// validateWalletExists checks if user has accounting wallet for currency
func (h *PaymentHandler) validateWalletExists(ctx context.Context, userID, currency string) (bool, error) {
	h.logger.Debug("checking wallet existence",
		zap.String("user_id", userID),
		zap.String("currency", currency))

	// Try to get user's account for this currency
	purpose := accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET
	accountNumber, err := h.GetAccountByCurrency(ctx, userID, "user", currency, &purpose)
	if err != nil {
		// Check if error is "account not found"
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "not found") ||
			strings.Contains(errMsg, "no account") ||
			strings.Contains(errMsg, "does not exist") {
			h.logger.Debug("wallet not found",
				zap.String("user_id", userID),
				zap.String("currency", currency))
			return false, nil // Wallet doesn't exist
		}
		// Other error - return it
		h.logger.Error("error checking wallet existence",
			zap.String("user_id", userID),
			zap.String("currency", currency),
			zap.Error(err))
		return false, err
	}

	// Wallet exists
	h.logger.Debug("wallet found",
		zap.String("user_id", userID),
		zap.String("currency", currency),
		zap.String("account_number", accountNumber))

	return accountNumber != "", nil
}

// sendWalletCreationRequired sends wallet creation required response
func (h *PaymentHandler) sendWalletCreationRequired(client *Client, currency, service string) {
	// Get chain/asset info if crypto
	var chain, asset string
	isCrypto := false

	if service == "crypto" {
		c, a, err := h.mapCurrencyToCrypto(currency)
		if err == nil {
			chain = c
			asset = a
			isCrypto = true
		}
	}

	walletWSMessage := map[string]interface{}{
		"type": "create_account",
		"data": map[string]interface{}{
			"currency": currency,
			//"service":  service,
		},
	}

	data := map[string]interface{}{
		"currency":        currency,
		"service":         service,
		"requires_wallet": true,
		"wallet_ws_message": walletWSMessage,
		"message":         fmt.Sprintf("You need to create a %s wallet first", currency),
		"action_required": "CREATE_WALLET",
	}

	if isCrypto {
		data["chain"] = chain
		data["asset"] = asset
		data["wallet_type"] = "crypto"
	} else {
		data["wallet_type"] = "fiat"
	}

	response := map[string]interface{}{
		"success": false,
		"error":   fmt.Sprintf("Wallet not found for %s", currency),
		"code":    "WALLET_NOT_FOUND",
		"data":    data,
	}

	client.SendJSON(response)

	h.logger.Info("wallet creation required response sent",
		zap.String("user_id", client.UserID),
		zap.String("currency", currency),
		zap.String("service", service),
		zap.Bool("is_crypto", isCrypto),
	)
}


// determineDepositType determines which type of deposit this is
func (h *PaymentHandler) determineDepositType(req *DepositRequest) DepositType {
	// Priority: Crypto > Agent > Partner

	if req.Service == "crypto" {
		return DepositTypeCrypto
	}

	if req.AgentID != nil && *req.AgentID != "" {
		return DepositTypeAgent
	}

	// Default to partner (M-Pesa)
	return DepositTypePartner
}

// buildDepositContext builds the deposit context based on type
func (h *PaymentHandler) buildDepositContext(
	ctx context.Context,
	userID string,
	userIDInt int64,
	req *DepositRequest,
	dtype DepositType,
) (*DepositContext, error) {

	dctx := &DepositContext{
		Request:   req,
		UserID:    userID,
		UserIDInt: userIDInt,
		Type:      dtype,
	}

	// Determine target currency
	dctx.TargetCurrency = "USD" // Default
	if req.TargetCurrency != nil && *req.TargetCurrency != "" {
		dctx.TargetCurrency = *req.TargetCurrency
	}

	switch dtype {
	case DepositTypeAgent:
		return h.buildAgentDepositContext(ctx, dctx)

	case DepositTypeCrypto:
		return h.buildCryptoDepositContext(ctx, dctx)

	case DepositTypePartner:
		return h.buildPartnerDepositContext(ctx, dctx)

	default:
		return nil, fmt.Errorf("unsupported deposit type")
	}
}

// processDepositByType routes to the appropriate processor
func (h *PaymentHandler) processDepositByType(ctx context.Context, client *Client, dctx *DepositContext) {
	switch dctx.Type {
	case DepositTypeAgent:
		h.processAgentDeposit(ctx, client, dctx)

	case DepositTypeCrypto:
		h.processCryptoDeposit(ctx, client, dctx)

	case DepositTypePartner:
		h.processPartnerDeposit(ctx, client, dctx)

	default:
		client.SendError("unsupported deposit type")
	}
}

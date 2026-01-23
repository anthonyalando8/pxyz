// handler/deposit.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	cryptopb "x/shared/genproto/shared/accounting/cryptopb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
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
	Request    *DepositRequest
	UserID     string
	UserIDInt  int64
	Type       DepositType

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

	// Step 3: Determine deposit type
	depositType := h.determineDepositType(req)

	// Step 4: Build deposit context based on type
	dctx, err := h.buildDepositContext(ctx, client.UserID, userIDInt, req, depositType)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 5: Process deposit based on type
	h.processDepositByType(ctx, client, dctx)
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
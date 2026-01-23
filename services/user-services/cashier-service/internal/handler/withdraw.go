// handler/withdrawal.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"

)

// WithdrawalRequest represents the unified withdrawal request
type WithdrawalRequest struct {
	Amount            float64 `json:"amount"`
	LocalCurrency     string  `json:"local_currency"`      // Required for crypto, extracted for partner
	Service           *string `json:"service,omitempty"`    // "mpesa", "bank_transfer", "crypto"
	AgentID           *string `json:"agent_id,omitempty"`  // For agent withdrawals
	Destination       string  `json:"destination"`          // Phone/Bank/Address
	VerificationToken string  `json:"verification_token"`
	Consent           bool    `json:"consent"`
}

// WithdrawalType determines the type of withdrawal
type WithdrawalType string

const (
	WithdrawalTypeAgent   WithdrawalType = "agent"
	WithdrawalTypePartner WithdrawalType = "partner"
	WithdrawalTypeCrypto  WithdrawalType = "crypto"
	WithdrawalTypeDirect  WithdrawalType = "direct"
)

// WithdrawalContext holds all withdrawal processing data
type WithdrawalContext struct {
	Request       *WithdrawalRequest
	UserID        string
	UserIDInt     int64
	Type          WithdrawalType
	
	// Currency conversion
	AmountInUSD   float64
	ExchangeRate  float64
	
	// Partner flow
	Partner       *partnersvcpb.Partner
	PhoneNumber   string
	BankAccount   string
	
	// Agent flow
	Agent         *accountingpb.Agent
	AgentAccount  *accountingpb.Account
	
	// Crypto flow
	CryptoAddress string
	CryptoChain   string
	CryptoAsset   string
	
	// Accounts
	UserAccount   string
	SystemAccount string
}

// Main handler - orchestrates withdrawal flow
func (h *PaymentHandler) handleWithdrawRequest(ctx context.Context, client *Client, data json.RawMessage) {
	// Step 1: Parse and validate request
	req, err := h.parseWithdrawalRequest(data)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 2: Validate verification token
	if err := h.validateWithdrawalVerification(ctx, client, req); err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 3: Basic request validation
	if err := h.validateWithdrawalRequest(req); err != nil {
		client.SendError(err.Error())
		return
	}

	userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

	// Step 4: Determine withdrawal type
	withdrawalType := h.determineWithdrawalType(req)

	// Step 5: Build withdrawal context based on type
	wctx, err := h.buildWithdrawalContext(ctx, client.UserID, userIDInt, req, withdrawalType)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 6: Process withdrawal based on type
	h.processWithdrawalByType(ctx, client, wctx)
}

// determineWithdrawalType determines which type of withdrawal this is
func (h *PaymentHandler) determineWithdrawalType(req *WithdrawalRequest) WithdrawalType {
	// Priority: Agent > Crypto > Partner > Direct
	
	if req.AgentID != nil && *req.AgentID != "" {
		return WithdrawalTypeAgent
	}
	
	if req.Service != nil && *req.Service == "crypto" {
		return WithdrawalTypeCrypto
	}
	
	if req.Service != nil && *req.Service != "" {
		return WithdrawalTypePartner
	}
	
	return WithdrawalTypeDirect
}

// buildWithdrawalContext builds the withdrawal context based on type
func (h *PaymentHandler) buildWithdrawalContext(
	ctx context.Context,
	userID string,
	userIDInt int64,
	req *WithdrawalRequest,
	wtype WithdrawalType,
) (*WithdrawalContext, error) {
	
	wctx := &WithdrawalContext{
		Request:   req,
		UserID:    userID,
		UserIDInt: userIDInt,
		Type:      wtype,
	}

	switch wtype {
	case WithdrawalTypeAgent:
		return h.buildAgentWithdrawalContext(ctx, wctx)
		
	case WithdrawalTypeCrypto:
		return h.buildCryptoWithdrawalContext(ctx, wctx)
		
	case WithdrawalTypePartner:
		return h.buildPartnerWithdrawalContext(ctx, wctx)
		
	case WithdrawalTypeDirect:
		return h.buildDirectWithdrawalContext(ctx, wctx)
		
	default:
		return nil, fmt.Errorf("unsupported withdrawal type")
	}
}

// processWithdrawalByType routes to the appropriate processor
func (h *PaymentHandler) processWithdrawalByType(ctx context.Context, client *Client, wctx *WithdrawalContext) {
	switch wctx.Type {
	case WithdrawalTypeAgent:
		h.processAgentWithdrawal(ctx, client, wctx)
		
	case WithdrawalTypeCrypto:
		h.processCryptoWithdrawal(ctx, client, wctx)
		
	case WithdrawalTypePartner:
		h.processPartnerWithdrawal(ctx, client, wctx)
		
	case WithdrawalTypeDirect:
		h.processDirectWithdrawal(ctx, client, wctx)
		
	default:
		client.SendError("unsupported withdrawal type")
	}
}
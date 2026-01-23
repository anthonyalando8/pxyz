// handler/deposit_agent.go
package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"cashier-service/internal/domain"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	"x/shared/utils/id"

	"go.uber.org/zap"
)

// buildAgentDepositContext builds context for agent deposit
func (h *PaymentHandler) buildAgentDepositContext(ctx context.Context, dctx *DepositContext) (*DepositContext, error) {
	req := dctx.Request

	// Validate agent ID
	if req.AgentID == nil || *req.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required for agent deposits")
	}

	// Fetch agent
	agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
		AgentExternalId: *req.AgentID,
		IncludeAccounts: false,
	})
	if err != nil {
		h.logger.Error("failed to get agent",
			zap.String("agent_id", *req.AgentID),
			zap.Error(err))
		return nil, fmt.Errorf("agent not found: %v", err)
	}

	if agentResp.Agent == nil || !agentResp.Agent.IsActive {
		return nil, fmt.Errorf("agent not found or inactive")
	}

	agent := agentResp.Agent

	// Get user profile for phone number
	phone, _, err := h.validateUserProfile(ctx, dctx.UserID, req.Service)
	if err != nil {
		return nil, err
	}
	dctx.PhoneNumber = phone

	//  Agent deposits are ALWAYS in USD (no conversion)
	dctx.AmountInUSD = req.Amount
	dctx.ExchangeRate = 1.0
	dctx.LocalCurrency = "USD"
	dctx.Agent = agent

	// Ensure target currency is USD for agent deposits
	dctx.TargetCurrency = "USD"

	h.logger.Info("agent deposit context built",
		zap.String("agent_id", agent.AgentExternalId),
		zap.String("agent_name", *agent.Name),
		zap.Float64("amount_usd", dctx.AmountInUSD),
		zap.String("phone", phone))

	return dctx, nil
}

// processAgentDeposit processes agent-assisted deposit
func (h *PaymentHandler) processAgentDeposit(ctx context.Context, client *Client, dctx *DepositContext) {
	req := dctx.Request

	// Create deposit request
	depositReq := &domain.DepositRequest{
		UserID:          dctx.UserIDInt,
		RequestRef:      id.GenerateTransactionID("DEP-AG"),
		Amount:          dctx.AmountInUSD,
		Currency:        "USD",
		Service:         req.Service,
		AgentExternalID: req.AgentID,
		PaymentMethod:   &req.Service,
		Status:          domain.DepositStatusPending,
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Store metadata (no conversion info since it's USD)
	depositReq.SetOriginalAmount(req.Amount, "USD", 1.0)
	if depositReq.Metadata == nil {
		depositReq.Metadata = make(map[string]interface{})
	}
	if dctx.PhoneNumber != "" {
		depositReq.Metadata["phone_number"] = dctx.PhoneNumber
	}
	depositReq.Metadata["deposit_type"] = "agent"
	depositReq.Metadata["agent_id"] = dctx.Agent.AgentExternalId
	depositReq.Metadata["agent_name"] = *dctx.Agent.Name

	// Save to database
	if err := h.userUc.CreateDepositRequest(ctx, depositReq); err != nil {
		h.logger.Error("failed to create deposit request", zap.Error(err))
		client.SendError(fmt.Sprintf("failed to create deposit request: %v", err))
		return
	}

	// Notify agent asynchronously
	go h.sendDepositRequestToAgent(depositReq, dctx)

	// Send success response
	client.SendSuccess("deposit request sent to agent", map[string]interface{}{
		"request_ref":   depositReq.RequestRef,
		"agent_id":      dctx.Agent.AgentExternalId,
		"agent_name":    *dctx.Agent.Name,
		"amount":        dctx.AmountInUSD,
		"currency":      "USD",
		"phone_number":  dctx.PhoneNumber,
		"status":        "sent_to_agent",
		"expires_at":    depositReq.ExpiresAt,
		"deposit_type":  "agent",
	})
}

// sendDepositRequestToAgent notifies agent about deposit request
func (h *PaymentHandler) sendDepositRequestToAgent(depositReq *domain.DepositRequest, dctx *DepositContext) {
	ctx := context.Background()

	// TODO: Implement agent notification system (SMS, push notification, etc.)
	log.Printf("[AgentDeposit] Deposit request %s sent to agent %s for %.2f USD (phone: %s)",
		depositReq.RequestRef, dctx.Agent.AgentExternalId, dctx.AmountInUSD, dctx.PhoneNumber)

	// Mark as sent to agent
	h.userUc.UpdateDepositStatus(ctx, depositReq.RequestRef, domain.DepositStatusSentToAgent, nil)

	h.logger.Info("deposit request sent to agent",
		zap.String("request_ref", depositReq.RequestRef),
		zap.String("agent_id", dctx.Agent.AgentExternalId),
		zap.Float64("amount", dctx.AmountInUSD),
		zap.String("phone", dctx.PhoneNumber))
}
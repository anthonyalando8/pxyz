// handler/transfer.go
package handler

import (
	"cashier-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	//"time"

	accountingpb "x/shared/genproto/shared/accounting/v1"
	//"x/shared/utils/id"

	"go.uber.org/zap"
)

// ============================================================================
// P2P TRANSFER & AGENT DEPOSIT
// ============================================================================

type TransferRequest struct {
	ToUserID          string  `json:"to_user_id"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	Description       string  `json:"description"`
	DepositRequestRef *string `json:"deposit_request_ref,omitempty"` // For agent deposit fulfillment
}

type TransferType string

const (
	TransferTypeP2P          TransferType = "p2p"           // Normal user-to-user
	TransferTypeAgentDeposit TransferType = "agent_deposit" // Agent fulfilling deposit
)

type TransferContext struct {
	Request          *TransferRequest
	Type             TransferType
	FromUserID       string
	ToUserID         string
	Amount           float64
	Currency         string
	Description      string
	
	// Agent info (if agent transfer)
	Agent            *accountingpb.Agent
	IsAgentTransfer  bool
	
	// Deposit info (if deposit fulfillment)
	DepositRequest   *domain.DepositRequest
	IsDepositFulfill bool
	
	// Accounts
	FromAccount      string
	ToAccount        string
}

// handleTransfer handles both P2P transfers and agent deposit fulfillment
func (h *PaymentHandler) handleTransfer(ctx context.Context, client *Client, data json.RawMessage) {
	// Step 1: Parse request
	req, err := h.parseTransferRequest(data)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 2: Basic validation
	if err := h.validateTransferRequest(req, client.UserID); err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 3: Build transfer context
	tctx, err := h.buildTransferContext(ctx, client.UserID, req)
	if err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 4: Validate balance
	if err := h.validateTransferBalance(ctx, tctx); err != nil {
		client.SendError(err.Error())
		return
	}

	// Step 5: Execute transfer
	h.executeTransfer(ctx, client, tctx)
}

// parseTransferRequest parses the transfer request
func (h *PaymentHandler) parseTransferRequest(data json.RawMessage) (*TransferRequest, error) {
	var req TransferRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request format")
	}

	// Round amount
	req.Amount = roundTo8Decimals(req.Amount)

	return &req, nil
}

// validateTransferRequest validates basic transfer request
func (h *PaymentHandler) validateTransferRequest(req *TransferRequest, fromUserID string) error {
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if req.ToUserID == "" {
		return fmt.Errorf("recipient user_id is required")
	}
	if req.ToUserID == fromUserID {
		return fmt.Errorf("cannot transfer to yourself")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	return nil
}

// buildTransferContext builds the transfer context
func (h *PaymentHandler) buildTransferContext(ctx context.Context, fromUserID string, req *TransferRequest) (*TransferContext, error) {
	tctx := &TransferContext{
		Request:    req,
		FromUserID: fromUserID,
		ToUserID:   req.ToUserID,
		Amount:     req.Amount,
		Currency:   req.Currency,
	}

	// Check if sender is an agent
	agentResp, err := h.accountingClient.Client.GetAgentByUserID(ctx, &accountingpb.GetAgentByUserIDRequest{
		UserExternalId:  fromUserID,
		IncludeAccounts: true,
	})

	if err == nil && agentResp.Agent != nil && agentResp.Agent.IsActive {
		tctx.Agent = agentResp.Agent
		tctx.IsAgentTransfer = true

		h.logger.Info("agent transfer detected",
			zap.String("agent_id", agentResp.Agent.AgentExternalId),
			zap.String("user_id", fromUserID))
	}

	// If deposit_request_ref provided, this is deposit fulfillment
	if req.DepositRequestRef != nil && *req.DepositRequestRef != "" {
		if err := h.validateDepositFulfillment(ctx, tctx); err != nil {
			return nil, err
		}
	}

	// Determine transfer type
	if tctx.IsDepositFulfill {
		tctx.Type = TransferTypeAgentDeposit
	} else {
		tctx.Type = TransferTypeP2P
	}

	// Get accounts
	fromAccount, err := h.GetAccountByCurrency(ctx, fromUserID, "user", req.Currency, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get your %s account: %v", req.Currency, err)
	}
	tctx.FromAccount = fromAccount

	toAccount, err := h.GetAccountByCurrency(ctx, req.ToUserID, "user", req.Currency, nil)
	if err != nil {
		return nil, fmt.Errorf("recipient %s account not found: %v", req.Currency, err)
	}
	tctx.ToAccount = toAccount

	// Build description
	tctx.Description = h.buildTransferDescription(tctx)

	h.logger.Info("transfer context built",
		zap.String("type", string(tctx.Type)),
		zap.String("from", tctx.FromUserID),
		zap.String("to", tctx.ToUserID),
		zap.Float64("amount", tctx.Amount),
		zap.String("currency", tctx.Currency),
		zap.Bool("is_agent", tctx.IsAgentTransfer),
		zap.Bool("is_deposit", tctx.IsDepositFulfill))

	return tctx, nil
}

// validateDepositFulfillment validates deposit fulfillment requirements
func (h *PaymentHandler) validateDepositFulfillment(ctx context.Context, tctx *TransferContext) error {
	req := tctx.Request

	// Must be an agent
	if !tctx.IsAgentTransfer {
		return fmt.Errorf("only agents can fulfill deposits")
	}

	// Get deposit request
	deposit, err := h.userUc.GetDepositByRef(ctx, *req.DepositRequestRef)
	if err != nil {
		h.logger.Error("deposit request not found",
			zap.String("request_ref", *req.DepositRequestRef),
			zap.Error(err))
		return fmt.Errorf("invalid deposit request reference")
	}

	if deposit == nil {
		return fmt.Errorf("deposit request not found")
	}

	// Verify deposit is for this agent
	if deposit.AgentExternalID == nil || *deposit.AgentExternalID != tctx.Agent.AgentExternalId {
		return fmt.Errorf("this deposit request is not assigned to you")
	}

	// Verify deposit status
	if deposit.Status != domain.DepositStatusSentToAgent && deposit.Status != domain.DepositStatusPending {
		return fmt.Errorf("deposit request is in invalid status: %s", deposit.Status)
	}

	// Verify recipient matches deposit user
	depositUserID := strconv.FormatInt(deposit.UserID, 10)
	if req.ToUserID != depositUserID {
		return fmt.Errorf("recipient must be the deposit user: %s", depositUserID)
	}

	// Verify amount matches (converted amount in USD)
	if deposit.Amount != req.Amount {
		return fmt.Errorf("amount must match deposit amount: %.2f %s",
			deposit.Amount, deposit.Currency)
	}

	// Verify currency matches
	if deposit.Currency != req.Currency {
		return fmt.Errorf("currency must match deposit currency: %s", deposit.Currency)
	}

	tctx.DepositRequest = deposit
	tctx.IsDepositFulfill = true

	h.logger.Info("deposit fulfillment validated",
		zap.String("agent_id", tctx.Agent.AgentExternalId),
		zap.String("deposit_ref", *req.DepositRequestRef),
		zap.Int64("deposit_user_id", deposit.UserID))

	return nil
}

// validateTransferBalance validates sender has sufficient balance
func (h *PaymentHandler) validateTransferBalance(ctx context.Context, tctx *TransferContext) error {
	balanceResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: tctx.FromAccount,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to check balance: %v", err)
	}

	if balanceResp.Balance.AvailableBalance < tctx.Amount {
		return fmt.Errorf("insufficient balance: available %.8f %s, need %.8f %s",
			balanceResp.Balance.AvailableBalance, tctx.Currency,
			tctx.Amount, tctx.Currency)
	}

	return nil
}

// buildTransferDescription builds the transfer description
func (h *PaymentHandler) buildTransferDescription(tctx *TransferContext) string {
	req := tctx.Request

	// Use provided description if available
	if req.Description != "" {
		return req.Description
	}

	// Build default description based on transfer type
	switch tctx.Type {
	case TransferTypeAgentDeposit:
		// Deposit fulfillment
		if origAmount, origCurrency, _, ok := tctx.DepositRequest.GetOriginalAmount(); ok {
			return fmt.Sprintf("Deposit fulfillment: %.2f %s (%.2f %s) via agent %s",
				origAmount, origCurrency, tctx.Amount, tctx.Currency, tctx.Agent.AgentExternalId)
		}
		return fmt.Sprintf("Deposit fulfillment: %.2f %s via agent %s",
			tctx.Amount, tctx.Currency, tctx.Agent.AgentExternalId)

	case TransferTypeP2P:
		// P2P transfer
		if tctx.IsAgentTransfer {
			return fmt.Sprintf("Agent transfer: %.2f %s from %s to %s",
				tctx.Amount, tctx.Currency, tctx.FromUserID, tctx.ToUserID)
		}
		return fmt.Sprintf("Transfer: %.2f %s from %s to %s",
			tctx.Amount, tctx.Currency, tctx.FromUserID, tctx.ToUserID)

	default:
		return fmt.Sprintf("Transfer: %.2f %s", tctx.Amount, tctx.Currency)
	}
}

// executeTransfer executes the transfer
func (h *PaymentHandler) executeTransfer(ctx context.Context, client *Client, tctx *TransferContext) {
	// Build transfer request
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:   tctx.FromAccount,
		ToAccountNumber:     tctx.ToAccount,
		Amount:              tctx.Amount,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         tctx.Description,
		CreatedByExternalId: tctx.FromUserID,
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
	}

	// Set transaction type based on transfer type
	switch tctx.Type {
	case TransferTypeAgentDeposit:
		transferReq.TransactionType = accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT
		transferReq.ExternalRef = tctx.Request.DepositRequestRef
		if tctx.Agent != nil {
			transferReq.AgentExternalId = &tctx.Agent.AgentExternalId
		}

	case TransferTypeP2P:
		transferReq.TransactionType = accountingpb.TransactionType_TRANSACTION_TYPE_TRANSFER
		if tctx.IsAgentTransfer {
			transferReq.AgentExternalId = &tctx.Agent.AgentExternalId
		}
	}

	// Execute transfer
	resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		h.logger.Error("transfer failed",
			zap.String("type", string(tctx.Type)),
			zap.Error(err))
		client.SendError("transfer failed: " + err.Error())
		return
	}

	h.logger.Info("transfer completed",
		zap.String("type", string(tctx.Type)),
		zap.String("receipt_code", resp.ReceiptCode),
		zap.Int64("journal_id", resp.JournalId))

	// If this was a deposit fulfillment, mark deposit as completed
	if tctx.IsDepositFulfill {
		h.completeDepositFulfillment(ctx, tctx, resp)
	}

	// Send success response
	h.sendTransferSuccessResponse(client, tctx, resp)

	// Notify recipient
	h.notifyTransferRecipient(tctx, resp)
}

// completeDepositFulfillment marks deposit as completed
func (h *PaymentHandler) completeDepositFulfillment(ctx context.Context, tctx *TransferContext, resp *accountingpb.TransferResponse) {
	if err := h.userUc.CompleteDeposit(ctx, *tctx.Request.DepositRequestRef, resp.ReceiptCode, resp.JournalId); err != nil {
		h.logger.Error("failed to complete deposit",
			zap.String("deposit_ref", *tctx.Request.DepositRequestRef),
			zap.Error(err))
		// Don't fail the transfer, just log the error
	} else {
		h.logger.Info("deposit completed via agent transfer",
			zap.String("deposit_ref", *tctx.Request.DepositRequestRef),
			zap.String("receipt_code", resp.ReceiptCode))
	}
}

// sendTransferSuccessResponse sends success response to sender
func (h *PaymentHandler) sendTransferSuccessResponse(client *Client, tctx *TransferContext, resp *accountingpb.TransferResponse) {
	response := map[string]interface{}{
		"receipt_code": resp.ReceiptCode,
		"journal_id":   resp.JournalId,
		"amount":       tctx.Amount,
		"currency":     tctx.Currency,
		"fee":          resp.FeeAmount,
		"to_user_id":   tctx.ToUserID,
		"transfer_type": string(tctx.Type),
		"created_at":   resp.CreatedAt.AsTime(),
	}

	// Add agent info if applicable
	if tctx.IsAgentTransfer {
		response["agent_id"] = tctx.Agent.AgentExternalId
		response["agent_name"] = tctx.Agent.Name
		response["agent_commission"] = resp.AgentCommission
	}

	// Add deposit info if applicable
	if tctx.IsDepositFulfill {
		response["deposit_request_ref"] = *tctx.Request.DepositRequestRef
		response["deposit_completed"] = true

		if origAmount, origCurrency, rate, ok := tctx.DepositRequest.GetOriginalAmount(); ok {
			response["original_amount"] = origAmount
			response["original_currency"] = origCurrency
			response["exchange_rate"] = rate
		}
	}

	message := "transfer completed"
	if tctx.Type == TransferTypeAgentDeposit {
		message = "deposit fulfilled successfully"
	}

	client.SendSuccess(message, response)
}

// notifyTransferRecipient notifies recipient via WebSocket
func (h *PaymentHandler) notifyTransferRecipient(tctx *TransferContext, resp *accountingpb.TransferResponse) {
	notificationData := map[string]interface{}{
		"from_user_id":  tctx.FromUserID,
		"amount":        tctx.Amount,
		"currency":      tctx.Currency,
		"description":   tctx.Description,
		"receipt_code":  resp.ReceiptCode,
		"transfer_type": string(tctx.Type),
	}

	// Add agent info if applicable
	if tctx.IsAgentTransfer {
		notificationData["agent_id"] = tctx.Agent.AgentExternalId
		notificationData["agent_name"] = *tctx.Agent.Name
	}

	// Add deposit info if applicable
	notificationType := "transfer_received"
	if tctx.IsDepositFulfill {
		notificationType = "deposit_completed"
		notificationData["deposit_request_ref"] = *tctx.Request.DepositRequestRef
		notificationData["is_deposit"] = true

		if origAmount, origCurrency, _, ok := tctx.DepositRequest.GetOriginalAmount(); ok {
			notificationData["original_amount"] = origAmount
			notificationData["original_currency"] = origCurrency
		}
	}

	notification := map[string]interface{}{
		"type": notificationType,
		"data": notificationData,
	}

	notificationJSON, _ := json.Marshal(notification)
	h.hub.SendToUser(tctx.ToUserID, notificationJSON)

	h.logger.Info("transfer notification sent",
		zap.String("type", notificationType),
		zap.String("to_user", tctx.ToUserID))
}
// handler/withdrawal_agent.go
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

// buildAgentWithdrawalContext builds context for agent withdrawal
func (h *PaymentHandler) buildAgentWithdrawalContext(ctx context.Context, wctx *WithdrawalContext) (*WithdrawalContext, error) {
	req := wctx.Request
	
	// Validate agent ID
	if req.AgentID == nil || *req.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required for agent withdrawals")
	}

	// Fetch agent with accounts
	agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
		AgentExternalId: *req.AgentID,
		IncludeAccounts: true,
	})
	if err != nil {
		h.logger.Error("failed to get agent",
			zap.String("agent_id", *req.AgentID),
			zap.Error(err))
		return nil, fmt.Errorf("invalid agent: %v", err)
	}

	if agentResp.Agent == nil || !agentResp.Agent.IsActive {
		return nil, fmt.Errorf("agent not found or inactive")
	}

	agent := agentResp.Agent

	// Find agent's USD account
	var agentAccount *accountingpb.Account
	for _, acc := range agent.Accounts {
		if acc.Currency == "USD" &&
			(acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET ||
				acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_COMMISSION) &&
			acc.IsActive && !acc.IsLocked {
			agentAccount = acc
			break
		}
	}

	if agentAccount == nil {
		return nil, fmt.Errorf("agent does not have an active USD account")
	}

	// Get user profile for phone/bank
	phone, bank, err := h.validateUserProfileForWithdrawal(ctx, wctx.UserID, req)
	if err != nil {
		return nil, err
	}

	// Agent withdrawals are always in USD (no conversion)
	wctx.AmountInUSD = req.Amount
	wctx.ExchangeRate = 1.0
	wctx.Agent = agent
	wctx.AgentAccount = agentAccount
	wctx.PhoneNumber = phone
	wctx.BankAccount = bank
	
	// Ensure local currency is USD for agent withdrawals
	req.LocalCurrency = "USD"

	// Get user USD account
	userAccount, err := h.GetAccountByCurrency(ctx, wctx.UserID, "user", "USD",nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %v", err)
	}
	wctx.UserAccount = userAccount

	// Validate balance
	balanceResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: userAccount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %v", err)
	}

	if balanceResp.Balance.AvailableBalance < wctx.AmountInUSD {
		return nil, fmt.Errorf("insufficient balance: need %.2f USD, have %.2f USD",
			wctx.AmountInUSD, balanceResp.Balance.AvailableBalance)
	}

	h.logger.Info("agent withdrawal context built",
		zap.String("agent_id", agent.AgentExternalId),
		zap.String("agent_account", agentAccount.AccountNumber),
		zap.Float64("amount_usd", wctx.AmountInUSD))

	return wctx, nil
}

// processAgentWithdrawal processes agent-assisted withdrawal
func (h *PaymentHandler) processAgentWithdrawal(ctx context.Context, client *Client, wctx *WithdrawalContext) {
	req := wctx.Request
	
	// Create withdrawal request
	withdrawalReq := &domain.WithdrawalRequest{
		UserID:          wctx.UserIDInt,
		RequestRef:      id.GenerateTransactionID("WD-AG"),
		Amount:          wctx.AmountInUSD,
		Currency:        "USD",
		Destination:     req.Destination,
		Service:         req.Service,
		AgentExternalID: req.AgentID,
		Status:          domain.WithdrawalStatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Store metadata
	withdrawalReq.SetOriginalAmount(req.Amount, "USD", 1.0)
	if withdrawalReq.Metadata == nil {
		withdrawalReq.Metadata = make(map[string]interface{})
	}
	if wctx.PhoneNumber != "" {
		withdrawalReq.Metadata["phone_number"] = wctx.PhoneNumber
	}
	if wctx.BankAccount != "" {
		withdrawalReq.Metadata["bank_account"] = wctx.BankAccount
	}
	withdrawalReq.Metadata["withdrawal_type"] = "agent"
	withdrawalReq.Metadata["agent_id"] = wctx.Agent.AgentExternalId
	withdrawalReq.Metadata["agent_name"] = *wctx.Agent.Name

	// Save to database
	if err := h.userUc.CreateWithdrawalRequest(ctx, withdrawalReq); err != nil {
		h.logger.Error("failed to create withdrawal request", zap.Error(err))
		client.SendError(fmt.Sprintf("failed to create withdrawal request: %v", err))
		return
	}

	// Process asynchronously
	go h.executeAgentWithdrawal(withdrawalReq, wctx)

	// Send success response
	client.SendSuccess("agent withdrawal request created", map[string]interface{}{
		"request_ref":     withdrawalReq.RequestRef,
		"amount_usd":      wctx.AmountInUSD,
		"withdrawal_type": "agent",
		"agent_id":        wctx.Agent.AgentExternalId,
		"agent_name":      *wctx.Agent.Name,
		"status":          "processing",
	})
}

// executeAgentWithdrawal executes the agent withdrawal
func (h *PaymentHandler) executeAgentWithdrawal(withdrawal *domain.WithdrawalRequest, wctx *WithdrawalContext) {
	ctx := context.Background()

	if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
		log.Printf("[AgentWithdrawal] Failed to mark as processing: %v", err)
		return
	}

	// Execute transfer (USD to agent account)
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:   wctx.UserAccount,
		ToAccountNumber:     wctx.AgentAccount.AccountNumber,
		Amount:              withdrawal.Amount,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         fmt.Sprintf("Agent withdrawal %.2f USD to %s",
			withdrawal.Amount, withdrawal.Destination),
		ExternalRef:         &withdrawal.RequestRef,
		CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
		AgentExternalId:     &wctx.Agent.AgentExternalId,
		TransactionType:     accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
	}

	resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		errMsg := err.Error()
		h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error": "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
		return
	}

	if err := h.userUc.UpdateWithdrawalWithReceipt(ctx, withdrawal.ID, resp.ReceiptCode, resp.JournalId, true); err != nil {
		log.Printf("[AgentWithdrawal] Failed to mark as completed: %v", err)
		return
	}

	h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "agent_id": "%s",
            "agent_name": "%s",
            "amount_usd": %.2f,
            "fee_amount": %.2f,
            "agent_commission": %.2f
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, 
		wctx.Agent.AgentExternalId, *wctx.Agent.Name,
		withdrawal.Amount, resp.FeeAmount, resp.AgentCommission)))
}
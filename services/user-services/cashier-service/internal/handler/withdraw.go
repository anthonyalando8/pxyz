package handler

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "strconv"
    
    "cashier-service/internal/domain"
    accountingpb "x/shared/genproto/shared/accounting/v1"
    "go.uber.org/zap"
)

// ============================================================================
// WITHDRAWAL OPERATIONS
// ============================================================================

// Handle withdrawal request
func (h *PaymentHandler) handleWithdrawRequest(ctx context. Context, client *Client, data json.RawMessage) {
	var req struct {
		Amount             float64 `json:"amount"`
		Currency           string  `json:"currency"`
		Destination        string  `json:"destination"`
		Service            *string `json:"service,omitempty"`
		AgentID            *string `json:"agent_id,omitempty"`
		VerificationToken  string  `json:"verification_token"` // ✅ Required token
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// ✅ Step 1: Validate verification token FIRST
	if req.VerificationToken == "" {
		client. SendError("verification_token is required.  Please complete verification first.")
		return
	}

	tokenUserID, err := h.validateVerificationToken(ctx, req.VerificationToken, VerificationPurposeWithdrawal)
	if err != nil {
		h.logger. Warn("invalid verification token",
			zap.String("user_id", client.UserID),
			zap.Error(err))
		client.SendError("invalid or expired verification token.  Please verify again.")
		return
	}

	// ✅ Ensure token belongs to the requesting user
	if tokenUserID != client.UserID {
		h.logger.Warn("token user mismatch",
			zap. String("client_user_id", client.UserID),
			zap.String("token_user_id", tokenUserID))
		client.SendError("verification token does not belong to you")
		return
	}

	h.logger.Info("withdrawal verification passed",
		zap.String("user_id", client.UserID),
		zap.Float64("amount", req.Amount),
		zap.String("currency", req.Currency))

	// ✅ Step 2: Validate withdrawal request
	if req.Amount <= 0 {
		client.SendError("amount must be greater than zero")
		return
	}
	if req.Currency == "" {
		client.SendError("currency is required")
		return
	}
	if req.Destination == "" {
		client.SendError("destination is required")
		return
	}

	userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)

	// ✅ Step 3: Validate and fetch agent if provided
	var agent *accountingpb.Agent
	var agentAccount *accountingpb.Account
	
	if req.AgentID != nil && *req.AgentID != "" {
		// Fetch agent with accounts
		agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
			AgentExternalId: *req.AgentID,
			IncludeAccounts: true,
		})
		
		if err != nil {
			h.logger.Error("failed to get agent",
				zap.String("agent_id", *req.AgentID),
				zap.Error(err))
			client.SendError("invalid agent: " + err.Error())
			return
		}
		
		if agentResp.Agent == nil || !agentResp.Agent.IsActive {
			client.SendError("agent not found or inactive")
			return
		}
		
		agent = agentResp.Agent
		
		// Find agent's wallet/commission account for the currency
		for _, acc := range agent.Accounts {
			if acc.Currency == req.Currency && 
			   (acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET ||
				acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_COMMISSION) &&
			   acc.IsActive && ! acc.IsLocked {
				agentAccount = acc
				break
			}
		}
		
		if agentAccount == nil {
			client.SendError(fmt.Sprintf("agent does not have an active %s account", req.Currency))
			return
		}
		
		h.logger.Info("withdrawal to agent",
			zap.Int64("user_id", userIDInt),
			zap.String("agent_id", agent.AgentExternalId),
			zap.String("agent_account", agentAccount.AccountNumber))
	}

	// ✅ Step 4: Get user account
	userAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.Currency)
	if err != nil {
		h.logger.Error("failed to get user account",
			zap.String("user_id", client.UserID),
			zap.String("currency", req.Currency),
			zap.Error(err))
		client.SendError("failed to get user account: " + err.Error())
		return
	}

	// ✅ Step 5: Check user balance
	balanceResp, err := h. accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: userAccount,
		},
	})
	if err != nil {
		client.SendError("failed to check balance: " + err.Error())
		return
	}

	if balanceResp.Balance. AvailableBalance < req.Amount {
		client.SendError(fmt.Sprintf("insufficient balance: available %.2f %s", 
			balanceResp.Balance.AvailableBalance, req.Currency))
		return
	}

	// ✅ Step 6: Create withdrawal request
	withdrawalReq, err := h.userUc.InitiateWithdrawal(
		ctx,
		userIDInt,
		req.Amount,
		req.Currency,
		req.Destination,
		req.Service,
		req. AgentID,
	)
	if err != nil {
		h.logger.Error("failed to create withdrawal request",
			zap.Int64("user_id", userIDInt),
			zap.Error(err))
		client.SendError("failed to create withdrawal request: " + err.Error())
		return
	}

	h.logger.Info("withdrawal request created",
		zap.String("request_ref", withdrawalReq.RequestRef),
		zap.Int64("user_id", userIDInt),
		zap.Float64("amount", req.Amount),
		zap.String("currency", req.Currency))

	// ✅ Step 7: Process withdrawal (different flow for agent vs. normal withdrawal)
	if agent != nil && agentAccount != nil {
		// Transfer to agent account
		go h.processWithdrawalToAgent(withdrawalReq, userAccount, agentAccount.AccountNumber, agent)
	} else {
		// Normal withdrawal (debit only)
		go h.processWithdrawal(withdrawalReq, userAccount)
	}

	// ✅ Step 8: Build response
	response := map[string]interface{}{
		"request_ref": withdrawalReq.RequestRef,
		"amount":      req.Amount,
		"currency":    req.Currency,
		"destination": req.Destination,
		"status":      "processing",
		"created_at":  withdrawalReq.CreatedAt. Unix(),
	}
	
	if req. AgentID != nil && *req.AgentID != "" {
		response["agent_id"] = *req.AgentID
		response["agent_name"] = agent.Name
		response["agent_account"] = agentAccount.AccountNumber
		response["withdrawal_type"] = "agent_assisted"
	} else {
		response["withdrawal_type"] = "direct"
	}

	client.SendSuccess("withdrawal request created and being processed", response)
}

// ✅ processWithdrawalToAgent - Transfer money from user to agent
func (h *PaymentHandler) processWithdrawalToAgent(
    withdrawal *domain.WithdrawalRequest,
    userAccount string,
    agentAccount string,
    agent *accountingpb.Agent,
) {
    ctx := context.Background()

    // Mark as processing
    if err := h. userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
        log.Printf("[Withdrawal] Failed to mark as processing: %v", err)
        return
    }

    // ✅ Execute transfer from user to agent
    transferReq := &accountingpb.TransferRequest{
        FromAccountNumber:   userAccount,
        ToAccountNumber:     agentAccount,
        Amount:              withdrawal.Amount,
        AccountType:         accountingpb. AccountType_ACCOUNT_TYPE_REAL,
        Description:         fmt.Sprintf("Withdrawal to %s via agent %s", withdrawal.Destination, *agent.Name),
        ExternalRef:         &withdrawal.RequestRef,
        CreatedByExternalId: fmt.Sprintf("%d", withdrawal. UserID),
        CreatedByType:       accountingpb. OwnerType_OWNER_TYPE_USER,
        AgentExternalId:     &agent.AgentExternalId, // ✅ Track agent commission
    }

    resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
    if err != nil {
        errMsg := err.Error()
        h.userUc.FailWithdrawal(ctx, withdrawal. RequestRef, errMsg)

        // Send failure notification
        h. hub.SendToUser(fmt. Sprintf("%d", withdrawal.UserID), []byte(fmt. Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error": "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
        return
    }

    // Mark as completed
    if err := h.userUc.CompleteWithdrawal(ctx, withdrawal. RequestRef, resp.ReceiptCode, resp.JournalId); err != nil {
        log.Printf("[Withdrawal] Failed to mark as completed: %v", err)
        return
    }

    // Send success notification
    h.hub.SendToUser(fmt.Sprintf("%d", withdrawal. UserID), []byte(fmt. Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "agent_id": "%s",
            "agent_name": "%s",
            "fee_amount": %.2f,
            "agent_commission": %.2f
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, agent.AgentExternalId, *agent.Name, resp.FeeAmount, resp.AgentCommission)))
}

// processWithdrawal - Normal withdrawal (debit from user to system)
func (h *PaymentHandler) processWithdrawal(withdrawal *domain.WithdrawalRequest, userAccount string) {
    ctx := context.Background()

    // Mark as processing
    if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
        log.Printf("[Withdrawal] Failed to mark as processing: %v", err)
        return
    }

    // Execute debit
    debitReq := &accountingpb.DebitRequest{
        AccountNumber:       userAccount,
        Amount:              withdrawal.Amount,
        Currency:            withdrawal.Currency,
        AccountType:         accountingpb. AccountType_ACCOUNT_TYPE_REAL,
        Description:         fmt.Sprintf("Withdrawal to %s", withdrawal.Destination),
        ExternalRef:         &withdrawal.RequestRef,
        CreatedByExternalId: fmt. Sprintf("%d", withdrawal.UserID),
        CreatedByType:       accountingpb. OwnerType_OWNER_TYPE_USER,
    }

    // Add service to description if provided
    if withdrawal.Service != nil {
        debitReq.Description = fmt.Sprintf("Withdrawal to %s via %s", withdrawal. Destination, *withdrawal.Service)
    }

    resp, err := h.accountingClient.Client.Debit(ctx, debitReq)
    if err != nil {
        errMsg := err.Error()
        h. userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

        // Send failure notification
        h.hub.SendToUser(fmt. Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error": "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
        return
    }

    // Mark as completed
    if err := h.userUc.CompleteWithdrawal(ctx, withdrawal.RequestRef, resp.ReceiptCode, resp.JournalId); err != nil {
        log.Printf("[Withdrawal] Failed to mark as completed: %v", err)
        return
    }

    // Send success notification
    h.hub. SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "balance_after": %.2f
        }
    }`, withdrawal. RequestRef, resp.ReceiptCode, resp.BalanceAfter)))
}

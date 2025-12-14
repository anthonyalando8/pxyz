package handler

import (
	"cashier-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	accountingpb "x/shared/genproto/shared/accounting/v1"

	"go.uber.org/zap"
)

// ============================================================================
// P2P TRANSFER
// ============================================================================

// Handle peer-to-peer transfer (agent-only feature)
func (h *PaymentHandler) handleTransfer(ctx context.Context, client *Client, data json.RawMessage) {
    var req struct {
        ToUserID        string  `json:"to_user_id"`
        Amount          float64 `json:"amount"`
        Currency        string  `json:"currency"`
        Description     string  `json:"description"`
        DepositRequestRef *string `json:"deposit_request_ref,omitempty"` // ✅ NEW: Link to deposit
    }

    if err := json.Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    // ✅ Step 1: Validation
    if req.Amount <= 0 {
        client.SendError("amount must be greater than zero")
        return
    }
    if req.ToUserID == "" {
        client.SendError("recipient user_id is required")
        return
    }
    if req.ToUserID == client.UserID {
        client.SendError("cannot transfer to yourself")
        return
    }
    if req.Currency == "" {
        client.SendError("currency is required")
        return
    }

    // ✅ Step 2: Check if sender is an agent
    agentResp, err := h.accountingClient.Client.GetAgentByUserID(ctx, &accountingpb.GetAgentByUserIDRequest{
        UserExternalId:  client.UserID,
        IncludeAccounts:  true,
    })

    if err != nil {
        h.logger.Error("failed to check agent status",
            zap.String("user_id", client.UserID),
            zap.Error(err))
        client.SendError("only agents can perform transfers")
        return
    }

    if agentResp.Agent == nil {
        client.SendError("only agents can perform transfers")
        return
    }

    agent := agentResp.Agent
    if ! agent.IsActive {
        client.SendError("your agent account is not active")
        return
    }

    // ✅ Step 3: If deposit_request_ref provided, validate and verify
    var depositRequest *domain.DepositRequest
    if req.DepositRequestRef != nil && *req.DepositRequestRef != "" {
        // Get deposit request
        deposit, err := h.userUc.GetDepositByRef(ctx, *req.DepositRequestRef)
        if err != nil {
            h.logger.Error("deposit request not found",
                zap.String("request_ref", *req.DepositRequestRef),
                zap.Error(err))
            client.SendError("invalid deposit request reference")
            return
        }

        if deposit == nil {
            client.SendError("deposit request not found")
            return
        }

        depositRequest = deposit

        // ✅ Verify deposit is for this agent
        if deposit.AgentExternalID == nil || *deposit.AgentExternalID != agent.AgentExternalId {
            client.SendError("this deposit request is not assigned to you")
            return
        }

        // ✅ Verify deposit status
        if deposit.Status != domain.DepositStatusSentToAgent && deposit.Status != domain.DepositStatusPending {
            client.SendError(fmt.Sprintf("deposit request is in invalid status: %s", deposit.Status))
            return
        }

        // ✅ Verify recipient matches deposit user
        depositUserID := strconv.FormatInt(deposit.UserID, 10)
        if req.ToUserID != depositUserID {
            client.SendError(fmt.Sprintf("recipient must be the deposit user: %s", depositUserID))
            return
        }

        // ✅ Verify amount matches (converted amount in USD)
        if deposit.Amount != req.Amount {
            client.SendError(fmt.Sprintf("amount must match deposit amount: %.2f %s", 
                deposit.Amount, deposit.Currency))
            return
        }

        // ✅ Verify currency matches
        if deposit.Currency != req.Currency {
            client.SendError(fmt.Sprintf("currency must match deposit currency: %s", deposit.Currency))
            return
        }

        h.logger.Info("deposit fulfillment via transfer",
            zap.String("agent_id", agent.AgentExternalId),
            zap.String("deposit_ref", *req.DepositRequestRef),
            zap.Int64("deposit_user_id", deposit.UserID))
    }

    h.logger.Info("agent transfer initiated",
        zap.String("agent_id", agent.AgentExternalId),
        zap.String("from_user", client.UserID),
        zap.String("to_user", req.ToUserID),
        zap.Float64("amount", req.Amount),
        zap.String("currency", req.Currency),
        zap.Bool("is_deposit", depositRequest != nil))

    // ✅ Step 4: Get sender's account (agent's account)
    fromAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.Currency)
    if err != nil {
        client.SendError("failed to get your account: " + err.Error())
        return
    }

    // ✅ Step 5: Get recipient's account
    toAccount, err := h.GetAccountByCurrency(ctx, req.ToUserID, "user", req.Currency)
    if err != nil {
        client.SendError("recipient account not found: " + err.Error())
        return
    }

    // ✅ Step 6: Check sender's balance
    balanceResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
        Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
            AccountNumber: fromAccount,
        },
    })
    if err != nil {
        client.SendError("failed to check balance:  " + err.Error())
        return
    }

    if balanceResp.Balance.AvailableBalance < req.Amount {
        client.SendError(fmt.Sprintf("insufficient balance: available %.2f %s", 
            balanceResp.Balance.AvailableBalance, req.Currency))
        return
    }

    // ✅ Step 7: Execute transfer via accounting service
    description := req.Description
    if description == "" {
        if depositRequest != nil {
            // Get original amount from metadata
            if origAmount, origCurrency, _, ok := depositRequest.GetOriginalAmount(); ok {
                description = fmt.Sprintf("Deposit fulfillment: %.2f %s (%.2f USD) via agent %s", 
                    origAmount, origCurrency, req.Amount, agent.AgentExternalId)
            } else {
                description = fmt.Sprintf("Deposit fulfillment: %.2f %s via agent %s", 
                    req.Amount, req.Currency, agent.AgentExternalId)
            }
        } else {
            description = fmt.Sprintf("Agent transfer from %s to %s", client.UserID, req.ToUserID)
        }
    }

    transferReq := &accountingpb.TransferRequest{
        FromAccountNumber:   fromAccount,
        ToAccountNumber:     toAccount,
        Amount:              req.Amount,
        AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
        Description:          description,
        CreatedByExternalId: client.UserID,
        CreatedByType:        accountingpb.OwnerType_OWNER_TYPE_USER,
        AgentExternalId:     &agent.AgentExternalId,
		TransactionType: accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT,
    }

    // ✅ Link to deposit if provided
    if depositRequest != nil {
        transferReq.ExternalRef = req.DepositRequestRef
    }

    resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
    if err != nil {
        h.logger.Error("transfer failed",
            zap.String("agent_id", agent.AgentExternalId),
            zap.Error(err))
        client.SendError("transfer failed: " + err.Error())
        return
    }

    h.logger.Info("agent transfer completed",
        zap.String("agent_id", agent.AgentExternalId),
        zap.String("receipt_code", resp.ReceiptCode),
        zap.Int64("journal_id", resp.JournalId))

    // ✅ Step 8: If this was a deposit fulfillment, mark deposit as completed
    if depositRequest != nil {
        if err := h.userUc.CompleteDeposit(ctx, *req.DepositRequestRef, resp.ReceiptCode, resp.JournalId); err != nil {
            h.logger.Error("failed to complete deposit",
                zap.String("deposit_ref", *req.DepositRequestRef),
                zap.Error(err))
            // Don't fail the transfer, just log the error
            // The deposit can be manually completed later
        } else {
            h.logger.Info("deposit completed via agent transfer",
                zap.String("deposit_ref", *req.DepositRequestRef),
                zap.String("receipt_code", resp.ReceiptCode))
        }
    }

    // ✅ Step 9: Send success response
    response := map[string]interface{}{
        "receipt_code":      resp.ReceiptCode,
        "journal_id":        resp.JournalId,
        "amount":            req.Amount,
        "currency":          req.Currency,
        "fee":               resp.FeeAmount,
        "agent_commission":  resp.AgentCommission,
        "agent_id":          agent.AgentExternalId,
        "agent_name":        agent.Name,
        "created_at":        resp.CreatedAt.AsTime(),
    }

    if depositRequest != nil {
        response["deposit_request_ref"] = *req.DepositRequestRef
        response["deposit_completed"] = true
        
        // Add original amount info if available
        if origAmount, origCurrency, rate, ok := depositRequest.GetOriginalAmount(); ok {
            response["original_amount"] = origAmount
            response["original_currency"] = origCurrency
            response["exchange_rate"] = rate
        }
    }

    client.SendSuccess("transfer completed", response)

    // ✅ Step 10: Notify recipient via WebSocket (if online)
    notificationData := map[string]interface{}{
        "type": "transfer_received",
        "data": map[string]interface{}{
            "from_user_id":     client.UserID,
            "agent_id":        agent.AgentExternalId,
            "agent_name":      *agent.Name,
            "amount":          req.Amount,
            "currency":        req.Currency,
            "description":     description,
            "receipt_code":    resp.ReceiptCode,
        },
    }

    // If this was a deposit, add deposit info to notification
    if depositRequest != nil {
        notificationData["data"].(map[string]interface{})["deposit_request_ref"] = *req.DepositRequestRef
        notificationData["data"].(map[string]interface{})["is_deposit"] = true
        notificationData["type"] = "deposit_completed"
        
        if origAmount, origCurrency, _, ok := depositRequest.GetOriginalAmount(); ok {
            notificationData["data"].(map[string]interface{})["original_amount"] = origAmount
            notificationData["data"].(map[string]interface{})["original_currency"] = origCurrency
        }
    }

    notificationJSON, _ := json.Marshal(notificationData)
    h.hub.SendToUser(req.ToUserID, notificationJSON)
}
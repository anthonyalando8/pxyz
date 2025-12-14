// handler/withdrawal.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"cashier-service/internal/domain"
	convsvc"cashier-service/internal/service"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"go.uber.org/zap"
)

// Handle withdrawal request
func (h *PaymentHandler) handleWithdrawRequest(ctx context.Context, client *Client, data json.RawMessage) {
    var req struct {
        Amount             float64 `json:"amount"`               // Amount user wants to receive locally
        LocalCurrency      string  `json:"local_currency"`       // ✅ Currency user will receive (KES, etc.)
        Service            *string `json:"service,omitempty"`
        AgentID            *string `json:"agent_id,omitempty"`   // ✅ If provided, use agent
        Destination        string  `json:"destination"`
        VerificationToken  string  `json:"verification_token"`
		Consent bool `json:"consent"`
    }

    if err := json.Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    // ✅ Step 1: Validate verification token
    if req.VerificationToken == "" {
        client.SendError("verification_token is required.Please complete verification first.")
        return
    }
	if !req.Consent {
		client.SendError("You have to consent before processing a withdraw.")
        return
	}

    tokenUserID, err := h.validateVerificationToken(ctx, req.VerificationToken, VerificationPurposeWithdrawal)
    if err != nil {
        h.logger.Warn("invalid verification token",
            zap.String("user_id", client.UserID),
            zap.Error(err))
        client.SendError("invalid or expired verification token.Please verify again.")
        return
    }

    if tokenUserID != client.UserID {
        h.logger.Warn("token user mismatch",
            zap.String("client_user_id", client.UserID),
            zap.String("token_user_id", tokenUserID))
        client.SendError("verification token does not belong to you")
        return
    }

    // ✅ Step 2: Validate request
    if req.Amount <= 0 {
        client.SendError("amount must be greater than zero")
        return
    }
    if req.LocalCurrency == "" {
        client.SendError("local_currency is required")
        return
    }
    if req.Destination == "" {
        client.SendError("destination is required")
        return
    }

    userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

    // ✅ Step 3: Determine flow and calculate USD amount
    var selectedPartner *partnersvcpb.Partner
    var agent *accountingpb.Agent
    var agentAccount *accountingpb.Account
    var amountInUSD float64
    var exchangeRate float64
    var isAgentFlow bool

    // Get partner for currency conversion (needed for both flows)
    var service string
    if req.Service != nil {
        service = *req.Service
    } else {
        // Default service based on currency/destination
        service = "mpesa" // or determine from destination
    }

    partners, err := h.GetPartnersByService(ctx, service)
    if err != nil || len(partners) == 0 {
        client.SendError("no partners available for currency conversion")
        return
    }
    selectedPartner = SelectRandomPartner(partners)

    // ✅ Convert local amount to USD
    currencyService := convsvc.NewCurrencyService(h.partnerClient)
    amountInUSD, exchangeRate = currencyService.ConvertToUSD(ctx, selectedPartner, req.Amount)

    if req.AgentID != nil && *req.AgentID != "" {
        // ===== AGENT FLOW =====
        isAgentFlow = true

        // Fetch agent with accounts
        agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
            AgentExternalId: *req.AgentID,
            IncludeAccounts: true,
        })
        
        if err != nil {
            h.logger.Error("failed to get agent",
                zap.String("agent_id", *req.AgentID),
                zap.Error(err))
            client.SendError("invalid agent:  " + err.Error())
            return
        }
        
        if agentResp.Agent == nil || ! agentResp.Agent.IsActive {
            client.SendError("agent not found or inactive")
            return
        }
        
        agent = agentResp.Agent
        
        // Find agent's USD account
        for _, acc := range agent.Accounts {
            if acc.Currency == "USD" && 
               (acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_WALLET ||
                acc.Purpose == accountingpb.AccountPurpose_ACCOUNT_PURPOSE_COMMISSION) &&
               acc.IsActive && ! acc.IsLocked {
                agentAccount = acc
                break
            }
        }
        
        if agentAccount == nil {
            client.SendError("agent does not have an active USD account")
            return
        }
        
        h.logger.Info("withdrawal to agent",
            zap.Int64("user_id", userIDInt),
            zap.String("agent_id", agent.AgentExternalId),
            zap.String("agent_account", agentAccount.AccountNumber))
    } else {
        // ===== PARTNER FLOW =====
        isAgentFlow = false
    }

    // ✅ Step 4: Get user USD account
    userAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", "USD")
    if err != nil {
        h.logger.Error("failed to get user account",
            zap.String("user_id", client.UserID),
            zap.Error(err))
        client.SendError("failed to get user account: " + err.Error())
        return
    }

    // ✅ Step 5: Check user balance (in USD)
    balanceResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
        Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
            AccountNumber: userAccount,
        },
    })
    if err != nil {
        client.SendError("failed to check balance: " + err.Error())
        return
    }

    if balanceResp.Balance.AvailableBalance < amountInUSD {
        client.SendError(fmt.Sprintf("insufficient balance: need %.2f USD (%.2f %s), have %.2f USD", 
            amountInUSD, req.Amount, req.LocalCurrency, balanceResp.Balance.AvailableBalance))
        return
    }

    // ✅ Step 6: Create withdrawal request
    withdrawalReq := &domain.WithdrawalRequest{
        UserID:          userIDInt,
        RequestRef:      fmt.Sprintf("WD-%d-%s", userIDInt, generateID()),
        Amount:          amountInUSD,         // Store USD amount
        Currency:        "USD",               // Store USD currency
        Destination:     req.Destination,
        Service:         req.Service,
        AgentExternalID: req.AgentID,
        PartnerID:       &selectedPartner.Id,
        Status:          domain.WithdrawalStatusPending,
        CreatedAt:       time.Now(),
        UpdatedAt:       time.Now(),
    }

    // Store original amount and rate in metadata
    withdrawalReq.SetOriginalAmount(req.Amount, req.LocalCurrency, exchangeRate)

    // Save to database
    if err := h.userUc.CreateWithdrawalRequest(ctx, withdrawalReq); err != nil {
        h.logger.Error("failed to create withdrawal request",
            zap.Int64("user_id", userIDInt),
            zap.Error(err))
        client.SendError("failed to create withdrawal request:  " + err.Error())
        return
    }

    h.logger.Info("withdrawal request created",
        zap.String("request_ref", withdrawalReq.RequestRef),
        zap.Int64("user_id", userIDInt),
        zap.Float64("amount_usd", amountInUSD),
        zap.Float64("amount_local", req.Amount),
        zap.String("currency_local", req.LocalCurrency))

    // Step 7: Process based on flow
    if isAgentFlow && agent != nil {
        // Transfer to agent account
        go h.processWithdrawalToAgent(withdrawalReq, userAccount, agentAccount.AccountNumber, agent, req.Amount, req.LocalCurrency)
        
        client.SendSuccess("withdrawal request created and being processed", map[string]interface{}{
            "request_ref":        withdrawalReq.RequestRef,
            "amount_usd":        amountInUSD,
            "amount_local":      req.Amount,
            "local_currency":    req.LocalCurrency,
            "exchange_rate":     exchangeRate,
            "destination":       req.Destination,
            "withdrawal_type":   "agent_assisted",
            "agent_id":           agent.AgentExternalId,
            "agent_name":        agent.Name,
            "status":            "processing",
        })
    } else if req.Service != nil && *req.Service != "" {
    //  NEW: Partner withdrawal (send to partner for payout)
		go h.processWithdrawalViaPartner(withdrawalReq, userAccount, selectedPartner, req.Amount, req.LocalCurrency)
		
		client.SendSuccess("withdrawal request created and being processed", map[string]interface{}{
			"request_ref":       withdrawalReq.RequestRef,
			"amount_usd":       amountInUSD,
			"amount_local":     req. Amount,
			"local_currency":   req.LocalCurrency,
			"exchange_rate":    exchangeRate,
			"destination":      req.Destination,
			"withdrawal_type":  "partner",
			"partner_id":       selectedPartner.Id,
			"partner_name":     selectedPartner.Name,
			"status":           "processing",
		})
	} else {
        // Normal withdrawal (debit only)
        go h.processWithdrawal(withdrawalReq, userAccount, req.Amount, req.LocalCurrency)
        
        client.SendSuccess("withdrawal request created and being processed", map[string]interface{}{
            "request_ref":      withdrawalReq.RequestRef,
            "amount_usd":        amountInUSD,
            "amount_local":     req.Amount,
            "local_currency":   req.LocalCurrency,
            "exchange_rate":     exchangeRate,
            "destination":      req.Destination,
            "withdrawal_type":  "direct",
            "status":           "processing",
        })
    }
}

// ✅ UPDATED: processWithdrawalToAgent with original amount tracking
func (h *PaymentHandler) processWithdrawalToAgent(
    withdrawal *domain.WithdrawalRequest,
    userAccount string,
    agentAccount string,
    agent *accountingpb.Agent,
    originalAmount float64,
    originalCurrency string,
) {
    ctx := context.Background()

    if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
        log.Printf("[Withdrawal] Failed to mark as processing: %v", err)
        return
    }

    // Execute transfer (in USD)
    transferReq := &accountingpb.TransferRequest{
        FromAccountNumber:    userAccount,
        ToAccountNumber:     agentAccount,
        Amount:              withdrawal.Amount, // USD amount
        AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
        Description:         fmt.Sprintf("Withdrawal %.2f %s to %s via agent %s", 
            originalAmount, originalCurrency, withdrawal.Destination, *agent.Name),
        ExternalRef:         &withdrawal.RequestRef,
        CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
        CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
        AgentExternalId:     &agent.AgentExternalId,
		TransactionType: accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
    }

    resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
    if err != nil {
        errMsg := err.Error()
        h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

        h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error":  "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
        return
    }

    if err := h.userUc.CompleteWithdrawal(ctx, withdrawal.RequestRef, resp.ReceiptCode, resp.JournalId); err != nil {
        log.Printf("[Withdrawal] Failed to mark as completed: %v", err)
        return
    }

    h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "agent_id": "%s",
            "agent_name": "%s",
            "amount_usd":  %.2f,
            "amount_local": %.2f,
            "local_currency": "%s",
            "fee_amount": %.2f,
            "agent_commission": %.2f
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, agent.AgentExternalId, *agent.Name, 
        withdrawal.Amount, originalAmount, originalCurrency, resp.FeeAmount, resp.AgentCommission)))
}

// ✅ UPDATED: processWithdrawal with original amount tracking
func (h *PaymentHandler) processWithdrawal(
    withdrawal *domain.WithdrawalRequest,
    userAccount string,
    originalAmount float64,
    originalCurrency string,
) {
    ctx := context.Background()

    if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
        log.Printf("[Withdrawal] Failed to mark as processing: %v", err)
        return
    }

    // Execute debit (in USD)
    debitReq := &accountingpb.DebitRequest{
        AccountNumber:        userAccount,
        Amount:              withdrawal.Amount, // USD amount
        //Currency:            "USD",
        AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
        Description:         fmt.Sprintf("Withdrawal %.2f %s to %s", 
            originalAmount, originalCurrency, withdrawal.Destination),
        ExternalRef:          &withdrawal.RequestRef,
        CreatedByExternalId:  fmt.Sprintf("%d", withdrawal.UserID),
        CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
    }

    if withdrawal.Service != nil {
        debitReq.Description = fmt.Sprintf("Withdrawal %.2f %s to %s via %s", 
            originalAmount, originalCurrency, withdrawal.Destination, *withdrawal.Service)
    }

    resp, err := h.accountingClient.Client.Debit(ctx, debitReq)
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

    if err := h.userUc.CompleteWithdrawal(ctx, withdrawal.RequestRef, resp.ReceiptCode, resp.JournalId); err != nil {
        log.Printf("[Withdrawal] Failed to mark as completed: %v", err)
        return
    }

    h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_completed",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "amount_usd": %.2f,
            "amount_local": %.2f,
            "local_currency": "%s",
            "balance_after": %.2f
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, withdrawal.Amount, originalAmount, originalCurrency, resp.BalanceAfter)))
}

// processWithdrawalViaPartner - Debit user and send to partner
// CORRECTED: processWithdrawalViaPartner - Transfer from user to partner account
func (h *PaymentHandler) processWithdrawalViaPartner(
    withdrawal *domain.WithdrawalRequest,
    userAccount string,
    partner *partnersvcpb.Partner,
    originalAmount float64,
    originalCurrency string,
) {
    ctx := context.Background()

    if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
        log.Printf("[Withdrawal] Failed to mark as processing:  %v", err)
        return
    }

    // ✅ Step 1: Get partner's account
    partnerAccount, err := h.GetAccountByCurrency(ctx, partner.Id, "partner", "USD")
    if err != nil {
        errMsg := fmt.Sprintf("partner account not found:  %v", err)
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

    // ✅ Step 2: Transfer from user account to partner account (in USD)
    transferReq := &accountingpb.TransferRequest{
        FromAccountNumber:    userAccount,
        ToAccountNumber:     partnerAccount,
        Amount:              withdrawal.Amount, // USD amount
        AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
        Description:         fmt.Sprintf("Withdrawal %.2f %s to %s via partner %s", 
            originalAmount, originalCurrency, withdrawal.Destination, partner.Name),
        ExternalRef:         &withdrawal.RequestRef,
        CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
        CreatedByType:        accountingpb.OwnerType_OWNER_TYPE_USER,
		TransactionType: accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
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

    // ✅ Step 3: Send withdrawal request to partner (for actual payout to user)
    partnerResp, err := h.partnerClient.Client.InitiateWithdrawal(ctx, &partnersvcpb.InitiateWithdrawalRequest{
        PartnerId:      partner.Id,
        TransactionRef: withdrawal.RequestRef,
        UserId:         fmt.Sprintf("%d", withdrawal.UserID),
        Amount:         originalAmount,      // Send local currency amount
        Currency:       originalCurrency,    // Send local currency
        PaymentMethod:  getPaymentMethod(withdrawal.Service),
        ExternalRef:    withdrawal.Destination,
        Metadata: map[string]string{
            "request_ref":       withdrawal.RequestRef,
            "amount_usd":        fmt.Sprintf("%.2f", withdrawal.Amount),
            "exchange_rate":     getExchangeRate(withdrawal.Metadata),
            "receipt_code":      resp.ReceiptCode,
            "journal_id":        fmt.Sprintf("%d", resp.JournalId),
        },
    })

    if err != nil {
        log.Printf("[Withdrawal] Failed to send to partner %s: %v", partner.Id, err)
        
        // ⚠️ Money already transferred to partner, so we mark as "sent_to_partner"
        // Partner should still process it or handle refund via their system
        h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "sent_to_partner", nil)
        
        // Store the receipt in case partner processes it later
        if withdrawal.Metadata == nil {
            withdrawal.Metadata = make(map[string]interface{})
        }
        withdrawal.Metadata["receipt_code"] = resp.ReceiptCode
        withdrawal.Metadata["journal_id"] = resp.JournalId
        withdrawal.Metadata["partner_notification_failed"] = true
        withdrawal.Metadata["partner_notification_error"] = err.Error()
        
        h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_processing",
            "data": {
                "request_ref": "%s",
                "receipt_code": "%s",
                "message": "Withdrawal sent to partner for processing"
            }
        }`, withdrawal.RequestRef, resp.ReceiptCode)))
        return
    }

    // ✅ Step 4: Mark as sent to partner (will be completed via webhook)
    h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "sent_to_partner", nil)

    // Store partner transaction reference
    if withdrawal.Metadata == nil {
        withdrawal.Metadata = make(map[string]interface{})
    }
    withdrawal.Metadata["partner_id"] = partner.Id
    withdrawal.Metadata["partner_name"] = partner.Name
    withdrawal.Metadata["partner_transaction_id"] = partnerResp.TransactionId
    withdrawal.Metadata["partner_transaction_ref"] = partnerResp.TransactionRef
    withdrawal.Metadata["receipt_code"] = resp.ReceiptCode
    withdrawal.Metadata["journal_id"] = resp.JournalId
    withdrawal.Metadata["sent_to_partner_at"] = time.Now()

    // ✅ Notify user
    h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_sent_to_partner",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "partner_id":  "%s",
            "partner_name": "%s",
            "partner_transaction_id": %d,
            "partner_transaction_ref": "%s",
            "amount_usd":  %.2f,
            "amount_local": %.2f,
            "local_currency":  "%s",
            "fee_amount": %.2f,
            "status": "sent_to_partner"
        }
    }`, withdrawal.RequestRef, resp.ReceiptCode, partner.Id, partner.Name,
        partnerResp.TransactionId, partnerResp.TransactionRef,
        withdrawal.Amount, originalAmount, originalCurrency, resp.FeeAmount)))
}

// ✅ Helper functions
func getPaymentMethod(service *string) string {
    if service != nil {
        return *service
    }
    return "default"
}

func getExchangeRate(metadata map[string]interface{}) string {
    if metadata == nil {
        return "0"
    }
    if rate, ok := metadata["exchange_rate"].(float64); ok {
        return fmt.Sprintf("%.4f", rate)
    }
    return "0"
}
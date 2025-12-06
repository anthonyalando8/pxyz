package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"cashier-service/internal/domain"
	usecase "cashier-service/internal/usecase/transaction"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	accountingpb "x/shared/genproto/shared/accounting/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *PaymentHandler) HandleWSMessage(client *Client, msg *WSMessage) {
	ctx := context.Background()

	switch msg.Type {
	// ========== Partner Operations ==========
	case "get_partners":
		h.handleGetPartners(ctx, client, msg.Data)

	// ========== Account Operations ==========
	case "get_accounts":
		h.handleGetAccounts(ctx, client)

	case "get_account_balance":
		h.handleGetAccountBalance(ctx, client, msg.Data)

	case "get_owner_summary":
		h.handleGetOwnerSummary(ctx, client)

	// ========== Deposit/Withdrawal Operations ==========
	case "deposit_request":
		h.handleDepositRequest(ctx, client, msg.Data)

	case "withdraw_request":
		h.handleWithdrawRequest(ctx, client, msg.Data)

	case "get_deposit_status":
		h.handleGetDepositStatus(ctx, client, msg.Data)

	case "cancel_deposit":
		h.handleCancelDeposit(ctx, client, msg.Data)

	// ========== Transaction History ==========
	case "get_history":
		h.handleGetHistory(ctx, client, msg.Data)

	case "get_transaction_by_receipt":
		h.handleGetTransactionByReceipt(ctx, client, msg.Data)

	// ========== Statements & Reports ==========
	case "get_account_statement":
		h.handleGetAccountStatement(ctx, client, msg.Data)

	case "get_owner_statement":
		h.handleGetOwnerStatement(ctx, client, msg.Data)

	case "get_ledgers":
		h.handleGetLedgers(ctx, client, msg.Data)

	// ========== P2P Transfer ==========
	case "transfer":
		h.handleTransfer(ctx, client, msg.Data)

	// ========== Fee Calculation ==========
	case "calculate_fee":
		h.handleCalculateFee(ctx, client, msg.Data)

	case "convert_and_transfer":
		h.handleConvertAndTransfer(ctx, client, msg.Data)

	default:
		client.SendError(fmt.Sprintf("unknown message type: %s", msg.Type))
	}
}

// ============================================================================
// PARTNER OPERATIONS
// ============================================================================

// Get partners by service
func (h *PaymentHandler) handleGetPartners(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		Service string `json:"service"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	partners, err := h.GetPartnersByService(ctx, req.Service)
	if err != nil {
		client.SendError("failed to fetch partners: " + err.Error())
		return
	}

	client.SendSuccess("partners retrieved", map[string]interface{}{
		"partners": partners,
		"count":    len(partners),
	})
}

// ============================================================================
// ACCOUNT OPERATIONS
// ============================================================================

// Get user accounts
func (h *PaymentHandler) handleGetAccounts(ctx context.Context, client *Client) {
	accountsResp, err := h.GetAccounts(ctx, client.UserID, "user")
	if err != nil {
		client.SendError("failed to fetch accounts: " + err.Error())
		return
	}

	// Format response with balances
	var formattedAccounts []map[string]interface{}
	for _, account := range accountsResp.Accounts {
		formattedAccounts = append(formattedAccounts, map[string]interface{}{
			"id":             account.Id,
			"account_number": account.AccountNumber,
			"currency":       account.Currency,
			"purpose":        account.Purpose.String(),
			"account_type":   account.AccountType.String(),
			"is_active":      account.IsActive,
			"is_locked":      account.IsLocked,
			"created_at":     account.CreatedAt.AsTime(),
		})
	}

	client.SendSuccess("accounts retrieved", map[string]interface{}{
		"accounts": formattedAccounts,
		"count":    len(formattedAccounts),
	})
}

// Get account balance
func (h *PaymentHandler) handleGetAccountBalance(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		AccountNumber string `json:"account_number"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// Verify account ownership
	if err := h.ValidateAccountOwnership(ctx, req.AccountNumber, client.UserID, "user"); err != nil {
		client.SendError("unauthorized: " + err.Error())
		return
	}

	balance, err := h.GetAccountBalance(ctx, req.AccountNumber)
	if err != nil {
		client.SendError("failed to get balance: " + err.Error())
		return
	}

	// The accounting service now returns decimal amounts (float64). Use them directly.
	client.SendSuccess("balance retrieved", map[string]interface{}{
		"account_number":    balance.AccountNumber,
		"balance":           balance.Balance,
		"available_balance": balance.AvailableBalance,
		"pending_debit":     balance.PendingDebit,
		"pending_credit":    balance.PendingCredit,
		"currency":          balance.Currency,
		"last_transaction":  balance.LastTransactionAt.AsTime(),
	})
}

// Get owner summary (all accounts consolidated)
func (h *PaymentHandler) handleGetOwnerSummary(ctx context.Context, client *Client) {
	req := &accountingpb.GetOwnerSummaryRequest{
		OwnerType:   accountingpb.OwnerType_OWNER_TYPE_USER,
		OwnerId:     client.UserID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
	}

	resp, err := h.accountingClient.Client.GetOwnerSummary(ctx, req)
	if err != nil {
		client.SendError("failed to get summary: " + err.Error())
		return
	}

	// Format account balances
	var balances []map[string]interface{}
	for _, bal := range resp.Summary.AccountBalances {
		balances = append(balances, map[string]interface{}{
			"account_number":    bal.AccountNumber,
			"currency":          bal.Currency,
			"balance":           bal.Balance,
			"available_balance": bal.AvailableBalance,
		})
	}

	client.SendSuccess("owner summary", map[string]interface{}{
		"account_balances":  balances,
		"total_balance_usd": resp.Summary.TotalBalanceUsdEquivalent,
		"total_accounts":    len(balances),
	})
}

// ============================================================================
// DEPOSIT OPERATIONS
// ============================================================================

// Handle deposit request
func (h *PaymentHandler) handleDepositRequest(ctx context.Context, client *Client, data json.RawMessage) {
    var req struct {
        Amount        float64 `json:"amount"`
        Currency      string  `json:"currency"`
        Service       string  `json:"service"`
        PartnerID     *string `json:"partner_id,omitempty"`
        PaymentMethod *string `json:"payment_method,omitempty"`
    }

    if err := json.Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

    // Select partner (your existing logic)
    var selectedPartner *partnersvcpb.Partner
    if req. PartnerID != nil && *req.PartnerID != "" {
        if err := h.ValidatePartnerService(ctx, *req.PartnerID, req.Service); err != nil {
            client.SendError(err. Error())
            return
        }
        var err error
        selectedPartner, err = h.GetPartnerByID(ctx, *req. PartnerID)
        if err != nil {
            client.SendError("partner not found")
            return
        }
    } else {
        partners, err := h.GetPartnersByService(ctx, req.Service)
        if err != nil || len(partners) == 0 {
            client.SendError("no partners available for this service")
            return
        }
        selectedPartner = SelectRandomPartner(partners)
    }

    // Use the new usecase method
    depositReq, err := h.userUc.InitiateDeposit(
        ctx,
        userIDInt,
        selectedPartner.Id,
        req.Amount,
        req.Currency,
        req.Service,
        req.PaymentMethod,
        30, // expiration minutes
    )
    if err != nil {
        client.SendError("failed to create deposit request: " + err.Error())
        return
    }

    // Send webhook to partner
    go h.sendDepositWebhookToPartner(depositReq, selectedPartner)

    // Update status
    h.userUc. MarkDepositSentToPartner(ctx, depositReq.RequestRef, "")

    client.SendSuccess("deposit request created", map[string]interface{}{
        "request_ref":  depositReq.RequestRef,
        "partner_id":   selectedPartner.Id,
        "partner_name": selectedPartner.Name,
        "amount":       req.Amount,
        "currency":     req.Currency,
        "status":       "sent_to_partner",
        "expires_at":   depositReq. ExpiresAt,
    })
}

// Get deposit status
func (h *PaymentHandler) handleGetDepositStatus(ctx context.Context, client *Client, data json.RawMessage) {
    var req struct {
        RequestRef string `json:"request_ref"`
    }

    if err := json.Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)
    
    deposit, err := h.userUc.GetDepositDetails(ctx, req.RequestRef, userIDInt)
    if err != nil {
        if err == usecase.ErrUnauthorized {
            client.SendError("unauthorized")
        } else {
            client.SendError("deposit not found")
        }
        return
    }

    client. SendSuccess("deposit status", deposit)
}

// Cancel deposit
func (h *PaymentHandler) handleCancelDeposit(ctx context.Context, client *Client, data json.RawMessage) {
    var req struct {
        RequestRef string `json:"request_ref"`
    }

    if err := json.Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

    if err := h.userUc. CancelDeposit(ctx, req.RequestRef, userIDInt); err != nil {
        client.SendError(err.Error())
        return
    }

    client. SendSuccess("deposit cancelled", nil)
}

// ============================================================================
// WITHDRAWAL OPERATIONS
// ============================================================================

// Handle withdrawal request
func (h *PaymentHandler) handleWithdrawRequest(ctx context.Context, client *Client, data json. RawMessage) {
    var req struct {
        Amount        float64 `json:"amount"`
        Currency      string  `json:"currency"`
        Destination   string  `json:"destination"`
        Service       *string `json:"service,omitempty"`
        AgentID       *string `json:"agent_id,omitempty"`
    }

    if err := json. Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    // Validation
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

    userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

    // ✅ Validate and fetch agent if provided
    var agent *accountingpb.Agent
    var agentAccount *accountingpb.Account
    
    if req.AgentID != nil && *req.AgentID != "" {
        // Fetch agent with accounts
        agentResp, err := h.accountingClient.Client.GetAgentByID(ctx, &accountingpb.GetAgentByIDRequest{
            AgentExternalId: *req.AgentID,
            IncludeAccounts: true, // ✅ Include accounts
        })
        
        if err != nil {
            client. SendError("invalid agent: " + err.Error())
            return
        }
        
        if agentResp. Agent == nil || !agentResp.Agent.IsActive {
            client.SendError("agent not found or inactive")
            return
        }
        
        agent = agentResp.Agent
        
        // ✅ Find agent's wallet/commission account for the currency
        for _, acc := range agent.Accounts {
            if acc.Currency == req. Currency && 
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
        
        log.Printf("[Withdrawal] User %d withdrawing to agent %s (%s) account %s", 
            userIDInt, 
            agent.AgentExternalId, 
            ptrToStr(agent.Name),
            agentAccount.AccountNumber)
    }

    // Get user account
    userAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.Currency)
    if err != nil {
        client.SendError("failed to get user account: " + err.Error())
        return
    }

    // Create withdrawal request
    withdrawalReq, err := h.userUc. InitiateWithdrawal(
        ctx,
        userIDInt,
        req.Amount,
        req.Currency,
        req.Destination,
        req.Service,
        req.AgentID,
    )
    if err != nil {
        client. SendError("failed to create withdrawal request: " + err.Error())
        return
    }

    // ✅ Process withdrawal (different flow for agent vs.  normal withdrawal)
    if agent != nil && agentAccount != nil {
        // Transfer to agent account
        go h.processWithdrawalToAgent(withdrawalReq, userAccount, agentAccount. AccountNumber, agent)
    } else {
        // Normal withdrawal (debit only)
        go h.processWithdrawal(withdrawalReq, userAccount)
    }

    // Build response
    response := map[string]interface{}{
        "request_ref": withdrawalReq. RequestRef,
        "amount":      req.Amount,
        "currency":    req.Currency,
        "destination": req.Destination,
        "status":      "processing",
    }
    
    if req.AgentID != nil && *req.AgentID != "" {
        response["agent_id"] = *req.AgentID
        response["agent_name"] = agent.Name
        response["agent_account"] = agentAccount.AccountNumber
    }

    client.SendSuccess("withdrawal request created", response)
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

// ============================================================================
// P2P TRANSFER
// ============================================================================

// Handle peer-to-peer transfer
func (h *PaymentHandler) handleTransfer(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		ToUserID    string  `json:"to_user_id"`
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		Description string  `json:"description"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// Validation
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

	// Get both accounts
	fromAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.Currency)
	if err != nil {
		client.SendError("failed to get your account: " + err.Error())
		return
	}

	toAccount, err := h.GetAccountByCurrency(ctx, req.ToUserID, "user", req.Currency)
	if err != nil {
		client.SendError("recipient account not found: " + err.Error())
		return
	}

	// Execute transfer via accounting service
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:   fromAccount,
		ToAccountNumber:     toAccount,
		Amount:              req.Amount, // pass decimal directly
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         req.Description,
		CreatedByExternalId: client.UserID,
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
	}

	resp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		client.SendError("transfer failed: " + err.Error())
		return
	}

	client.SendSuccess("transfer completed", map[string]interface{}{
		"receipt_code":     resp.ReceiptCode,
		"journal_id":       resp.JournalId,
		"amount":           req.Amount,
		"fee":              resp.FeeAmount,
		"agent_commission": resp.AgentCommission,
		"created_at":       resp.CreatedAt.AsTime(),
	})
}

// ============================================================================
// STATEMENTS & REPORTS
// ============================================================================

// Get account statement
func (h *PaymentHandler) handleGetAccountStatement(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		AccountNumber string    `json:"account_number"`
		From          time.Time `json:"from"`
		To            time.Time `json:"to"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// Verify ownership
	if err := h.ValidateAccountOwnership(ctx, req.AccountNumber, client.UserID, "user"); err != nil {
		client.SendError("unauthorized: " + err.Error())
		return
	}

	stmtReq := &accountingpb.GetAccountStatementRequest{
		AccountNumber: req.AccountNumber,
		AccountType:   accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		From:          timestamppb.New(req.From),
		To:            timestamppb.New(req.To),
	}

	resp, err := h.accountingClient.Client.GetAccountStatement(ctx, stmtReq)
	if err != nil {
		client.SendError("failed to get statement: " + err.Error())
		return
	}

	// Format ledgers
	var formattedLedgers []map[string]interface{}
	for _, ledger := range resp.Statement.Ledgers {
		formattedLedgers = append(formattedLedgers, map[string]interface{}{
			"id":            ledger.Id,
			"amount":        ledger.Amount,
			"type":          ledger.DrCr.String(),
			"currency":      ledger.Currency,
			"balance_after": ledger.BalanceAfter,
			"description":   ledger.Description,
			"receipt_code":  ledger.ReceiptCode,
			"created_at":    ledger.CreatedAt.AsTime(),
		})
	}

	client.SendSuccess("account statement", map[string]interface{}{
		"account_number":   resp.Statement.AccountNumber,
		"opening_balance":  resp.Statement.OpeningBalance,
		"closing_balance":  resp.Statement.ClosingBalance,
		"total_debits":     resp.Statement.TotalDebits,
		"total_credits":    resp.Statement.TotalCredits,
		"period_start":     resp.Statement.PeriodStart.AsTime(),
		"period_end":       resp.Statement.PeriodEnd.AsTime(),
		"ledgers":          formattedLedgers,
		"transaction_count": len(formattedLedgers),
	})
}

// Get owner statement (all accounts)
func (h *PaymentHandler) handleGetOwnerStatement(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	stmtReq := &accountingpb.GetOwnerStatementRequest{
		OwnerType:   accountingpb.OwnerType_OWNER_TYPE_USER,
		OwnerId:     client.UserID,
		AccountType: accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		From:        timestamppb.New(req.From),
		To:          timestamppb.New(req.To),
	}

	resp, err := h.accountingClient.Client.GetOwnerStatement(ctx, stmtReq)
	if err != nil {
		client.SendError("failed to get statement: " + err.Error())
		return
	}

	// Format statements
	var statements []map[string]interface{}
	for _, stmt := range resp.Statements {
		var ledgers []map[string]interface{}
		for _, ledger := range stmt.Ledgers {
			ledgers = append(ledgers, map[string]interface{}{
				"amount":        ledger.Amount,
				"type":          ledger.DrCr.String(),
				"balance_after": ledger.BalanceAfter,
				"description":   ledger.Description,
				"created_at":    ledger.CreatedAt.AsTime(),
			})
		}

		statements = append(statements, map[string]interface{}{
			"account_number":  stmt.AccountNumber,
			"opening_balance": stmt.OpeningBalance,
			"closing_balance": stmt.ClosingBalance,
			"total_debits":    stmt.TotalDebits,
			"total_credits":   stmt.TotalCredits,
			"ledgers":         ledgers,
		})
	}

	client.SendSuccess("owner statement", map[string]interface{}{
		"statements":   statements,
		"count":        len(statements),
		"period_start": req.From,
		"period_end":   req.To,
	})
}

// Get ledgers for account
func (h *PaymentHandler) handleGetLedgers(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		AccountNumber string     `json:"account_number"`
		From          *time.Time `json:"from,omitempty"`
		To            *time.Time `json:"to,omitempty"`
		Limit         int32      `json:"limit"`
		Offset        int32      `json:"offset"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	if req.Limit == 0 {
		req.Limit = 50
	}

	// Verify ownership
	if err := h.ValidateAccountOwnership(ctx, req.AccountNumber, client.UserID, "user"); err != nil {
		client.SendError("unauthorized: " + err.Error())
		return
	}

	ledgerReq := &accountingpb.ListLedgersByAccountRequest{
		AccountNumber: req.AccountNumber,
		AccountType:   accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Limit:         req.Limit,
		Offset:        req.Offset,
	}

	if req.From != nil {
		ledgerReq.From = timestamppb.New(*req.From)
	}
	if req.To != nil {
		ledgerReq.To = timestamppb.New(*req.To)
	}

	resp, err := h.accountingClient.Client.ListLedgersByAccount(ctx, ledgerReq)
	if err != nil {
		client.SendError("failed to get ledgers: " + err.Error())
		return
	}

	// Format ledgers
	var ledgers []map[string]interface{}
	for _, ledger := range resp.Ledgers {
		ledgers = append(ledgers, map[string]interface{}{
			"id":            ledger.Id,
			"journal_id":    ledger.JournalId,
			"amount":        ledger.Amount,
			"type":          ledger.DrCr.String(),
			"currency":      ledger.Currency,
			"balance_after": ledger.BalanceAfter,
			"description":   ledger.Description,
			"receipt_code":  ledger.ReceiptCode,
			"created_at":    ledger.CreatedAt.AsTime(),
		})
	}

	client.SendSuccess("ledgers retrieved", map[string]interface{}{
		"ledgers": ledgers,
		"total":   resp.Total,
		"limit":   req.Limit,
		"offset":  req.Offset,
	})
}

// ============================================================================
// TRANSACTION QUERIES
// ============================================================================

// Get transaction by receipt
func (h *PaymentHandler) handleGetTransactionByReceipt(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		ReceiptCode string `json:"receipt_code"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	txReq := &accountingpb.GetTransactionByReceiptRequest{
		ReceiptCode: req.ReceiptCode,
	}

	resp, err := h.accountingClient.Client.GetTransactionByReceipt(ctx, txReq)
	if err != nil {
		client.SendError("transaction not found: " + err.Error())
		return
	}

	// Verify user is involved in this transaction
	isInvolved := false
	for _, ledger := range resp.Ledgers {
		// Check if any ledger belongs to user's account
		accResp, err := h.accountingClient.Client.GetAccount(ctx, &accountingpb.GetAccountRequest{
			Identifier: &accountingpb.GetAccountRequest_AccountNumber{
				AccountNumber: ledger.AccountNumber,
			},
		})
		if err == nil && accResp.Account.OwnerId == client.UserID {
			isInvolved = true
			break
		}
	}

	if !isInvolved {
		client.SendError("unauthorized to view this transaction")
		return
	}

	// Format response
	var ledgers []map[string]interface{}
	for _, ledger := range resp.Ledgers {
		ledgers = append(ledgers, map[string]interface{}{
			"account_number": ledger.AccountNumber,
			"amount":         ledger.Amount,
			"type":           ledger.DrCr.String(),
			"balance_after":  ledger.BalanceAfter,
			"description":    ledger.Description,
		})
	}

	var fees []map[string]interface{}
	for _, fee := range resp.Fees {
		fees = append(fees, map[string]interface{}{
			"type":     fee.FeeType.String(),
			"amount":   fee.Amount,
			"currency": fee.Currency,
		})
	}

	client.SendSuccess("transaction details", map[string]interface{}{
		"journal": map[string]interface{}{
			"id":               resp.Journal.Id,
			"transaction_type": resp.Journal.TransactionType.String(),
			"description":      resp.Journal.Description,
			"created_at":       resp.Journal.CreatedAt.AsTime(),
		},
		"ledgers": ledgers,
		"fees":    fees,
	})
}

// Get transaction history (deposits + withdrawals)
func (h *PaymentHandler) handleGetHistory(ctx context.Context, client *Client, data json.RawMessage) {
    var req struct {
        Type   string `json:"type"` // deposits, withdrawals, all
        Limit  int    `json:"limit"`
        Offset int    `json:"offset"`
    }

    if err := json.Unmarshal(data, &req); err != nil {
        client.SendError("invalid request format")
        return
    }

    if req. Limit == 0 {
        req.Limit = 20
    }

    userIDInt, _ := strconv. ParseInt(client.UserID, 10, 64)

    var deposits []domain.DepositRequest
    var withdrawals []domain.WithdrawalRequest
    var err error

    switch req.Type {
    case "deposits":
        deposits, _, err = h.userUc. GetUserDepositHistory(ctx, userIDInt, req.Limit, req.Offset)
    case "withdrawals":
        withdrawals, _, err = h.userUc.GetUserWithdrawalHistory(ctx, userIDInt, req.Limit, req.Offset)
    case "all":
        deposits, _, _ = h.userUc.GetUserDepositHistory(ctx, userIDInt, req.Limit/2, req.Offset)
        withdrawals, _, _ = h.userUc.GetUserWithdrawalHistory(ctx, userIDInt, req.Limit/2, req.Offset)
    default:
        client. SendError("invalid type: must be 'deposits', 'withdrawals', or 'all'")
        return
    }

    if err != nil {
        client.SendError("failed to fetch history: " + err.Error())
        return
    }

    client.SendSuccess("transaction history", map[string]interface{}{
        "deposits":    deposits,
        "withdrawals": withdrawals,
    })
}

// ============================================================================
// FEE CALCULATION
// ============================================================================

// Calculate fee for a transaction
func (h *PaymentHandler) handleCalculateFee(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		TransactionType string  `json:"transaction_type"` // transfer, withdrawal, conversion
		Amount          float64 `json:"amount"`
		SourceCurrency  string  `json:"source_currency,omitempty"`
		TargetCurrency  string  `json:"target_currency,omitempty"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	accountType := accountingpb.AccountType_ACCOUNT_TYPE_REAL
	ownerType := accountingpb.OwnerType_OWNER_TYPE_USER
	feeReq := &accountingpb.CalculateFeeRequest{
		TransactionType: mapTransactionType(req.TransactionType),
		Amount:          req.Amount, // pass decimal directly
		AccountType:     &accountType,
		OwnerType:       &ownerType,
	}

	if req.SourceCurrency != "" {
		feeReq.SourceCurrency = &req.SourceCurrency
	}
	if req.TargetCurrency != "" {
		feeReq.TargetCurrency = &req.TargetCurrency
	}

	resp, err := h.accountingClient.Client.CalculateFee(ctx, feeReq)
	if err != nil {
		client.SendError("failed to calculate fee: " + err.Error())
		return
	}

	client.SendSuccess("fee calculated", map[string]interface{}{
		"fee_type":     resp.Calculation.FeeType.String(),
		"amount":       resp.Calculation.Amount,
		"currency":     resp.Calculation.Currency,
		"applied_rate": resp.Calculation.AppliedRate,
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func mapTransactionType(s string) accountingpb.TransactionType {
	switch s {
	case "deposit":
		return accountingpb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case "withdrawal":
		return accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case "transfer":
		return accountingpb.TransactionType_TRANSACTION_TYPE_TRANSFER
	case "conversion":
		return accountingpb.TransactionType_TRANSACTION_TYPE_CONVERSION
	default:
		return accountingpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

// Helper: Send webhook to partner
func (h *PaymentHandler) sendDepositWebhookToPartner(deposit *domain.DepositRequest, partner *partnersvcpb.Partner) {
	ctx := context.Background()

	_, err := h.partnerClient. Client.InitiateDeposit(ctx, &partnersvcpb.InitiateDepositRequest{
		PartnerId:      partner.Id,
		TransactionRef: deposit.RequestRef,
		UserId:         fmt. Sprintf("%d", deposit.UserID),
		Amount:         deposit. Amount,
		Currency:       deposit.Currency,
		PaymentMethod:  ptrToStr(deposit.PaymentMethod),
		Metadata: map[string]string{
			"request_ref": deposit.RequestRef,
		},
	})

	if err != nil {
		log.Printf("[Deposit] Failed to send webhook to partner %s: %v", partner.Id, err)
		// ❌ OLD: h.userUc.UpdateDepositStatus(ctx, deposit.ID, "failed", strToPtr(err.Error()))
		// ✅ NEW: Use the business method
		h.userUc. FailDeposit(ctx, deposit. RequestRef, err.Error())
	}
}
// ============================================================================
// CURRENCY CONVERSION & TRANSFER
// ============================================================================

// Handle currency conversion and transfer
func (h *PaymentHandler) handleConvertAndTransfer(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		FromCurrency string  `json:"from_currency"`
		ToCurrency   string  `json:"to_currency"`
		Amount       float64 `json:"amount"`
		Description  string  `json:"description,omitempty"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// Validation
	if req.Amount <= 0 {
		client.SendError("amount must be greater than zero")
		return
	}
	if req.FromCurrency == "" {
		client.SendError("from_currency is required")
		return
	}
	if req.ToCurrency == "" {
		client.SendError("to_currency is required")
		return
	}
	if req.FromCurrency == req.ToCurrency {
		client.SendError("from_currency and to_currency must be different")
		return
	}

	// Get user's source account
	fromAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.FromCurrency)
	if err != nil {
		client.SendError("source account not found: " + err.Error())
		return
	}

	// Get user's destination account
toAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.ToCurrency)
	if err != nil {
		client.SendError("destination account not found: " + err.Error())
		return
	}

	// Verify both accounts belong to the user (extra safety check)
	if err := h.ValidateAccountOwnership(ctx, fromAccount, client.UserID, "user"); err != nil {
		client.SendError("unauthorized: source account doesn't belong to you")
		return
	}
	if err := h.ValidateAccountOwnership(ctx, toAccount, client.UserID, "user"); err != nil {
		client.SendError("unauthorized: destination account doesn't belong to you")
		return
	}

	// Execute conversion via accounting service
	conversionReq := &accountingpb.ConversionRequest{
		FromAccountNumber:   fromAccount,
		ToAccountNumber:     toAccount,
		Amount:              req.Amount, // decimal
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		CreatedByExternalId: client.UserID,
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
	}

	// Add description if provided
	if req.Description != "" {
		conversionReq.ExternalRef = &req.Description
	}

	resp, err := h.accountingClient.Client.ConvertAndTransfer(ctx, conversionReq)
	if err != nil {
		client.SendError("conversion failed: " + err.Error())
		return
	}

	// Send success response with conversion details
	client.SendSuccess("conversion completed", map[string]interface{}{
		"receipt_code":      resp.ReceiptCode,
		"journal_id":        resp.JournalId,
		"source_currency":   resp.SourceCurrency,
		"dest_currency":     resp.DestCurrency,
		"source_amount":     resp.SourceAmount,
		"converted_amount":  resp.ConvertedAmount,
		"fx_rate":           resp.FxRate,
		"fx_rate_id":        resp.FxRateId,
		"fee":               resp.FeeAmount,
		"created_at":        resp.CreatedAt.AsTime(),
	})

	// Log the conversion for audit
	log.Printf("[Conversion] User %s: %.2f %s -> %.2f %s (Rate: %s, Fee: %.2f)",
		client.UserID,
		resp.SourceAmount,
		resp.SourceCurrency,
		resp.ConvertedAmount,
		resp.DestCurrency,
		resp.FxRate,
		resp.FeeAmount,
	)
}
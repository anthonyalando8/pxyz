package handler

import (
    "context"
    "encoding/json"
    "time"
    
    accountingpb "x/shared/genproto/shared/accounting/v1"
    "google.golang.org/protobuf/types/known/timestamppb"
)

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




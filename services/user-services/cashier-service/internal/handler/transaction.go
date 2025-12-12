package handler

import (
	"cashier-service/internal/domain"
	"context"
	"encoding/json"
	"strconv"

	accountingpb "x/shared/genproto/shared/accounting/v1"
)

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



package handler

import (
    "context"
    "encoding/json"
    
    accountingpb "x/shared/genproto/shared/accounting/v1"
)



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


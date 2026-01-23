package handler

import (
	"context"
	"encoding/json"
	"log"

	accountingpb "x/shared/genproto/shared/accounting/v1"
)

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
	fromAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.FromCurrency,nil)
	if err != nil {
		client.SendError("source account not found: " + err.Error())
		return
	}

	// Get user's destination account
toAccount, err := h.GetAccountByCurrency(ctx, client.UserID, "user", req.ToCurrency,nil)
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
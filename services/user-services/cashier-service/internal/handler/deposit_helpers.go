// handler/deposit_helpers.go
package handler

import (
	"context"
	"encoding/json"
	"strconv"

	transaction "cashier-service/internal/usecase/transaction"
)

// ============================================================================
// DEPOSIT STATUS OPERATIONS
// ============================================================================

// handleGetDepositStatus gets deposit status
func (h *PaymentHandler) handleGetDepositStatus(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		RequestRef string `json:"request_ref"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

	deposit, err := h.userUc.GetDepositDetails(ctx, req.RequestRef, userIDInt)
	if err != nil {
		if err == transaction.ErrUnauthorized {
			client.SendError("unauthorized")
		} else {
			client.SendError("deposit not found")
		}
		return
	}

	// Build response with both original and converted amounts
	response := map[string]interface{}{
		"id":                      deposit.ID,
		"request_ref":             deposit.RequestRef,
		"converted_amount":        deposit.Amount,
		"target_currency":         deposit.Currency,
		"status":                  deposit.Status,
		"service":                 deposit.Service,
		"payment_method":          deposit.PaymentMethod,
		"partner_transaction_ref": deposit.PartnerTransactionRef,
		"receipt_code":            deposit.ReceiptCode,
		"error_message":           deposit.ErrorMessage,
		"expires_at":              deposit.ExpiresAt,
		"created_at":              deposit.CreatedAt,
		"completed_at":            deposit.CompletedAt,
	}

	// Add original amount if available
	if origAmount, origCurrency, rate, ok := deposit.GetOriginalAmount(); ok {
		response["original_amount"] = origAmount
		response["original_currency"] = origCurrency
		response["exchange_rate"] = rate
	}

	// Add phone number if available
	if phoneNumber, ok := deposit.Metadata["phone_number"].(string); ok && phoneNumber != "" {
		response["phone_number"] = phoneNumber
	}

	// Add deposit type if available
	if depositType, ok := deposit.Metadata["deposit_type"].(string); ok {
		response["deposit_type"] = depositType
	}

	// Add agent info if present
	if deposit.AgentExternalID != nil {
		response["agent_id"] = *deposit.AgentExternalID
		if agentName, ok := deposit.Metadata["agent_name"]; ok {
			response["agent_name"] = agentName
		}
	}

	// Add partner info if present
	if deposit.PartnerID != "" {
		response["partner_id"] = deposit.PartnerID
		if partnerName, ok := deposit.Metadata["partner_name"].(string); ok {
			response["partner_name"] = partnerName
		}
	}

	client.SendSuccess("deposit status", response)
}

// handleCancelDeposit cancels a deposit request
func (h *PaymentHandler) handleCancelDeposit(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		RequestRef string `json:"request_ref"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	userIDInt, _ := strconv.ParseInt(client.UserID, 10, 64)

	if err := h.userUc.CancelDeposit(ctx, req.RequestRef, userIDInt); err != nil {
		client.SendError(err.Error())
		return
	}

	client.SendSuccess("deposit cancelled", map[string]interface{}{
		"request_ref": req.RequestRef,
		"status":      "cancelled",
	})
}
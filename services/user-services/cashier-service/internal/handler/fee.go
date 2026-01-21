package handler

import (
    "context"
    "encoding/json"
    
    accountingpb "x/shared/genproto/shared/accounting/v1"
)

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
		ToAddress 	 *string  `json:"to_address,omitempty"`
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
	if req.ToAddress != nil {
		feeReq.ToAddress = req.ToAddress
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
		"calculated_from": resp.Calculation.CalculatedFrom,
	})
}

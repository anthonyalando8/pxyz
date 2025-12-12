package handler

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "strconv"
    
    "cashier-service/internal/domain"
    usecase "cashier-service/internal/usecase/transaction"
    partnersvcpb "x/shared/genproto/partner/svcpb"
)

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
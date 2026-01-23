// handler/deposit_partner.go
package handler

import (
	"context"
	"fmt"
	"time"

	"cashier-service/internal/domain"
	convsvc "cashier-service/internal/service"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"x/shared/utils/id"

	"go.uber.org/zap"
)

// buildPartnerDepositContext builds context for partner deposit (M-Pesa)
func (h *PaymentHandler) buildPartnerDepositContext(ctx context.Context, dctx *DepositContext) (*DepositContext, error) {
	req := dctx.Request

	// Validate phone number for M-Pesa
	phone, _, err := h.validateUserProfile(ctx, dctx.UserID, req.Service)
	if err != nil {
		return nil, err
	}
	dctx.PhoneNumber = phone

	// Get partner
	var selectedPartner *partnersvcpb.Partner
	if req.PartnerID != nil && *req.PartnerID != "" {
		if err := h.ValidatePartnerService(ctx, *req.PartnerID, req.Service); err != nil {
			return nil, err
		}
		selectedPartner, err = h.GetPartnerByID(ctx, *req.PartnerID)
		if err != nil {
			return nil, fmt.Errorf("partner not found")
		}
	} else {
		partners, err := h.GetPartnersByService(ctx, req.Service)
		if err != nil || len(partners) == 0 {
			return nil, fmt.Errorf("no partners available for service: %s", req.Service)
		}
		selectedPartner = SelectRandomPartner(partners)
	}

	dctx.Partner = selectedPartner
	dctx.LocalCurrency = selectedPartner.LocalCurrency

	//  Convert to USD using partner's currency
	currencyService := convsvc.NewCurrencyService(h.partnerClient)
	amountInUSD, exchangeRate, err := currencyService.ConvertToUSDWithValidation(ctx, selectedPartner, req.Amount)
	if err != nil {
		h.logger.Error("currency conversion failed",
			zap.String("user_id", dctx.UserID),
			zap.Float64("amount", req.Amount),
			zap.String("currency", dctx.LocalCurrency),
			zap.Error(err))
		return nil, fmt.Errorf("currency conversion failed: %v", err)
	}

	dctx.AmountInUSD = amountInUSD
	dctx.ExchangeRate = exchangeRate

	h.logger.Info("partner deposit context built",
		zap.String("partner_id", selectedPartner.Id),
		zap.String("partner_name", selectedPartner.Name),
		zap.String("local_currency", dctx.LocalCurrency),
		zap.Float64("amount_local", req.Amount),
		zap.Float64("amount_usd", amountInUSD),
		zap.Float64("exchange_rate", exchangeRate),
		zap.String("phone", phone))

	return dctx, nil
}

// processPartnerDeposit processes partner deposit (M-Pesa)
func (h *PaymentHandler) processPartnerDeposit(ctx context.Context, client *Client, dctx *DepositContext) {
	req := dctx.Request

	// Create deposit request
	depositReq := &domain.DepositRequest{
		UserID:        dctx.UserIDInt,
		PartnerID:     dctx.Partner.Id,
		RequestRef:    id.GenerateTransactionID("DEP-PT"),
		Amount:        dctx.AmountInUSD,
		Currency:      dctx.TargetCurrency,
		Service:       req.Service,
		PaymentMethod: &req.Service,
		Status:        domain.DepositStatusPending,
		ExpiresAt:     time.Now().Add(30 * time.Minute),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Store original amount and metadata
	depositReq.SetOriginalAmount(req.Amount, dctx.LocalCurrency, dctx.ExchangeRate)
	if depositReq.Metadata == nil {
		depositReq.Metadata = make(map[string]interface{})
	}
	if dctx.PhoneNumber != "" {
		depositReq.Metadata["phone_number"] = dctx.PhoneNumber
	}
	depositReq.Metadata["deposit_type"] = "partner"
	depositReq.Metadata["partner_id"] = dctx.Partner.Id
	depositReq.Metadata["partner_name"] = dctx.Partner.Name
	depositReq.Metadata["exchange_rate"] = dctx.ExchangeRate

	// Save to database
	if err := h.userUc.CreateDepositRequest(ctx, depositReq); err != nil {
		h.logger.Error("failed to create deposit request", zap.Error(err))
		client.SendError(fmt.Sprintf("failed to create deposit request: %v", err))
		return
	}

	// Send webhook to partner asynchronously
	go h.sendDepositWebhookToPartner(depositReq, dctx)

	// Mark as sent to partner
	h.userUc.MarkDepositSentToPartner(ctx, depositReq.RequestRef, "")

	// Send success response
	client.SendSuccess("deposit request created and sent to partner", map[string]interface{}{
		"request_ref":       depositReq.RequestRef,
		"partner_id":        dctx.Partner.Id,
		"partner_name":      dctx.Partner.Name,
		"original_amount":   req.Amount,
		"original_currency": dctx.LocalCurrency,
		"converted_amount":  dctx.AmountInUSD,
		"target_currency":   dctx.TargetCurrency,
		"exchange_rate":     dctx.ExchangeRate,
		"phone_number":      dctx.PhoneNumber,
		"status":            "sent_to_partner",
		"expires_at":        depositReq.ExpiresAt,
	})
}

// sendDepositWebhookToPartner sends webhook to partner
func (h *PaymentHandler) sendDepositWebhookToPartner(depositReq *domain.DepositRequest, dctx *DepositContext) {
	ctx := context.Background()

	metadata := map[string]string{
		"request_ref":       depositReq.RequestRef,
		"original_amount":   fmt.Sprintf("%.2f", dctx.Request.Amount),
		"converted_amount":  fmt.Sprintf("%.2f", dctx.AmountInUSD),
		"original_currency": dctx.LocalCurrency,
		"target_currency":   dctx.TargetCurrency,
		"exchange_rate":     fmt.Sprintf("%.4f", dctx.ExchangeRate),
	}

	if dctx.PhoneNumber != "" {
		metadata["phone_number"] = dctx.PhoneNumber
	}

	h.logger.Info("sending deposit webhook to partner",
		zap.String("partner_id", dctx.Partner.Id),
		zap.String("request_ref", depositReq.RequestRef),
		zap.Any("metadata", metadata))

	_, err := h.partnerClient.Client.InitiateDeposit(ctx, &partnersvcpb.InitiateDepositRequest{
		PartnerId:      dctx.Partner.Id,
		TransactionRef: depositReq.RequestRef,
		UserId:         fmt.Sprintf("%d", depositReq.UserID),
		Amount:         dctx.AmountInUSD,
		Currency:       dctx.TargetCurrency,
		PaymentMethod:  dctx.Request.Service,
		Metadata:       metadata,
	})

	if err != nil {
		h.logger.Error("failed to send webhook to partner",
			zap.String("partner_id", dctx.Partner.Id),
			zap.Error(err))
		h.userUc.FailDeposit(ctx, depositReq.RequestRef, err.Error())
	} else {
		h.logger.Info("deposit webhook sent successfully",
			zap.String("partner_id", dctx.Partner.Id),
			zap.String("request_ref", depositReq.RequestRef))
	}
}
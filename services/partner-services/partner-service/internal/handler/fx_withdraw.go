package handler

import (
	//"encoding/json"
	"context"
	"fmt"

	"net/http"
	domain "partner-service/internal/domain"
	"time"

	// emailclient "x/shared/email"
	// smsclient "x/shared/sms"

	//"partner-service/internal/usecase"
	partnerMiddleware "partner-service/pkg/auth"

	"partner-service/internal/events"

	"x/shared/response"

	"go.uber.org/zap"
)
// handler/partner_handler.go

// DebitUser allows partner to process a withdrawal (via API key authentication)
// This is called by payment service after successful B2C or bank transfer
type DebitUser struct {
	UserID         string                 `json:"user_id"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	TransactionRef string                 `json:"transaction_ref"`
	Description    string                 `json:"description"`
	PaymentMethod  string                 `json:"payment_method,omitempty"`
	ExternalRef    string                 `json:"external_ref,omitempty"` // M-Pesa or bank reference
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// DebitUser processes withdrawal completion
func (h *PartnerHandler) DebitUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Get and validate partner
	partner, ok := partnerMiddleware.GetPartnerFromContext(ctx)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	h.logger.Info("withdrawal completion request received",
		zap.String("partner_id", partner.ID),
		zap.String("partner_name", partner.Name))

	// 2. Parse and validate request
	var req DebitUser
	if err := decodeJSON(r, &req); err != nil {
		response. Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validateDebitRequest(&req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// 3. Get and validate existing withdrawal transaction
	partnerTx, err := h.getAndValidateWithdrawalTransaction(ctx, partner.ID, &req)
	if err != nil {
		h.handleWithdrawalError(ctx, w, 0, err. Error(), http.StatusBadRequest)
		return
	}

	// 4. Update status to processing
	if err := h. uc.UpdateTransactionStatus(ctx, partnerTx.ID, "processing", ""); err != nil {
		h. logger.Error("failed to update withdrawal status",
			zap.Int64("transaction_id", partnerTx.ID),
			zap.Error(err))
		response.Error(w, http.StatusInternalServerError, "failed to update transaction status")
		return
	}

	// 5. Complete withdrawal transaction
	if err := h.completeWithdrawalTransaction(ctx, partnerTx. ID, req.ExternalRef); err != nil {
		h.handleWithdrawalError(ctx, w, partnerTx.ID, err.Error(), http.StatusInternalServerError)
		return
	}

	// 6. Get updated transaction
	updatedTx, err := h.uc.GetTransactionByID(ctx, partnerTx.ID)
	if err != nil {
		h.logger.Error("failed to get updated transaction",
			zap.Int64("transaction_id", partnerTx.ID),
			zap.Error(err))
	}

	// 7. Publish withdrawal completed event
	h.publishWithdrawalCompleted(ctx, partner, &req, updatedTx)

	// 8. Send webhook
	h.sendWithdrawalWebhook(ctx, partner.ID, &req, updatedTx)

	// 9. Log API activity
	h.logAPIActivity(ctx, partner.ID, "POST", "/transactions/debit", http.StatusOK, req, updatedTx)

	// 10. Return success response
	h.sendDebitResponse(w, &req, updatedTx)

	h.logger.Info("withdrawal transaction completed",
		zap.String("partner_id", partner.ID),
		zap.String("user_id", req.UserID),
		zap.Int64("transaction_id", partnerTx.ID),
		zap.Float64("amount", req.Amount),
		zap.String("external_ref", req.ExternalRef))
}

// ============================================
// WITHDRAWAL HELPER METHODS
// ============================================

// validateDebitRequest validates the withdrawal completion request
func (h *PartnerHandler) validateDebitRequest(req *DebitUser) error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.TransactionRef == "" {
		return fmt.Errorf("transaction_ref is required")
	}
	if req.ExternalRef == "" {
		return fmt.Errorf("external_ref is required (M-Pesa code or bank reference)")
	}
	if len(req.Currency) < 3 || len(req.Currency) > 8 {
		return fmt. Errorf("invalid currency format")
	}
	return nil
}

// getAndValidateWithdrawalTransaction retrieves and validates the withdrawal transaction
func (h *PartnerHandler) getAndValidateWithdrawalTransaction(ctx context.Context, partnerID string, req *DebitUser) (*domain.PartnerTransaction, error) {
	// Get transaction
	partnerTx, err := h. uc.GetTransactionByRef(ctx, partnerID, req.TransactionRef)
	if err != nil {
		h.logger.Error("withdrawal transaction not found",
			zap.String("partner_id", partnerID),
			zap.String("transaction_ref", req.TransactionRef),
			zap.Error(err))
		return nil, fmt.Errorf("transaction with ref %s not found", req.TransactionRef)
	}

	// Verify transaction type is withdrawal
	if partnerTx.TransactionType != "withdrawal" {
		h.logger. Warn("transaction is not a withdrawal",
			zap.String("partner_id", partnerID),
			zap.String("transaction_ref", req.TransactionRef),
			zap.String("transaction_type", partnerTx.TransactionType))
		return nil, fmt.Errorf("transaction is not a withdrawal")
	}

	// Verify status
	if partnerTx.Status != "sent_to_partner" && partnerTx.Status != "pending" {
		h.logger. Warn("withdrawal transaction not in valid status",
			zap.String("partner_id", partnerID),
			zap.String("transaction_ref", req.TransactionRef),
			zap.String("current_status", partnerTx. Status))
		return nil, fmt.Errorf("transaction already %s", partnerTx.Status)
	}

	// Verify amount
	if partnerTx.Amount != req.Amount {
		h.logger.Warn("withdrawal amount mismatch",
			zap. Float64("expected", partnerTx.Amount),
			zap.Float64("provided", req.Amount))
		return nil, fmt. Errorf("amount mismatch:  expected %.2f, got %.2f", partnerTx.Amount, req. Amount)
	}

	// Verify currency
	if partnerTx.Currency != req.Currency {
		h.logger. Warn("withdrawal currency mismatch",
			zap.String("expected", partnerTx.Currency),
			zap.String("provided", req.Currency))
		return nil, fmt.Errorf("currency mismatch: expected %s, got %s", partnerTx.Currency, req.Currency)
	}

	// Verify user ID
	if partnerTx.UserID != req.UserID {
		h.logger. Warn("withdrawal user_id mismatch",
			zap.String("expected", partnerTx.UserID),
			zap.String("provided", req.UserID))
		return nil, fmt.Errorf("user_id mismatch")
	}

	return partnerTx, nil
}

// completeWithdrawalTransaction updates withdrawal transaction with external reference
func (h *PartnerHandler) completeWithdrawalTransaction(ctx context.Context, txnID int64, externalRef string) error {
	return h.uc.CompleteWithdrawal(ctx, txnID, externalRef, "completed")
}

// publishWithdrawalCompleted publishes withdrawal completed event to Redis
func (h *PartnerHandler) publishWithdrawalCompleted(
	ctx context.Context,
	partner *domain.Partner,
	req *DebitUser,
	partnerTx *domain.PartnerTransaction,
) {
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		withdrawalEvent := &events.WithdrawalCompletedEvent{
			TransactionRef: req.TransactionRef,
			TransactionID:  partnerTx.ID,
			PartnerID:      partner.ID,
			UserID:         req.UserID,
			Amount:         req.Amount,
			Currency:       req. Currency,
			ExternalRef:    req.ExternalRef,
			PaymentMethod:  req.PaymentMethod,
			Metadata:       req.Metadata,
			CompletedAt:    time.Now(),
		}

		if err := h. eventPublisher.PublishWithdrawalCompleted(pubCtx, withdrawalEvent); err != nil {
			h.logger. Error("failed to publish withdrawal completed event",
				zap.String("transaction_ref", req.TransactionRef),
				zap.Error(err))
		}
	}()
}

// publishWithdrawalFailed publishes withdrawal failed event to Redis
func (h *PartnerHandler) publishWithdrawalFailed(
	ctx context.Context,
	partner *domain.Partner,
	req *DebitUser,
	partnerTx *domain.PartnerTransaction,
	errorMsg string,
) {
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		failedEvent := &events.WithdrawalFailedEvent{
			TransactionRef: req.TransactionRef,
			TransactionID:   partnerTx.ID,
			PartnerID:      partner.ID,
			UserID:         req.UserID,
			Amount:         req.Amount,
			Currency:       req.Currency,
			ErrorMessage:   errorMsg,
			FailedAt:       time.Now(),
		}

		if err := h. eventPublisher.PublishWithdrawalFailed(pubCtx, failedEvent); err != nil {
			h.logger.Error("failed to publish withdrawal failed event",
				zap.String("transaction_ref", req.TransactionRef),
				zap.Error(err))
		}
	}()
}

// sendWithdrawalWebhook sends webhook notification to partner
func (h *PartnerHandler) sendWithdrawalWebhook(
	ctx context.Context,
	partnerID string,
	req *DebitUser,
	partnerTx *domain.PartnerTransaction,
) {
	go h.uc.SendWebhook(context.Background(), partnerID, "withdrawal. completed", map[string]interface{}{
		"transaction_ref": req.TransactionRef,
		"user_id":         req.UserID,
		"amount":          req.Amount,
		"currency":         req.Currency,
		"external_ref":    req.ExternalRef,
		"payment_method":  req. PaymentMethod,
		"status":          partnerTx.Status,
		"timestamp":       time.Now().Unix(),
	})
}

// sendDebitResponse sends success response
func (h *PartnerHandler) sendDebitResponse(
	w http.ResponseWriter,
	req *DebitUser,
	partnerTx *domain. PartnerTransaction,
) {
	response. JSON(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"transaction_ref": req.TransactionRef,
		"transaction_id":  partnerTx.ID,
		"external_ref":    req.ExternalRef,
		"status":          partnerTx.Status,
		"message":         "Withdrawal completed successfully",
	})
}

// handleWithdrawalError handles errors and publishes failed events
func (h *PartnerHandler) handleWithdrawalError(ctx context.Context, w http.ResponseWriter, txnID int64, errorMsg string, statusCode int) {
	// Update transaction status if we have txnID
	if txnID > 0 {
		if err := h.uc.UpdateTransactionStatus(ctx, txnID, "failed", errorMsg); err != nil {
			h.logger.Error("failed to update withdrawal transaction status",
				zap.Int64("transaction_id", txnID),
				zap.Error(err))
		}

		// Get transaction details for event
		if partnerTx, err := h. uc.GetTransactionByID(ctx, txnID); err == nil {
			partner, _ := partnerMiddleware.GetPartnerFromContext(ctx)
			if partner != nil {
				// Publish failed event
				h. publishWithdrawalFailed(ctx, partner, &DebitUser{
					UserID:          partnerTx.UserID,
					Amount:         partnerTx.Amount,
					Currency:       partnerTx.Currency,
					TransactionRef: partnerTx.TransactionRef,
				}, partnerTx, errorMsg)
			}
		}
	}

	response.Error(w, statusCode, errorMsg)
}
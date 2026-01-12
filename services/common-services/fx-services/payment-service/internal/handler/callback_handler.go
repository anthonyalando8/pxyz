// internal/handler/callback_handler.go
package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"payment-service/internal/domain"
	"payment-service/internal/usecase"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type CallbackHandler struct {
	callbackUC *usecase.CallbackUsecase
	paymentUC  *usecase.PaymentUsecase
	logger     *zap.Logger
}

func NewCallbackHandler(callbackUC *usecase.CallbackUsecase, paymentUC *usecase.PaymentUsecase, logger *zap.Logger) *CallbackHandler {
	return &CallbackHandler{
		callbackUC:  callbackUC,
		paymentUC:  paymentUC,
		logger:     logger,
	}
}

// ============================================
// STK PUSH CALLBACKS (Deposits)
// ============================================

// HandleMpesaSTKCallback handles M-Pesa STK callback
func (h *CallbackHandler) HandleMpesaSTKCallback(w http.ResponseWriter, r *http.Request) {
	paymentRef := chi. URLParam(r, "payment_ref")

	h.logger.Info("received M-Pesa STK callback",
		zap.String("payment_ref", paymentRef),
		zap.String("remote_addr", r. RemoteAddr))

	// Read payload
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read callback payload",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
		h.sendCallbackResponse(w, "1", "Failed to read payload")
		return
	}

	h.logger.Debug("M-Pesa STK callback payload received",
		zap.String("payment_ref", paymentRef),
		zap.Int("payload_size", len(payload)))

	// Process callback asynchronously with timeout context
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := h.callbackUC.ProcessMpesaSTKCallback(ctx, paymentRef, payload); err != nil {
			h.logger.Error("failed to process M-Pesa STK callback",
				zap.String("payment_ref", paymentRef),
				zap.Error(err))
		}
	}()

	// Return success immediately (M-Pesa requires quick response)
	h.logger.Info("M-Pesa STK callback acknowledged",
		zap. String("payment_ref", paymentRef))
	h.sendCallbackResponse(w, "0", "Success")
}

// ============================================
// B2C CALLBACKS (Mobile Money Withdrawals)
// ============================================

// HandleMpesaB2CCallback handles M-Pesa B2C callback
func (h *CallbackHandler) HandleMpesaB2CCallback(w http.ResponseWriter, r *http.Request) {
	paymentRef := chi.URLParam(r, "payment_ref")

	h.logger.Info("received M-Pesa B2C callback",
		zap.String("payment_ref", paymentRef),
		zap.String("remote_addr", r.RemoteAddr))

	// Read payload
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger. Error("failed to read B2C callback payload",
			zap. String("payment_ref", paymentRef),
			zap.Error(err))
		h.sendCallbackResponse(w, "1", "Failed to read payload")
		return
	}

	h.logger.Debug("M-Pesa B2C callback payload received",
		zap.String("payment_ref", paymentRef),
		zap.Int("payload_size", len(payload)))

	// Process callback asynchronously with timeout context
	go func() {
		ctx, cancel := context. WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := h.callbackUC.ProcessMpesaB2CCallback(ctx, paymentRef, payload); err != nil {
			h.logger.Error("failed to process M-Pesa B2C callback",
				zap.String("payment_ref", paymentRef),
				zap.Error(err))
		}
	}()

	// Return success immediately
	h.logger.Info("M-Pesa B2C callback acknowledged",
		zap.String("payment_ref", paymentRef))
	h.sendCallbackResponse(w, "0", "Success")
}

// HandleMpesaB2CTimeout handles M-Pesa B2C timeout
func (h *CallbackHandler) HandleMpesaB2CTimeout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	paymentRef := chi.URLParam(r, "payment_ref")

	h.logger.Warn("received M-Pesa B2C timeout",
		zap.String("payment_ref", paymentRef),
		zap.String("remote_addr", r.RemoteAddr))

	// Read payload
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read timeout payload",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
		h.sendCallbackResponse(w, "1", "Failed to read payload")
		return
	}

	h.logger.Info("M-Pesa B2C timeout payload",
		zap.String("payment_ref", paymentRef),
		zap.String("payload", string(payload)))

	// Mark payment as failed due to timeout
	payment, err := h.paymentUC.GetByPaymentRef(ctx, paymentRef)
	if err != nil {
		h.logger.Error("payment not found for timeout",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
		h.sendCallbackResponse(w, "0", "Success")
		return
	}

	_ = h.paymentUC.UpdateStatus(ctx, payment. ID, domain.PaymentStatusFailed)
	_ = h.paymentUC.SetError(ctx, payment.ID, "B2C request timed out")

	h.logger.Info("payment marked as timed out",
		zap. String("payment_ref", paymentRef))

	h.sendCallbackResponse(w, "0", "Success")
}

// ============================================
// B2B CALLBACKS (Bank Withdrawals)
// ============================================

// HandleMpesaB2BCallback handles M-Pesa B2B callback (bank transfers)
func (h *CallbackHandler) HandleMpesaB2BCallback(w http.ResponseWriter, r *http.Request) {
	paymentRef := chi.URLParam(r, "payment_ref")

	h.logger.Info("received M-Pesa B2B callback",
		zap.String("payment_ref", paymentRef),
		zap.String("remote_addr", r.RemoteAddr))

	// Read payload
	payload, err := io.ReadAll(r. Body)
	if err != nil {
		h.logger.Error("failed to read B2B callback payload",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
		h.sendCallbackResponse(w, "1", "Failed to read payload")
		return
	}

	h. logger.Debug("M-Pesa B2B callback payload received",
		zap.String("payment_ref", paymentRef),
		zap.Int("payload_size", len(payload)),
		zap.String("payload", string(payload)))

	// Process callback asynchronously with timeout context
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := h.callbackUC.ProcessMpesaB2BCallback(ctx, paymentRef, payload); err != nil {
			h.logger.Error("failed to process M-Pesa B2B callback",
				zap.String("payment_ref", paymentRef),
				zap.Error(err))
		}
	}()

	// Return success immediately
	h.logger.Info("M-Pesa B2B callback acknowledged",
		zap.String("payment_ref", paymentRef))
	h.sendCallbackResponse(w, "0", "Success")
}

// HandleMpesaB2BTimeout handles M-Pesa B2B timeout
func (h *CallbackHandler) HandleMpesaB2BTimeout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	paymentRef := chi.URLParam(r, "payment_ref")

	h.logger.Warn("received M-Pesa B2B timeout",
		zap.String("payment_ref", paymentRef),
		zap.String("remote_addr", r.RemoteAddr))

	// Read payload
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read B2B timeout payload",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
		h.sendCallbackResponse(w, "1", "Failed to read payload")
		return
	}

	h.logger.Info("M-Pesa B2B timeout payload",
		zap.String("payment_ref", paymentRef),
		zap.String("payload", string(payload)))

	// Mark payment as failed due to timeout
	payment, err := h. paymentUC.GetByPaymentRef(ctx, paymentRef)
	if err != nil {
		h.logger.Error("payment not found for B2B timeout",
			zap.String("payment_ref", paymentRef),
			zap.Error(err))
		h.sendCallbackResponse(w, "0", "Success")
		return
	}

	_ = h.paymentUC.UpdateStatus(ctx, payment.ID, domain.PaymentStatusFailed)
	_ = h.paymentUC.SetError(ctx, payment.ID, "B2B request timed out")

	h.logger.Info("bank withdrawal marked as timed out",
		zap.String("payment_ref", paymentRef))

	h.sendCallbackResponse(w, "0", "Success")
}

// ============================================
// SHARED HELPERS
// ============================================

// sendCallbackResponse sends M-Pesa callback response
func (h *CallbackHandler) sendCallbackResponse(w http.ResponseWriter, resultCode, resultDesc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// M-Pesa expects this specific format
	response := map[string]interface{}{
		"ResultCode":  resultCode,
		"ResultDesc": resultDesc,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger. Error("failed to encode callback response", zap.Error(err))
	}
}
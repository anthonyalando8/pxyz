// internal/handler/callback_handler.go
package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"payment-service/internal/usecase"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type CallbackHandler struct {
    callbackUC *usecase.CallbackUsecase
    logger     *zap.Logger
}

func NewCallbackHandler(callbackUC *usecase.CallbackUsecase, logger *zap.Logger) *CallbackHandler {
    return &CallbackHandler{
        callbackUC: callbackUC,
        logger:      logger,
    }
}

// HandleMpesaSTKCallback handles M-Pesa STK Push callback
func (h *CallbackHandler) HandleMpesaSTKCallback(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    paymentRef := chi.URLParam(r, "payment_ref")

    h.logger.Info("received M-Pesa STK callback",
        zap.String("payment_ref", paymentRef),
        zap.String("remote_addr", r.RemoteAddr))

    // Read payload
    payload, err := io. ReadAll(r.Body)
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

    // Process callback asynchronously
    go func() {
        if err := h.callbackUC.ProcessMpesaSTKCallback(ctx, paymentRef, payload); err != nil {
            h.logger.Error("failed to process M-Pesa STK callback",
                zap.String("payment_ref", paymentRef),
                zap.Error(err))
        }
    }()

    // Return success immediately (M-Pesa requires quick response)
    h.logger.Info("M-Pesa STK callback acknowledged",
        zap.String("payment_ref", paymentRef))
    h.sendCallbackResponse(w, "0", "Success")
}

// HandleMpesaB2CCallback handles M-Pesa B2C callback
func (h *CallbackHandler) HandleMpesaB2CCallback(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    paymentRef := chi.URLParam(r, "payment_ref")

    h.logger.Info("received M-Pesa B2C callback",
        zap.String("payment_ref", paymentRef),
        zap.String("remote_addr", r.RemoteAddr))

    // Read payload
    payload, err := io.ReadAll(r.Body)
    if err != nil {
        h.logger.Error("failed to read B2C callback payload",
            zap.String("payment_ref", paymentRef),
            zap.Error(err))
        h.sendCallbackResponse(w, "1", "Failed to read payload")
        return
    }

    h.logger.Debug("M-Pesa B2C callback payload received",
        zap. String("payment_ref", paymentRef),
        zap.Int("payload_size", len(payload)))

    // Process callback asynchronously
    go func() {
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
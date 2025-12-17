// internal/handler/webhook_handler.go
package handler

import (
    "encoding/json"
    "io"
    "net/http"

    "payment-service/internal/domain"
    "payment-service/internal/usecase"
    
    "go.uber.org/zap"
)

type WebhookHandler struct {
    paymentUC *usecase.PaymentUsecase
    logger    *zap.Logger
}

func NewWebhookHandler(paymentUC *usecase.PaymentUsecase, logger *zap. Logger) *WebhookHandler {
    return &WebhookHandler{
        paymentUC:  paymentUC,
        logger:    logger,
    }
}

// HandleDepositWebhook handles incoming deposit requests from partner
func (h *WebhookHandler) HandleDepositWebhook(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    h.logger.Info("received deposit webhook",
        zap.String("remote_addr", r.RemoteAddr),
        zap.String("user_agent", r.UserAgent()))

    // Parse request
    var req domain.DepositRequest
    if err := json. NewDecoder(r.Body).Decode(&req); err != nil {
        h.logger. Error("failed to decode deposit request",
            zap.Error(err))
        h.sendError(w, http.StatusBadRequest, "invalid request body", err)
        return
    }

    h.logger.Info("deposit request parsed",
        zap.String("transaction_ref", req.TransactionRef),
        zap.String("partner_id", req. PartnerID),
        zap.String("provider", string(req.Provider)),
        zap.Float64("amount", req. Amount))

    // Initiate deposit
    payment, err := h.paymentUC.InitiateDeposit(ctx, &req)
    if err != nil {
        h.logger.Error("failed to initiate deposit",
            zap.String("transaction_ref", req.TransactionRef),
            zap.Error(err))
        h.sendError(w, http.StatusInternalServerError, "failed to process deposit", err)
        return
    }

    h.logger.Info("deposit initiated successfully",
        zap.String("payment_ref", payment.PaymentRef),
        zap.String("status", string(payment.Status)))

    // Send response
    h.sendSuccess(w, http.StatusOK, "deposit initiated successfully", map[string]interface{}{
        "payment_ref":     payment.PaymentRef,
        "partner_tx_ref":  payment.PartnerTxRef,
        "status":         payment.Status,
        "amount":          payment.Amount,
        "currency":       payment.Currency,
        "provider":        payment.Provider,
    })
}

// HandleAllWebhooks handles all incoming webhooks for logging purposes
func (h *WebhookHandler) HandleAllWebhooks(w http.ResponseWriter, r *http.Request) {
    h.logger.Info("received generic webhook",
        zap.String("remote_addr", r.RemoteAddr),
        zap.String("user_agent", r. UserAgent()),
        zap.String("method", r.Method),
        zap.String("path", r.URL.Path),
        zap.String("content_type", r.Header.Get("Content-Type")))

    // Read body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        h.logger.Error("failed to read webhook body",
            zap.Error(err))
        h.sendError(w, http.StatusBadRequest, "failed to read request body", err)
        return
    }

    // Parse as generic JSON
    var payload map[string]interface{}
    if err := json.Unmarshal(body, &payload); err != nil {
        h.logger. Warn("webhook body is not valid JSON",
            zap.Error(err),
            zap.String("raw_body", string(body)))
    } else {
        h.logger. Info("webhook payload received",
            zap.Any("payload", payload),
            zap.Int("payload_size", len(body)))
    }

    // Log headers
    h.logger.Debug("webhook headers",
        zap.Any("headers", r.Header))

    // Send success response
    h.sendSuccess(w, http.StatusOK, "webhook received and logged", map[string]interface{}{
        "received":  true,
        "timestamp": "processed",
    })
}

// Response helpers
func (h *WebhookHandler) sendSuccess(w http.ResponseWriter, statusCode int, message string, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": message,
        "data":    data,
    })
}

func (h *WebhookHandler) sendError(w http.ResponseWriter, statusCode int, message string, err error) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    
    response := map[string]interface{}{
        "success": false,
        "message": message,
    }
    
    if err != nil {
        response["error"] = err.Error()
    }
    
    json.NewEncoder(w).Encode(response)
}
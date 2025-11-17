// handler/partner_api_handler.go
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"partner-service/internal/domain"
	"x/shared/auth/middleware"
	authpb "x/shared/genproto/partner/authpb"
	"x/shared/response"

	"github.com/go-chi/chi/v5"
)

// ==================== API CREDENTIALS MANAGEMENT ====================

// GenerateAPICredentials creates new API credentials for the current partner
func (h *PartnerHandler) GenerateAPICredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user's partner ID
	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Generate credentials
	apiKey, apiSecret, err := h.uc.GenerateAPICredentials(ctx, partnerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to generate API credentials: "+err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"api_key":    apiKey,
		"api_secret": apiSecret,
		"partner_id": partnerID,
		"message":    "API credentials generated successfully. Store the secret securely - it won't be shown again.",
	})
}

// RevokeAPICredentials removes API access for the current partner
func (h *PartnerHandler) RevokeAPICredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	if err := h.uc.RevokeAPICredentials(ctx, partnerID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to revoke API credentials: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "API credentials revoked successfully",
	})
}

// RotateAPISecret generates a new API secret while keeping the same key
func (h *PartnerHandler) RotateAPISecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	apiSecret, err := h.uc.RotateAPISecret(ctx, partnerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to rotate API secret: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"api_secret": apiSecret,
		"message":    "API secret rotated successfully. Update your integration with the new secret.",
	})
}

// GetAPISettings retrieves current API configuration
func (h *PartnerHandler) GetAPISettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	partner, err := h.uc.GetPartnerByID(ctx, partnerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch partner settings: "+err.Error())
		return
	}

	settings := map[string]interface{}{
		"partner_id":      partner.ID,
		"is_api_enabled":  partner.IsAPIEnabled,
		"api_rate_limit":  partner.APIRateLimit,
		"allowed_ips":     partner.AllowedIPs,
		"has_webhook_url": partner.WebhookURL != nil && *partner.WebhookURL != "",
		"has_callback_url": partner.CallbackURL != nil && *partner.CallbackURL != "",
	}

	if partner.APIKey != nil {
		settings["api_key"] = *partner.APIKey
	}
	if partner.WebhookURL != nil {
		settings["webhook_url"] = *partner.WebhookURL
	}
	if partner.CallbackURL != nil {
		settings["callback_url"] = *partner.CallbackURL
	}

	response.JSON(w, http.StatusOK, settings)
}

// UpdateAPISettings updates API configuration
func (h *PartnerHandler) UpdateAPISettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req struct {
		IsAPIEnabled bool     `json:"is_api_enabled"`
		APIRateLimit int      `json:"api_rate_limit"`
		AllowedIPs   []string `json:"allowed_ips"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate rate limit
	if req.APIRateLimit < 10 || req.APIRateLimit > 10000 {
		response.Error(w, http.StatusBadRequest, "api_rate_limit must be between 10 and 10000")
		return
	}

	if err := h.uc.UpdateAPISettings(ctx, partnerID, req.IsAPIEnabled, req.APIRateLimit, req.AllowedIPs); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update API settings: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "API settings updated successfully",
	})
}

// ==================== TRANSACTION MANAGEMENT ====================

// InitiateDeposit creates a new deposit transaction (partner-initiated)
func (h *PartnerHandler) InitiateDeposit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req struct {
		TransactionRef string                 `json:"transaction_ref"`
		UserID         string                 `json:"user_id"`
		Amount         float64                `json:"amount"`
		Currency       string                 `json:"currency"`
		PaymentMethod  string                 `json:"payment_method"`
		ExternalRef    string                 `json:"external_ref"`
		Metadata       map[string]interface{} `json:"metadata"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validation
	if req.TransactionRef == "" {
		response.Error(w, http.StatusBadRequest, "transaction_ref is required")
		return
	}
	if req.UserID == "" {
		response.Error(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}
	if req.Currency == "" {
		response.Error(w, http.StatusBadRequest, "currency is required")
		return
	}

	txn := &domain.PartnerTransaction{
		PartnerID:      partnerID,
		TransactionRef: req.TransactionRef,
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Metadata:       req.Metadata,
	}

	if req.PaymentMethod != "" {
		txn.PaymentMethod = &req.PaymentMethod
	}
	if req.ExternalRef != "" {
		txn.ExternalRef = &req.ExternalRef
	}

	if err := h.uc.InitiateDeposit(ctx, txn); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"transaction_id":  txn.ID,
		"transaction_ref": txn.TransactionRef,
		"status":          txn.Status,
		"message":         "Deposit initiated successfully",
	})
}

// GetTransactionStatus retrieves transaction by reference
func (h *PartnerHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	transactionRef := chi.URLParam(r, "ref")
	if transactionRef == "" {
		response.Error(w, http.StatusBadRequest, "transaction reference is required")
		return
	}

	txn, err := h.uc.GetTransactionStatus(ctx, partnerID, transactionRef)
	if err != nil {
		response.Error(w, http.StatusNotFound, "transaction not found")
		return
	}

	response.JSON(w, http.StatusOK, txn)
}

// ListTransactions returns paginated list of transactions
func (h *PartnerHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	status := r.URL.Query().Get("status")

	limit := 20
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var statusFilter *string
	if status != "" {
		statusFilter = &status
	}

	txns, total, err := h.uc.ListTransactions(ctx, partnerID, limit, offset, statusFilter, nil, nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch transactions: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txns,
		"total_count":  total,
		"limit":        limit,
		"offset":       offset,
	})
}

// GetTransactionsByDateRange returns transactions within a date range
func (h *PartnerHandler) GetTransactionsByDateRange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req struct {
		From   time.Time `json:"from"`
		To     time.Time `json:"to"`
		Status string    `json:"status"`
		Limit  int       `json:"limit"`
		Offset int       `json:"offset"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	var statusFilter *string
	if req.Status != "" {
		statusFilter = &req.Status
	}

	txns, total, err := h.uc.ListTransactions(ctx, partnerID, req.Limit, req.Offset, statusFilter, &req.From, &req.To)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch transactions: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txns,
		"total_count":  total,
		"from":         req.From,
		"to":           req.To,
		"limit":        req.Limit,
		"offset":       req.Offset,
	})
}

// ==================== WEBHOOK MANAGEMENT ====================

// UpdateWebhookConfig updates webhook settings
func (h *PartnerHandler) UpdateWebhookConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req struct {
		WebhookURL    string `json:"webhook_url"`
		WebhookSecret string `json:"webhook_secret"`
		CallbackURL   string `json:"callback_url"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.uc.UpdateWebhookConfig(ctx, partnerID, req.WebhookURL, req.WebhookSecret, req.CallbackURL); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update webhook config: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Webhook configuration updated successfully",
	})
}

// TestWebhook sends a test webhook to verify configuration
func (h *PartnerHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	statusCode, err := h.uc.TestWebhook(ctx, partnerID)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "webhook test failed: "+err.Error())
		return
	}

	success := statusCode >= 200 && statusCode < 300

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"success":     success,
		"status_code": statusCode,
		"message":     "Webhook test completed",
	})
}

// ListWebhookLogs returns webhook delivery logs
func (h *PartnerHandler) ListWebhookLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logs, total, err := h.uc.ListWebhookLogs(ctx, partnerID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch webhook logs: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"logs":        logs,
		"total_count": total,
		"limit":       limit,
		"offset":      offset,
	})
}

// RetryFailedWebhook manually retries a failed webhook
func (h *PartnerHandler) RetryFailedWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	webhookIDStr := chi.URLParam(r, "id")
	webhookID, err := strconv.ParseInt(webhookIDStr, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid webhook id")
		return
	}

	// Verify webhook belongs to partner
	webhook, err := h.uc.GetWebhookByID(ctx, webhookID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "webhook not found")
		return
	}

	if webhook.PartnerID != partnerID {
		response.Error(w, http.StatusForbidden, "webhook does not belong to this partner")
		return
	}

	// Retry webhook (implement in usecase)
	if err := h.uc.RetryWebhook(ctx, webhookID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to retry webhook: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Webhook retry initiated",
	})
}

// ==================== API LOGS ====================

// GetAPILogs returns API request logs for the partner
func (h *PartnerHandler) GetAPILogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	endpoint := r.URL.Query().Get("endpoint")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var endpointFilter *string
	if endpoint != "" {
		endpointFilter = &endpoint
	}

	logs, total, err := h.uc.GetAPILogs(ctx, partnerID, limit, offset, endpointFilter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch API logs: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"logs":        logs,
		"total_count": total,
		"limit":       limit,
		"offset":      offset,
	})
}

// GetAPIUsageStats returns API usage statistics
func (h *PartnerHandler) GetAPIUsageStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	partnerID, err := h.getPartnerIDFromContext(ctx)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	}

	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Default to last 30 days if not specified
	if req.To.IsZero() {
		req.To = time.Now()
	}
	if req.From.IsZero() {
		req.From = req.To.AddDate(0, 0, -30)
	}

	stats, err := h.uc.GetAPIUsageStats(ctx, partnerID, req.From, req.To)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch API usage stats: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, stats)
}

// ==================== HELPER METHODS ====================

// getPartnerIDFromContext extracts partner ID from authenticated user
func (h *PartnerHandler) getPartnerIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		return "", errors.New("missing or invalid user ID")
	}

	// Fetch partner ID from profile
	profileResp, err := h.authClient.PartnerClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
		UserId: userID,
	})
	if err != nil || profileResp == nil || profileResp.User == nil {
		return "", errors.New("failed to fetch user profile from auth service")
	}

	partnerID := profileResp.User.PartnerId
	if partnerID == "" {
		return "", errors.New("your account is not linked to a partner")
	}

	return partnerID, nil
}
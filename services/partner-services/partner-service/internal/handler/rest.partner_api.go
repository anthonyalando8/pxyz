// handler/partner_api_handler.go
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	//"partner-service/internal/domain"
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
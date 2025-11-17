// usecase/partner_webhooks.go
package usecase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"partner-service/internal/domain"
	"time"
)

// UpdateWebhookConfig updates webhook settings for a partner
func (uc *PartnerUsecase) UpdateWebhookConfig(ctx context.Context, partnerID, webhookURL, webhookSecret, callbackURL string) error {
	if partnerID == "" {
		return errors.New("partner_id is required")
	}

	return uc.partnerRepo.UpdateWebhookConfig(ctx, partnerID, webhookURL, webhookSecret, callbackURL)
}

// SendWebhook sends a webhook notification to partner
func (uc *PartnerUsecase) SendWebhook(ctx context.Context, partnerID, eventType string, payload map[string]interface{}) error {
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, partnerID)
	if err != nil {
		return fmt.Errorf("failed to get partner: %w", err)
	}

	if partner.WebhookURL == nil || *partner.WebhookURL == "" {
		return errors.New("partner has no webhook URL configured")
	}

	// Create webhook record
	webhook := &domain.PartnerWebhook{
		PartnerID:   partnerID,
		EventType:   eventType,
		Payload:     payload,
		Status:      "pending",
		Attempts:    0,
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store webhook (for retry mechanism)
	if err := uc.partnerRepo.CreateWebhook(ctx, webhook); err != nil {
		return fmt.Errorf("failed to create webhook record: %w", err)
	}

	// Send webhook asynchronously
	go uc.executeWebhook(context.Background(), webhook.ID, partner)

	return nil
}

// executeWebhook performs the actual HTTP request
func (uc *PartnerUsecase) executeWebhook(ctx context.Context, webhookID int64, partner *domain.Partner) {
	webhook, err := uc.partnerRepo.GetWebhookByID(ctx, webhookID)
	if err != nil {
		return
	}

	// Marshal payload
	payloadBytes, err := json.Marshal(webhook.Payload)
	if err != nil {
		uc.updateWebhookStatus(ctx, webhookID, "failed", 0, "", fmt.Sprintf("failed to marshal payload: %v", err))
		return
	}

	// Create signature
	var signature string
	if partner.WebhookSecret != nil && *partner.WebhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(*partner.WebhookSecret))
		mac.Write(payloadBytes)
		signature = hex.EncodeToString(mac.Sum(nil))
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", *partner.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		uc.updateWebhookStatus(ctx, webhookID, "failed", 0, "", fmt.Sprintf("failed to create request: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", webhook.EventType)
	req.Header.Set("X-Webhook-ID", fmt.Sprintf("%d", webhookID))

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		uc.updateWebhookStatus(ctx, webhookID, "retrying", 0, "", fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	// Read response
	var responseBody bytes.Buffer
	responseBody.ReadFrom(resp.Body)

	// Update webhook status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		uc.updateWebhookStatus(ctx, webhookID, "sent", resp.StatusCode, responseBody.String(), "")
	} else {
		uc.updateWebhookStatus(ctx, webhookID, "retrying", resp.StatusCode, responseBody.String(), fmt.Sprintf("received status code %d", resp.StatusCode))
	}
}

func (uc *PartnerUsecase) updateWebhookStatus(ctx context.Context, webhookID int64, status string, statusCode int, responseBody, errorMsg string) {
	// Implement webhook status update in repo
	uc.partnerRepo.UpdateWebhookStatus(ctx, webhookID, status, statusCode, responseBody, errorMsg)
}

// usecase/partner_webhooks.go - ADD THESE METHODS

// GetWebhookByID retrieves a webhook by its ID
func (uc *PartnerUsecase) GetWebhookByID(ctx context.Context, webhookID int64) (*domain.PartnerWebhook, error) {
	if webhookID <= 0 {
		return nil, errors.New("invalid webhook ID")
	}

	webhook, err := uc.partnerRepo.GetWebhookByID(ctx, webhookID)
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook: %w", err)
	}

	return webhook, nil
}

// RetryWebhook manually retries a failed webhook delivery
func (uc *PartnerUsecase) RetryWebhook(ctx context.Context, webhookID int64) error {
	if webhookID <= 0 {
		return errors.New("invalid webhook ID")
	}

	// Get webhook details
	webhook, err := uc.partnerRepo.GetWebhookByID(ctx, webhookID)
	if err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}

	// Check if webhook can be retried
	if webhook.Status == "sent" {
		return errors.New("webhook already sent successfully")
	}

	if webhook.Attempts >= webhook.MaxAttempts {
		return fmt.Errorf("webhook has reached maximum retry attempts (%d)", webhook.MaxAttempts)
	}

	// Get partner details for webhook URL
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, webhook.PartnerID)
	if err != nil {
		return fmt.Errorf("failed to get partner: %w", err)
	}

	if partner.WebhookURL == nil || *partner.WebhookURL == "" {
		return errors.New("partner has no webhook URL configured")
	}

	// Reset webhook status for retry
	if err := uc.partnerRepo.ResetWebhookForRetry(ctx, webhookID); err != nil {
		return fmt.Errorf("failed to reset webhook for retry: %w", err)
	}

	// Execute webhook asynchronously
	go uc.executeWebhook(context.Background(), webhookID, partner)

	return nil
}


// TestWebhook sends a test webhook to verify configuration
func (uc *PartnerUsecase) TestWebhook(ctx context.Context, partnerID string) (int, error) {
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, partnerID)
	if err != nil {
		return 0, fmt.Errorf("failed to get partner: %w", err)
	}

	if partner.WebhookURL == nil || *partner.WebhookURL == "" {
		return 0, errors.New("partner has no webhook URL configured")
	}

	// Create test payload
	testPayload := map[string]interface{}{
		"event_type": "webhook.test",
		"timestamp":  time.Now().Unix(),
		"message":    "This is a test webhook",
	}

	payloadBytes, _ := json.Marshal(testPayload)

	// Create and send request
	req, err := http.NewRequestWithContext(ctx, "POST", *partner.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// ListWebhookLogs returns webhook delivery logs
func (uc *PartnerUsecase) ListWebhookLogs(ctx context.Context, partnerID string, limit, offset int) ([]domain.PartnerWebhook, int64, error) {
	if partnerID == "" {
		return nil, 0, errors.New("partner_id is required")
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return uc.partnerRepo.ListWebhookLogs(ctx, partnerID, limit, offset)
}
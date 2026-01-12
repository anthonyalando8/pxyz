// pkg/client/partner_credit_client.go
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"payment-service/config"

	"go.uber.org/zap"
)

type PartnerCreditClient struct {
	config     *config.Config  // ✅ Changed to full config
	httpClient *http.Client
	logger     *zap. Logger
}

// ✅ Updated constructor
func NewPartnerCreditClient(cfg *config.Config, logger *zap.Logger) *PartnerCreditClient {
	return &PartnerCreditClient{
		config:  cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:  logger,
	}
}

// CreditUserRequest represents the credit request sent to partner
type CreditUserRequest struct {
	UserID         string  `json:"user_id"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	TransactionRef string  `json:"transaction_ref"`
	Description    string  `json:"description"`
	ExternalRef    string  `json:"external_ref"`
}

// CreditUserResponse represents partner credit response
type CreditUserResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	ReceiptCode    string `json:"receipt_code,omitempty"`
	Error          string `json:"error,omitempty"`
	TransactionRef string `json:"transaction_ref"`  // ✅ Fixed typo
	TransactionID  string `json:"transaction_id"`
	JournalID      string `json:"journal_id"`
	CreatedAt      string `json:"created_at"`
}

// ✅ Updated CreditUser to accept partnerID
func (c *PartnerCreditClient) CreditUser(ctx context.Context, partnerID string, req *CreditUserRequest) (*CreditUserResponse, error) {
	// Get partner config
	partner, err := c.config.GetPartner(partnerID)
	if err != nil {
		c. logger.Error("failed to get partner config",
			zap.String("partner_id", partnerID),
			zap.Error(err))
		return nil, fmt.Errorf("partner not found: %w", err)
	}

	c.logger.Info("crediting user on partner system",
		zap.String("partner_id", partnerID),
		zap.String("partner_name", partner.Name),
		zap.String("user_id", req.UserID),
		zap.String("transaction_ref", req.TransactionRef),
		zap.Float64("amount", req.Amount),
		zap.String("currency", req.Currency),
		zap.String("external_ref", req.ExternalRef))

	// Build URL - use partner's webhook URL
	url := fmt.Sprintf("%s/transactions/credit", partner.WebhookURL)

	// Marshal payload
	payload, err := json.Marshal(req)
	if err != nil {
		c.logger. Error("failed to marshal credit request",
			zap.String("partner_id", partnerID),
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes. NewBuffer(payload))
	if err != nil {
		c. logger.Error("failed to create credit request",
			zap.String("partner_id", partnerID),
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers with partner-specific credentials
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", partner.APIKey)
	httpReq.Header.Set("X-API-Secret", partner.APISecret)

	// Generate signature using partner's secret
	timestamp := time.Now().Unix()
	signature := generateSignature(payload, timestamp, partner.APISecret)
	httpReq.Header.Set("X-Signature", signature)
	httpReq.Header.Set("X-Timestamp", fmt.Sprintf("%d", timestamp))

	c.logger.Debug("sending credit request to partner",
		zap.String("partner_id", partnerID),
		zap.String("url", url),
		zap.String("user_id", req. UserID))

	// Send request
	resp, err := c. httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("failed to send credit request",
			zap.String("partner_id", partnerID),
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to send request:  %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var response CreditUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		c.logger.Error("failed to decode credit response",
			zap. String("partner_id", partnerID),
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp. StatusCode != http.StatusOK || !response.Success {
		c.logger.Error("partner credit request failed",
			zap. String("partner_id", partnerID),
			zap.String("partner_name", partner.Name),
			zap.String("user_id", req.UserID),
			zap.Int("status_code", resp.StatusCode),
			zap.String("error", response.Error),
			zap.String("message", response.Message))
		return &response, fmt.Errorf("credit failed: %s", response.Error)
	}

	c.logger.Info("user credited successfully on partner system",
		zap.String("partner_id", partnerID),
		zap.String("partner_name", partner.Name),
		zap.String("user_id", req.UserID),
		zap.String("receipt_code", response.ReceiptCode),
		zap.String("transaction_ref", response.TransactionRef),
		zap.String("transaction_id", response.TransactionID))

	return &response, nil
}
// pkg/client/partner_debit_client.go
package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"payment-service/config"

	"go.uber.org/zap"
)

type PartnerDebitClient struct {
	config     config. PartnerConfig
	httpClient *http.Client
	logger     *zap. Logger
}

func NewPartnerDebitClient(cfg config. PartnerConfig, logger *zap.Logger) *PartnerDebitClient {
	return &PartnerDebitClient{
		config:  cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:  logger,
	}
}

// DebitUserRequest represents the debit request sent to partner
type DebitUserRequest struct {
	UserID         string                 `json:"user_id"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	TransactionRef string                 `json:"transaction_ref"`
	Description    string                 `json:"description"`
	ExternalRef    string                 `json:"external_ref"` // M-Pesa code or bank reference
	PaymentMethod  string                 `json:"payment_method,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// DebitUserResponse represents partner debit response
type DebitUserResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	TransactionRef string `json:"transaction_ref"`
	TransactionID  int64  `json:"transaction_id"`
	ExternalRef    string `json:"external_ref"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
}

// DebitUser debits user account on partner system (for withdrawal completion)
func (c *PartnerDebitClient) DebitUser(ctx context.Context, req *DebitUserRequest) (*DebitUserResponse, error) {
	c.logger.Info("debiting user on partner system",
		zap.String("user_id", req.UserID),
		zap.String("transaction_ref", req.TransactionRef),
		zap.Float64("amount", req.Amount),
		zap.String("currency", req.Currency),
		zap.String("external_ref", req.ExternalRef),
		zap.String("payment_method", req.PaymentMethod))

	// Build URL
	url := fmt.Sprintf("%s/api/v1/partner/api/transactions/debit", c.config.WebhookURL)

	// Marshal payload
	payload, err := json.Marshal(req)
	if err != nil {
		c.logger. Error("failed to marshal debit request",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		c.logger.Error("failed to create debit request",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.config.APIKey)
	httpReq.Header.Set("X-API-Secret", c.config.APISecret)

	// Generate signature
	timestamp := time.Now().Unix()
	signature := c.generateSignature(payload, timestamp)
	httpReq.Header.Set("X-Signature", signature)
	httpReq.Header.Set("X-Timestamp", fmt.Sprintf("%d", timestamp))

	c.logger.Debug("sending debit request to partner",
		zap.String("url", url),
		zap.String("user_id", req.UserID))

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c. logger.Error("failed to send debit request",
			zap. String("user_id", req. UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to send request:  %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var response DebitUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		c.logger.Error("failed to decode debit response",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK || ! response.Success {
		c. logger.Error("partner debit request failed",
			zap.String("user_id", req.UserID),
			zap.Int("status_code", resp.StatusCode),
			zap.String("error", response. Error))
		return &response, fmt.Errorf("debit failed: %s", response.Error)
	}

	c.logger.Info("user debited successfully on partner system",
		zap.String("user_id", req.UserID),
		zap.String("transaction_ref", response.TransactionRef),
		zap.String("external_ref", response.ExternalRef))

	return &response, nil
}

// generateSignature generates HMAC-SHA256 signature
func (c *PartnerDebitClient) generateSignature(payload []byte, timestamp int64) string {
	message := fmt. Sprintf("%s. %d", string(payload), timestamp)
	h := hmac.New(sha256.New, []byte(c.config.APISecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
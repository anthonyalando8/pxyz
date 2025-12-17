// pkg/client/partner_credit_client.go
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

type PartnerCreditClient struct {
    config     config.PartnerConfig
    httpClient *http.Client
    logger     *zap. Logger
}

func NewPartnerCreditClient(cfg config.PartnerConfig, logger *zap.Logger) *PartnerCreditClient {
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
    Success     bool   `json:"success"`
    Message     string `json:"message"`
    ReceiptCode string `json:"receipt_code,omitempty"`
    Error       string `json:"error,omitempty"`
}

// CreditUser credits user account on partner system
func (c *PartnerCreditClient) CreditUser(ctx context.Context, req *CreditUserRequest) (*CreditUserResponse, error) {
    c.logger.Info("crediting user on partner system",
        zap. String("user_id", req. UserID),
        zap.String("transaction_ref", req.TransactionRef),
        zap.Float64("amount", req.Amount),
        zap.String("currency", req.Currency),
        zap.String("external_ref", req.ExternalRef))

    // Build URL
    url := fmt.Sprintf("%s/api/transactions/credit", c.config.WebhookURL)

    // Marshal payload
    payload, err := json. Marshal(req)
    if err != nil {
        c.logger. Error("failed to marshal credit request",
            zap.String("user_id", req.UserID),
            zap.Error(err))
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
    if err != nil {
        c.logger. Error("failed to create credit request",
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

    c.logger.Debug("sending credit request to partner",
        zap.String("url", url),
        zap.String("user_id", req.UserID))

    // Send request
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        c.logger.Error("failed to send credit request",
            zap.String("user_id", req.UserID),
            zap.Error(err))
        return nil, fmt. Errorf("failed to send request:  %w", err)
    }
    defer resp.Body.Close()

    // Parse response
    var response CreditUserResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        c.logger.Error("failed to decode credit response",
            zap. String("user_id", req. UserID),
            zap.Error(err))
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    if resp.StatusCode != http.StatusOK || ! response.Success {
        c. logger.Error("partner credit request failed",
            zap.String("user_id", req.UserID),
            zap.Int("status_code", resp.StatusCode),
            zap.String("error", response.Error))
        return &response, fmt.Errorf("credit failed: %s", response.Error)
    }

    c.logger.Info("user credited successfully on partner system",
        zap. String("user_id", req. UserID),
        zap.String("receipt_code", response.ReceiptCode))

    return &response, nil
}

// generateSignature generates HMAC-SHA256 signature
func (c *PartnerCreditClient) generateSignature(payload []byte, timestamp int64) string {
    message := fmt.Sprintf("%s. %d", string(payload), timestamp)
    h := hmac.New(sha256.New, []byte(c. config.APISecret))
    h.Write([]byte(message))
    return hex.EncodeToString(h.Sum(nil))
}
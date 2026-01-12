// pkg/client/partner_client.go
package client

import (
    "bytes"
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "payment-service/config"
    
    "go.uber.org/zap"
)

type PartnerClient struct {
    config     *config.Config  //  Changed to full config
    httpClient *http.Client
    logger     *zap.Logger
}

//  Updated constructor
func NewPartnerClient(cfg *config.Config, logger *zap.Logger) *PartnerClient {
    return &PartnerClient{
        config:  cfg,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        logger:  logger,
    }
}

// PartnerNotification represents the notification sent to partner
type PartnerNotification struct {
    PaymentRef        string  `json:"payment_ref"`
    PartnerTxRef      string  `json:"partner_tx_ref"`
    Status            string  `json:"status"`
    Amount            float64 `json:"amount"`
    Currency          string  `json:"currency"`
    ProviderReference string  `json:"provider_reference"`
    ResultCode        string  `json:"result_code"`
    ResultDescription string  `json:"result_description"`
    Timestamp         int64   `json:"timestamp"`
}

//  Updated SendNotification to accept partnerID
func (c *PartnerClient) SendNotification(ctx context.Context, partnerID string, notification *PartnerNotification) error {
    // Get partner config
    partner, err := c. config.GetPartner(partnerID)
    if err != nil {
        c.logger.Error("failed to get partner config",
            zap. String("partner_id", partnerID),
            zap.Error(err))
        return fmt. Errorf("partner not found: %w", err)
    }

    // Add timestamp
    notification.Timestamp = time. Now().Unix()

    c.logger.Info("sending notification to partner",
        zap.String("partner_id", partnerID),
        zap.String("partner_name", partner.Name),
        zap.String("payment_ref", notification.PaymentRef),
        zap.String("status", notification.Status))

    // Marshal payload
    payload, err := json.Marshal(notification)
    if err != nil {
        c.logger.Error("failed to marshal notification",
            zap.String("partner_id", partnerID),
            zap.Error(err))
        return fmt. Errorf("failed to marshal notification:  %w", err)
    }

    // Create request
    req, err := http.NewRequestWithContext(ctx, "POST", partner.WebhookURL, bytes. NewBuffer(payload))
    if err != nil {
        c. logger.Error("failed to create request",
            zap.String("partner_id", partnerID),
            zap.Error(err))
        return fmt.Errorf("failed to create request: %w", err)
    }

    // Set headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-API-Key", partner.APIKey)
    req.Header.Set("X-Timestamp", fmt.Sprintf("%d", notification.Timestamp))

    // Generate signature using partner's secret
    signature := generateSignature(payload, notification. Timestamp, partner.APISecret)
    req.Header.Set("X-Signature", signature)

    c.logger.Debug("partner notification request prepared",
        zap.String("partner_id", partnerID),
        zap.String("url", partner.WebhookURL),
        zap.String("payment_ref", notification.PaymentRef))

    // Send request
    resp, err := c.httpClient.Do(req)
    if err != nil {
        c.logger.Error("failed to send notification to partner",
            zap.String("partner_id", partnerID),
            zap.String("payment_ref", notification.PaymentRef),
            zap.Error(err))
        return fmt. Errorf("failed to send notification:  %w", err)
    }
    defer resp.Body.Close()

    // Read response
    responseBody, _ := io.ReadAll(resp. Body)

    if resp.StatusCode != http.StatusOK {
        c.logger.Error("partner returned non-OK status",
            zap.String("partner_id", partnerID),
            zap.String("payment_ref", notification.PaymentRef),
            zap.Int("status_code", resp.StatusCode),
            zap.String("response", string(responseBody)))
        return fmt.Errorf("partner returned status %d: %s", resp. StatusCode, string(responseBody))
    }

    c.logger.Info("partner notified successfully",
        zap. String("partner_id", partnerID),
        zap.String("partner_name", partner.Name),
        zap.String("payment_ref", notification.PaymentRef),
        zap.Int("status_code", resp.StatusCode))

    return nil
}

//  Exported helper function
func generateSignature(payload []byte, timestamp int64, secret string) string {
    message := fmt.Sprintf("%s. %d", string(payload), timestamp)
    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(message))
    return hex.EncodeToString(h. Sum(nil))
}
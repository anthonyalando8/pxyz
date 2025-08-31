package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sms-service/internal/domain"
	"time"
	"log"
)

type MessageUsecase struct {
    SMSAPIKey    string
    WhatsAppKey  string
    SMSBaseURL   string
    WAAPIBaseURL string
    SenderID     string
	UserID   string
	Password string
    client       *http.Client
}

func NewMessageUsecase(smsKey, waKey, smsURL, waURL, sender, userID, password string) *MessageUsecase {
    return &MessageUsecase{
        SMSAPIKey:    smsKey,
        WhatsAppKey:  waKey,
        SMSBaseURL:   smsURL,
        WAAPIBaseURL: waURL,
        SenderID:     sender,
		UserID:   userID,
		Password: password,
        client:       &http.Client{Timeout: 10 * time.Second},
    }
}

// --- usecase ---
func (u *MessageUsecase) SendMessage(ctx context.Context, msg *domain.Message) error {
	switch msg.Channel {
	case "SMS":
		return u.sendSMS(ctx, msg)
	case "WHATSAPP":
		return u.sendWhatsApp(ctx, msg)
	default:
		return fmt.Errorf("unsupported channel: %s", msg.Channel)
	}
}

func (u *MessageUsecase) sendSMS(ctx context.Context, msg *domain.Message) error {
	start := time.Now()
	log.Printf("[SMS] Preparing to send message to %s via %s", msg.Recipient, u.SMSAPIKey)

	// Build multipart body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	_ = writer.WriteField("userid", u.UserID)
	_ = writer.WriteField("password", u.Password)
	_ = writer.WriteField("senderid", u.SenderID)
	_ = writer.WriteField("sendMethod", "quick")
	_ = writer.WriteField("msgType", "text")
	_ = writer.WriteField("msg", msg.Body)
	_ = writer.WriteField("mobile", msg.Recipient) // phone number with country code
	_ = writer.WriteField("duplicatecheck", "true")
	_ = writer.WriteField("output", "json")

	_ = writer.Close()

	// Send request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", u.SMSAPIKey, &buf)
	if err != nil {
		log.Printf("[SMS] Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[SMS] HTTP error sending to %s: %v", msg.Recipient, err)
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	duration := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[SMS] Failed sending to %s | Status: %d | Duration: %v | Response: %s",
			msg.Recipient, resp.StatusCode, duration, string(body))
		return fmt.Errorf("sms api error: %s", string(body))
	}

	log.Printf("[SMS] Successfully sent to %s | Duration: %v | Response: %s",
		msg.Recipient, duration, string(body))

	// Optional: parse response JSON if needed
	return nil
}



func (u *MessageUsecase) sendWhatsApp(_ context.Context, msg *domain.Message) error {
    payload := map[string]interface{}{
        "api_key":  u.WhatsAppKey,
        "to":       msg.Recipient,
        "message":  msg.Body,
    }

    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/send", u.WAAPIBaseURL), bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := u.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("failed to send WhatsApp, status: %d", resp.StatusCode)
    }
    return nil
}

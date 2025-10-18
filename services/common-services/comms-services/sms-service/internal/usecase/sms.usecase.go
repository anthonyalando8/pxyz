package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sms-service/internal/domain"
	"strings"
	"time"
)

type MessageUsecase struct {
    SMSAPIKey    string
    WhatsAppKey  string
    SMSBaseURL   string
    WAAPIBaseURL string
    SenderID     string
	WaSender   string
	UserID   string
	Password string
    client       *http.Client
}

func NewMessageUsecase(smsKey, waKey, waSender, smsURL, waURL, sender, userID, password string) *MessageUsecase {
    return &MessageUsecase{
        SMSAPIKey:    smsKey,
        WhatsAppKey:  waKey,
		WaSender: waSender,
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

	log.Printf("[SMS] Preparing to send message | Recipient=%s | SenderID=%s | UserID=%s | APIKeySet=%t",
		msg.Recipient, u.SenderID, u.UserID, u.SMSAPIKey != "")

	// Build form data
	form := url.Values{}
	form.Set("userid", u.UserID)
	form.Set("password", u.Password)
	form.Set("senderid", u.SenderID)
	form.Set("sendMethod", "quick")
	form.Set("msgType", "text")
	form.Set("msg", msg.Body)
	form.Set("mobile", msg.Recipient)
	form.Set("duplicatecheck", "true")
	form.Set("output", "json")

	// Send request
	apiURL := "https://smsportal.hostpinnacle.co.ke/SMSApi/send"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// If using API key authentication instead of userid/password
	if u.SMSAPIKey != "" {
		httpReq.Header.Set("apikey", u.SMSAPIKey)
	}

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
		log.Printf("[SMS] Failed sending | Recipient=%s | SenderID=%s | Status=%d | Duration=%v | Response=%s",
			msg.Recipient, u.SenderID, resp.StatusCode, duration, string(body))
		return fmt.Errorf("sms api error: %s", string(body))
	}

	log.Printf("[SMS] Successfully sent | Recipient=%s | SenderID=%s | Duration=%v | Response=%s",
		msg.Recipient, u.SenderID, duration, string(body))

	return nil
}


func (u *MessageUsecase) sendWhatsApp(ctx context.Context, msg *domain.Message) error {
	start := time.Now()

	log.Printf(
		"[WhatsApp] Preparing to send | Recipient=%s | Sender=%s | BaseURL=%s | TokenSet=%t",
		msg.Recipient, u.WaSender, u.WAAPIBaseURL, u.WhatsAppKey != "",
	)

	// Build payload
	payload := map[string]interface{}{
		"messageType": "text",
		"requestType": "POST",
		"token":       u.WhatsAppKey,
		"from":        u.WaSender,    // registered WA sender number
		"to":          msg.Recipient, // recipient phone number
		"text":        msg.Body,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[WhatsApp] Failed to marshal payload | Error=%v | Payload=%+v", err, payload)
		return fmt.Errorf("failed to marshal WhatsApp payload: %w", err)
	}

	// Build request
	url := fmt.Sprintf("%s/api/qr/rest/send_message", u.WAAPIBaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[WhatsApp] Failed to create request | Error=%v | URL=%s", err, url)
		return fmt.Errorf("failed to create WhatsApp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send
	resp, err := u.client.Do(req)
	if err != nil {
		log.Printf("[WhatsApp] HTTP error | Recipient=%s | Sender=%s | Error=%v", msg.Recipient, u.WaSender, err)
		return fmt.Errorf("WhatsApp HTTP error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	duration := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		log.Printf(
			"[WhatsApp] Failed sending | Recipient=%s | Sender=%s | Status=%d | Duration=%v | Response=%s",
			msg.Recipient, u.WaSender, resp.StatusCode, duration, string(respBody),
		)
		return fmt.Errorf("failed to send WhatsApp | Status=%d | Response=%s", resp.StatusCode, string(respBody))
	}

	log.Printf(
		"[WhatsApp] Successfully sent | Recipient=%s | Sender=%s | Duration=%v | Response=%s",
		msg.Recipient, u.WaSender, duration, string(respBody),
	)

	return nil
}


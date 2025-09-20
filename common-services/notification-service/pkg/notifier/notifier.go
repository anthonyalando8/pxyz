package notifier

import (
	"context"
	"log"
	"time"
	"notification-service/pkg/notifier/ws"
	"notification-service/pkg/template"

	emailpb "x/shared/genproto/emailpb"
	smswhatsapppb "x/shared/genproto/smswhatsapppb"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"
)

// NotificationType defines category of messages
type NotificationType string

const (
	Info          NotificationType = "INFO"
	Alert         NotificationType = "ALERT"
	Transactional NotificationType = "TRANSACTIONAL"
	Security      NotificationType = "SECURITY"

	OTP           NotificationType = "OTP"
	WELCOME		NotificationType = "WELCOME"
	GENERAL NotificationType = "GENERAL"
	EMAIL_UPDATE_OLD NotificationType = "EMAIL_UPDATE_OLD"
	EMAIL_UPDATE_NEW NotificationType = "EMAIL_UPDATE_NEW"
	PHONE_UPDATE NotificationType = "PHONE_UPDATE"
	PASSWORD_UPDATE NotificationType = "PASSWORD_UPDATE"
	TWOFA_ENABLED NotificationType = "2FA_ENABLED"
	TWOFA_DISABLED NotificationType = "2FA_DISABLED"
	TWOFA_BACKUP_CODE_REGN NotificationType = "2FA_BACKUP_CODE_REGN"
	KYC_SUBMITTED NotificationType = "KYC_SUBMITTED"
	KYC_REVIEWED NotificationType = "KYC_REVIEWED"
	WELCOME_ADMIN NotificationType = "WELCOME_ADMIN"
	PARTNER_USER_ADDED NotificationType = "PARTNER_USER_ADDED"
	WELCOME_NEW_PARTNER_USER NotificationType = "WELCOME_NEW_PARTNER_USER"
	PARTNER_CREATED NotificationType  = "PARTNER_CREATED"
	ACCOUNT_CREDITED NotificationType = "ACCOUNT_CREDITED"
	ACCOUNT_DEBITED NotificationType = "ACCOUNT_DEBITED"
	TRANSACTION_FAILED NotificationType = "TRANSACTION_FAILED"
)

// Message represents a generic notification payload
type Message struct {
    OwnerID     string
    OwnerType   string
    Recipient   string                 // kept for backward-compatibility (primary recipient)
    Recipients  map[string]string      // {"email": "...", "phone": "..."}
    Title       string
    Body        string
    Metadata    map[string]interface{}
    Channels    []string               // ["email", "sms", "ws"], if empty = all
    Type        NotificationType
    Data        any                    // used for template rendering
    Ctx         context.Context        // allow cancellation/timeouts
}


// Notifier holds all channel clients and template service
type Notifier struct {
	Email     *emailclient.EmailClient
	SMS       *smsclient.SMSClient
	WS        *ws.Manager
	Templates *template.TemplateService
}

// NewNotifier creates a new notifier with injected clients + template service
func NewNotifier(email *emailclient.EmailClient, sms *smsclient.SMSClient, ws *ws.Manager, tmpl *template.TemplateService) *Notifier {
	return &Notifier{
		Email:     email,
		SMS:       sms,
		WS:        ws,
		Templates: tmpl,
	}
}

// Notify sends a message to the requested channels
func (n *Notifier) Notify(msg *Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	targets := msg.Channels
	if len(targets) == 0 {
		targets = []string{"email", "sms", "ws"}
	}

	for _, ch := range targets {
		switch ch {
		case "email":
			if n.Email != nil {
				recipient := msg.Recipients["email"]
				if recipient == "" {
					recipient = msg.Recipient
				}
				if recipient == "" {
					log.Printf("⚠️ Skipping email notify (no recipient for %s_%s)", msg.OwnerType, msg.OwnerID)
					continue
				}

				body := msg.Body
				if n.Templates != nil {
					if rendered, err := n.Templates.Render("email", string(msg.Type), msg.Data); err == nil {
						body = rendered
					} else {
						log.Printf("⚠️ Email template render failed: %v", err)
					}
				}

				_, err := n.Email.SendEmail(ctx, &emailpb.SendEmailRequest{
					UserId:         msg.OwnerID,
					RecipientEmail: recipient,
					Subject:        msg.Title,
					Body:           body,
					Type:           string(msg.Type),
				})
				if err != nil {
					log.Printf("⚠️ Email notify failed for %s_%s (type=%s): %v",
						msg.OwnerType, msg.OwnerID, msg.Type, err)
				}
			}

		case "sms", "whatsapp":
			var client = n.SMS
			var channel smswhatsapppb.Channel

			if ch == "sms" {
				channel = smswhatsapppb.Channel_SMS
			} else {
				channel = smswhatsapppb.Channel_WHATSAPP
			}

			if client != nil {
				recipient := msg.Recipients["phone"]
				if recipient == "" {
					recipient = msg.Recipient
				}
				if recipient == "" {
					log.Printf("⚠️ Skipping %s notify (no recipient for %s_%s)", ch, msg.OwnerType, msg.OwnerID)
					continue
				}

				body := msg.Body
				if n.Templates != nil {
					if rendered, err := n.Templates.Render(ch, string(msg.Type), msg.Data); err == nil {
						body = rendered
					} else {
						log.Printf("⚠️ %s template render failed: %v", ch, err)
					}
				}

				_, err := client.SendMessage(ctx, &smswhatsapppb.SendMessageRequest{
					UserId:    msg.OwnerID,
					Recipient: recipient,
					Body:      body,
					Channel:   channel,
					Type:      string(msg.Type),
				})
				if err != nil {
					log.Printf("⚠️ %s notify failed for %s_%s (type=%s): %v",
						ch, msg.OwnerType, msg.OwnerID, msg.Type, err)
				}
			}

		case "ws":
			if n.WS != nil {
				n.WS.Send(msg.OwnerType, msg.OwnerID, msg)
			}
		}
	}
}


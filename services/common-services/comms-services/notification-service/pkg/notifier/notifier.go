package notifier

import (
	"context"
	"log"
	"notification-service/internal/domain"
	"notification-service/pkg/notifier/ws"
	"notification-service/pkg/template"
	"time"

	emailpb "x/shared/genproto/emailpb"
	smswhatsapppb "x/shared/genproto/smswhatsapppb"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"
)

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
func (n *Notifier) Notify(msg *domain.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 🔍 Log full notification message context
	log.Printf("[Notifier] Dispatching notification | OwnerType=%s | OwnerID=%s | Type=%s | Channels=%v | Data=%+v",
		msg.OwnerType, msg.OwnerID, msg.Type, msg.Channels, msg.Data)

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



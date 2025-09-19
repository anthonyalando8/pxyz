package notifier

import (
	"context"
	"log"
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
)

// Message represents a generic notification payload
type Message struct {
	OwnerID    string
	OwnerType  string
	Recipient  string                 // email/phone number
	Title      string
	Body       string
	Metadata   map[string]interface{}
	Channels   []string               // ["email", "sms", "ws"], if empty = all
	Type       NotificationType
	Data       any                    // used for template rendering
	Ctx        context.Context        // allow cancellation/timeouts
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
	ctx := msg.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	targets := msg.Channels
	if len(targets) == 0 {
		targets = []string{"email", "sms", "ws"}
	}

	for _, ch := range targets {
		switch ch {
		case "email":
			if n.Email != nil {
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
					RecipientEmail: msg.Recipient,
					Subject:        msg.Title,
					Body:           body,
					Type:           string(msg.Type),
				})
				if err != nil {
					log.Printf("⚠️ Email notify failed for %s_%s (type=%s): %v",
						msg.OwnerType, msg.OwnerID, msg.Type, err)
				}
			}

		case "sms":
			if n.SMS != nil {
				body := msg.Body
				if n.Templates != nil {
					if rendered, err := n.Templates.Render("sms", string(msg.Type), msg.Data); err == nil {
						body = rendered
					} else {
						log.Printf("⚠️ SMS template render failed: %v", err)
					}
				}
				_, err := n.SMS.SendMessage(ctx, &smswhatsapppb.SendMessageRequest{
					UserId:    msg.OwnerID,
					Recipient: msg.Recipient,
					Body:      body,
					Channel:   smswhatsapppb.Channel_SMS,
					Type:      string(msg.Type),
				})
				if err != nil {
					log.Printf("⚠️ SMS notify failed for %s_%s (type=%s): %v",
						msg.OwnerType, msg.OwnerID, msg.Type, err)
				}
			}

		case "ws":
			if n.WS != nil {
				n.WS.Send(msg.OwnerType, msg.OwnerID, msg)
			}
		}
	}
}

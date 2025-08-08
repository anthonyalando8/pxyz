package domain
import "time"

type EmailLog struct {
	ID             int64      `json:"id"`
	UserID         *int64     `json:"user_id,omitempty"`
	Subject        *string    `json:"subject,omitempty"`
	RecipientEmail string     `json:"recipient_email"`
	EmailType      string     `json:"email_type"`       // otp, password-reset, etc.
	DeliveryStatus string     `json:"delivery_status"`  // sent, failed
	SentAt         time.Time  `json:"sent_at"`
}

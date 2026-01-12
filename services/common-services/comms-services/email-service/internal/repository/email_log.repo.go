package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailLog struct {
	ID             string      `json:"id"`
	UserID         string     `json:"user_id,omitempty"`
	Subject        string    `json:"subject,omitempty"`
	RecipientEmail string     `json:"recipient_email"`
	Type      string     `json:"email_type"`       // otp, password-reset, etc.
	Status string     `json:"delivery_status"`  // sent, failed
	SentAt         time.Time  `json:"sent_at"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	Duration       time.Duration      `json:"duration"`         // in milliseconds
	CreatedAt      time.Time  `json:"created_at"`
}

type EmailLogRepo struct {
	db *pgxpool.Pool
}

func NewEmailLogRepo(db *pgxpool.Pool) *EmailLogRepo {
	return &EmailLogRepo{db: db}
}

func (r *EmailLogRepo) LogEmail(ctx context.Context, log EmailLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO email_logs (id, user_id, subject, recipient_email, type, status, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,log.ID, log.UserID, log.Subject, log.RecipientEmail, log.Type, log.Status, log.SentAt)
	return err
}

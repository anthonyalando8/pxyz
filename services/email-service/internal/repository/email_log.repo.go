package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailLog struct {
	ID             string 
	UserID         string
	Subject        string
	RecipientEmail string
	Type           string
	Status         string
	SentAt         time.Time
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

package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OTP struct {
	ID         string
	UserID     string
	Code       string
	Channel    string
	Purpose    string
	IssuedAt   time.Time
	ValidUntil time.Time
	IsVerified bool
	IsActive   bool
	UpdatedAt  time.Time
}

type OTPRepo struct {
	db *pgxpool.Pool
}

func NewOTPRepo(db *pgxpool.Pool) *OTPRepo {
	return &OTPRepo{db: db}
}

func (r *OTPRepo) Create(ctx context.Context, o *OTP) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_otps (id, user_id, code, channel, purpose, issued_at, valid_until, is_verified, is_active, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, o.ID, o.UserID, o.Code, o.Channel, o.Purpose, o.IssuedAt, o.ValidUntil, o.IsVerified, o.IsActive, o.UpdatedAt)
	return err
}

func (r *OTPRepo) GetActiveByUserAndPurpose(ctx context.Context, userID int64, purpose string) (*OTP, error) {
	var o OTP
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, code, channel, purpose, issued_at, valid_until, is_verified, is_active, updated_at
		FROM user_otps
		WHERE user_id=$1 AND purpose=$2 AND is_active=TRUE
		ORDER BY issued_at DESC
		LIMIT 1
	`, userID, purpose).Scan(&o.ID, &o.UserID, &o.Code, &o.Channel, &o.Purpose, &o.IssuedAt, &o.ValidUntil, &o.IsVerified, &o.IsActive, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OTPRepo) VerifyAndInvalidate(ctx context.Context, userID int64, purpose, code string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var id int64
	var validUntil time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, valid_until FROM user_otps
		WHERE user_id=$1 AND purpose=$2 AND code=$3 AND is_active=TRUE AND is_verified=FALSE
		LIMIT 1
	`, userID, purpose, code).Scan(&id, &validUntil)
	if err != nil {
		return false, err
	}

	if time.Now().After(validUntil) {
		// expired -> mark inactive
		_, _ = tx.Exec(ctx, `UPDATE user_otps SET is_active=FALSE, updated_at=NOW() WHERE id=$1`, id)
		_ = tx.Commit(ctx)
		return false, nil
	}

	_, err = tx.Exec(ctx, `UPDATE user_otps SET is_verified=TRUE, is_active=FALSE, updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

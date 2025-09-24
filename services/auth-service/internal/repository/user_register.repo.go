package repository

import (
	"auth-service/internal/domain"
	"context"
	"fmt"
	"x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
)

func (r *UserRepository) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	insertQuery := `
		INSERT INTO users (
			id, email, phone, password_hash, first_name, last_name,
			is_email_verified, is_phone_verified, signup_stage,
			account_status, account_type, account_restored,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,
			'active',$10,FALSE,
			NOW(),NOW()
		)
		ON CONFLICT (email) 
		DO UPDATE SET 
			-- no-op update just to allow RETURNING
			email = EXCLUDED.email
		RETURNING id, email, phone, password_hash, first_name, last_name,
		          is_email_verified, is_phone_verified, signup_stage,
		          account_status, account_type, account_restored,
		          created_at, updated_at
	`

	var saved domain.User
	err := r.db.QueryRow(
		ctx, insertQuery,
		user.ID,
		nullOrNilPtr(user.Email),
		nullOrNilPtr(user.Phone),
		nullOrNilPtr(user.PasswordHash),
		nullOrNilPtr(user.FirstName),
		nullOrNilPtr(user.LastName),
		user.IsEmailVerified,
		user.IsPhoneVerified,
		coalesceString(user.SignupStage, "email_or_phone_submitted"),
		coalesceString(user.AccountType, "password"),
	).Scan(
		&saved.ID, &saved.Email, &saved.Phone, &saved.PasswordHash,
		&saved.FirstName, &saved.LastName,
		&saved.IsEmailVerified, &saved.IsPhoneVerified, &saved.SignupStage,
		&saved.AccountStatus, &saved.AccountType, &saved.AccountRestored,
		&saved.CreatedAt, &saved.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// If we hit conflict, we need to decide what error to return based on signup stage
	if saved.ID != user.ID { // means conflict occurred
		switch saved.SignupStage {
		case "email_or_phone_submitted":
			return &saved, NewSignupError(saved.SignupStage, "verify_otp")
		case "otp_verified":
			return &saved, NewSignupError(saved.SignupStage, "set_password")
		case "password_set", "complete":
			if (user.Email != nil && saved.Email != nil) && (*saved.Email == *user.Email) {
				return &saved, xerrors.ErrEmailAlreadyInUse
			}
			if (user.Phone != nil && saved.Phone != nil) && (*saved.Phone == *user.Phone) {
				return &saved, xerrors.ErrPhoneAlreadyInUse
			}
			return &saved, xerrors.ErrUserAlreadyExists
		}
		return &saved, xerrors.ErrUserAlreadyExists
	}

	return &saved, nil
}



// Helper for default strings
func coalesceString(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
func nullOrNilPtr(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}



// VerifyEmail sets is_email_verified = TRUE for a given user
// and updates signup_stage if appropriate
func (r *UserRepository) VerifyEmail(ctx context.Context, userID string) error {
	query := `
		UPDATE users
		SET is_email_verified = TRUE,
		    signup_stage = CASE
		       WHEN signup_stage = 'email_or_phone_submitted' THEN 'otp_verified'
		       ELSE signup_stage
		    END,
		    updated_at = NOW()
		WHERE id = $1
		  AND account_status != 'deleted'
	`

	cmdTag, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to verify email for user %s: %w", userID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no active user found with id %s", userID)
	}
	return nil
}

func (r *UserRepository) GetEmailVerificationStatus(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT is_email_verified
		FROM users
		WHERE id = $1
		  AND account_status != 'deleted'
	`

	var isVerified bool
	err := r.db.QueryRow(ctx, query, userID).Scan(&isVerified)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, xerrors.ErrNotFound
		}
		return false, fmt.Errorf("failed to fetch email verification status for user %s: %w", userID, err)
	}

	return isVerified, nil
}


// VerifyPhone sets is_phone_verified = TRUE for a given user
// and updates signup_stage if appropriate
func (r *UserRepository) VerifyPhone(ctx context.Context, userID string) error {
	query := `
		UPDATE users
		SET is_phone_verified = TRUE,
		    signup_stage = CASE
		       WHEN signup_stage = 'email_or_phone_submitted' THEN 'otp_verified'
		       ELSE signup_stage
		    END,
		    updated_at = NOW()
		WHERE id = $1
		  AND account_status != 'deleted'
	`

	cmdTag, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to verify phone for user %s: %w", userID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no active user found with id %s", userID)
	}
	return nil
}

func (r *UserRepository) GetPhoneVerificationStatus(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT is_phone_verified
		FROM users
		WHERE id = $1
		  AND account_status != 'deleted'
	`

	var isVerified bool
	err := r.db.QueryRow(ctx, query, userID).Scan(&isVerified)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, xerrors.ErrNotFound
		}
		return false, fmt.Errorf("failed to fetch phone verification status for user %s: %w", userID, err)
	}

	return isVerified, nil
}


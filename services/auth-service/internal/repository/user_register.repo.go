package repository

import (
	"auth-service/internal/domain"
	"x/shared/utils/errors"
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

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

	// If insert fails due to duplicate
	if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
		var existing domain.User
		checkQuery := `
			SELECT id, email, phone, password_hash, first_name, last_name,
			       is_email_verified, is_phone_verified, signup_stage,
			       account_status, account_type, account_restored,
			       created_at, updated_at
			FROM users
			WHERE (email = $1 OR phone = $2)
			  AND account_status != 'deleted'
			LIMIT 1
		`
		errCheck := r.db.QueryRow(ctx, checkQuery, user.Email, user.Phone).Scan(
			&existing.ID, &existing.Email, &existing.Phone, &existing.PasswordHash,
			&existing.FirstName, &existing.LastName,
			&existing.IsEmailVerified, &existing.IsPhoneVerified, &existing.SignupStage,
			&existing.AccountStatus, &existing.AccountType, &existing.AccountRestored,
			&existing.CreatedAt, &existing.UpdatedAt,
		)
		if errCheck != nil {
			return nil, errCheck
		}

		// Return SignupError depending on stage
		switch existing.SignupStage {
		case "email_or_phone_submitted":
			return &existing, NewSignupError(existing.SignupStage, "verify_otp")
		case "otp_verified":
			return &existing, NewSignupError(existing.SignupStage, "set_password")
		case "password_set", "complete":
			if strings.Contains(pgErr.Message, "email") {
				return &existing, xerrors.ErrEmailAlreadyInUse
			}
			if strings.Contains(pgErr.Message, "phone") {
				return &existing, xerrors.ErrPhoneAlreadyInUse
			}
			return &existing, xerrors.ErrUserAlreadyExists
		}
		return &existing, xerrors.ErrUserAlreadyExists
	}

	if err != nil {
		return nil, err
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

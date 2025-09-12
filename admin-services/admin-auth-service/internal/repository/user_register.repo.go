package repository

import (
	"admin-auth-service/internal/domain"
	"x/shared/utils/errors"
	"context"
	"fmt"

)

func (r *UserRepository) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	insertQuery := `
		INSERT INTO users (
			id, email, phone, password_hash, first_name, last_name,
			is_email_verified, is_phone_verified, is_temp_pass,
			account_status, account_type,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,
			'active',$10,
			NOW(),NOW()
		)
		ON CONFLICT (email) 
		DO UPDATE SET 
			email = EXCLUDED.email
		RETURNING id, email, phone, password_hash, first_name, last_name,
		          is_email_verified, is_phone_verified, is_temp_pass,
		          account_status, account_type,
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
		user.IsTempPass,
		coalesceString(user.AccountType, "password"),
	).Scan(
		&saved.ID, &saved.Email, &saved.Phone, &saved.PasswordHash,
		&saved.FirstName, &saved.LastName,
		&saved.IsEmailVerified, &saved.IsPhoneVerified, &saved.IsTempPass,
		&saved.AccountStatus, &saved.AccountType,
		&saved.CreatedAt, &saved.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Conflict handling simplified (no signup_stage anymore)
	if saved.ID != user.ID {
		if (user.Email != nil && saved.Email != nil) && (*saved.Email == *user.Email) {
			return &saved, xerrors.ErrEmailAlreadyInUse
		}
		if (user.Phone != nil && saved.Phone != nil) && (*saved.Phone == *user.Phone) {
			return &saved, xerrors.ErrPhoneAlreadyInUse
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


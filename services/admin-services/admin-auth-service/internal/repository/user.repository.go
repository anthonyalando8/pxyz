package repository

import (
	"admin-auth-service/internal/domain"
	"context"
	"errors"
	"fmt"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func NewSignupError(stage, next string) *xerrors.SignupError {
	return &xerrors.SignupError{Stage: stage, NextStage: next}
}


func (r *UserRepository) UpdateEmail(ctx context.Context, userID, newEmail string) error {
	query := `
		UPDATE users
		SET 
			changed_emails = COALESCE(changed_emails, '[]'::jsonb) || 
				jsonb_build_array(
					jsonb_build_object(
						'email', email,
						'date_added', NOW()
					)
				),
			email = $1,
			updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, newEmail, userID)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, hash string) error {
	query := `UPDATE users SET password_hash=$1, updated_at=NOW()
			  WHERE id=$2 AND account_status!='deleted'`
	_, err := r.db.Exec(ctx, query, hash, userID)
	return err
}


// UpdateName sets first and last name and advances signup_stage if appropriate
func (r *UserRepository) UpdateName(ctx context.Context, userID, firstName, lastName string) error {
	query := `
		UPDATE users
		SET first_name = $1,
		    last_name  = $2,
		    updated_at = NOW()
		WHERE id = $3
		  AND account_status != 'deleted'
	`

	cmdTag, err := r.db.Exec(ctx, query, firstName, lastName, userID)
	if err != nil {
		return fmt.Errorf("failed to update name for user %s: %w", userID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no active user found with id %s", userID)
	}
	return nil
}


func (r *UserRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	const q = `
		SELECT 
			id,
			email,
			phone,
			password_hash,
			first_name,
			last_name,
			is_email_verified,
			is_phone_verified,
			is_temp_pass,
			account_status,
			account_type,
			created_at,
			updated_at
		FROM users
		WHERE id = $1 AND account_status != 'deleted'
		LIMIT 1
	`

	var user domain.User
	err := r.db.QueryRow(ctx, q, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.IsEmailVerified,
		&user.IsPhoneVerified,
		&user.IsTempPass,
		&user.AccountStatus,
		&user.AccountType,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}


func (r *UserRepository) UpdatePhone(ctx context.Context, userID, newPhone string, isPhoneVerified bool) error {
	query := `
		UPDATE users
		SET 
			phone = $1::VARCHAR,
			is_phone_verified = 
				CASE WHEN phone IS DISTINCT FROM $1::VARCHAR 
					THEN $3 
					ELSE is_phone_verified 
				END,
			changed_phones = COALESCE(changed_phones, '[]'::jsonb) || 
				CASE 
					WHEN phone IS NOT NULL 
					AND phone <> '' 
					AND phone <> $1::VARCHAR 
					THEN jsonb_build_object('phone', phone, 'changed_at', NOW())
					ELSE '[]'::jsonb
				END,
			updated_at = NOW()
		WHERE id = $2;
	`

	_, err := r.db.Exec(ctx, query, newPhone, userID, isPhoneVerified)
	return err
}


func (r *UserRepository) DeleteUser(ctx context.Context, userID string) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		} else {
			tx.Commit(ctx)
		}
	}()

	// Remove sessions
	if _, err = tx.Exec(ctx, `
		DELETE FROM sessions WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete sessions: %w", err)
	}

	// Finally, remove the user itself
	if _, err = tx.Exec(ctx, `
		DELETE FROM users WHERE id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
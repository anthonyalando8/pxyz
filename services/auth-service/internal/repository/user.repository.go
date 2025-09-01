package repository

import (
	"auth-service/internal/domain"
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

func getString(v *string) string {
	if v != nil {
		return *v
	}
	return ""
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

func (r *UserRepository) SetPendingEmail(ctx context.Context, userID, newEmail string) error {
	query := `
		UPDATE users
		SET 
			pending_email = $1,
			pending_email_expires_at = NOW() + interval '15 minutes',
			updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, newEmail, userID)
	return err
}


func (r *UserRepository) GetAndClearPendingEmail(ctx context.Context, userID string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	const selectQ = `
		SELECT pending_email
		FROM users
		WHERE id = $1 AND account_status != 'deleted'
		LIMIT 1
	`
	var pendingEmail *string
	err = tx.QueryRow(ctx, selectQ, userID).Scan(&pendingEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", xerrors.ErrUserNotFound
		}
		return "", err
	}

	// If no pending email, just commit and return empty
	if pendingEmail == nil {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return "", nil
	}

	// Clear pending email after retrieval
	const clearQ = `
		UPDATE users
		SET pending_email = NULL,
		    pending_email_expires_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := tx.Exec(ctx, clearQ, userID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return *pendingEmail, nil
}


func (r *UserRepository) UpdatePassword(ctx context.Context, userID, hash string) error {
	query := `UPDATE users SET password_hash=$1, updated_at=NOW()
			  WHERE id=$2 AND account_status!='deleted'`
	_, err := r.db.Exec(ctx, query, hash, userID)
	return err
}

// UpdatePassword sets a new password and advances signup_stage if appropriate
func (r *UserRepository) UpdatePasswordWithStage(ctx context.Context, userID, newPassword string) error {
	query := `
		UPDATE users
		SET password_hash = $1,
		    signup_stage = CASE
		       WHEN signup_stage = 'otp_verified' THEN 'password_set'
		       ELSE signup_stage
		    END,
		    updated_at = NOW()
		WHERE id = $2
		  AND account_status != 'deleted'
	`

	cmdTag, err := r.db.Exec(ctx, query, newPassword, userID)
	if err != nil {
		return fmt.Errorf("failed to update password for user %s: %w", userID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no active user found with id %s", userID)
	}
	return nil
}

// UpdateName sets first and last name and advances signup_stage if appropriate
func (r *UserRepository) UpdateName(ctx context.Context, userID, firstName, lastName string) error {
	query := `
		UPDATE users
		SET first_name = $1,
		    last_name  = $2,
		    signup_stage = CASE
		       WHEN signup_stage = 'password_set' THEN 'complete'
		       ELSE signup_stage
		    END,
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
			signup_stage,
			account_status,
			account_type,
			account_restored,
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
		&user.SignupStage,
		&user.AccountStatus,
		&user.AccountType,
		&user.AccountRestored,
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


func (r *UserRepository) UpdatePhone(ctx context.Context, userID, newPhone string) error {
	query := `
		UPDATE users
		SET 
			phone = $1,
			is_phone_verified = false,
			changed_phones = COALESCE(changed_phones, '[]'::jsonb) || jsonb_build_object(
				'phone', phone,
				'changed_at', NOW()
			),
			updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, newPhone, userID)
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

	// Remove user permissions
	if _, err = tx.Exec(ctx, `
		DELETE FROM user_permissions WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete user_permissions: %w", err)
	}

	// Remove user roles
	if _, err = tx.Exec(ctx, `
		DELETE FROM user_roles WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete user_roles: %w", err)
	}

	// Remove email logs
	if _, err = tx.Exec(ctx, `
		DELETE FROM email_logs WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete email_logs: %w", err)
	}

	// Remove sessions
	if _, err = tx.Exec(ctx, `
		DELETE FROM sessions WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete sessions: %w", err)
	}

	// Remove OTPs
	if _, err = tx.Exec(ctx, `
		DELETE FROM user_otps WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete user_otps: %w", err)
	}

	// Remove OAuth accounts
	if _, err = tx.Exec(ctx, `
		DELETE FROM oauth_accounts WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete oauth_accounts: %w", err)
	}

	// Remove account deletion requests
	if _, err = tx.Exec(ctx, `
		DELETE FROM account_deletion_requests WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete account_deletion_requests: %w", err)
	}

	// Finally, remove the user itself
	if _, err = tx.Exec(ctx, `
		DELETE FROM users WHERE id = $1
	`, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
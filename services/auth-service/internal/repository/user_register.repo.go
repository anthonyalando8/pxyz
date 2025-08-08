package repository

import (
	"auth-service/internal/domain"
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func (r *UserRepository) CreateUser(ctx context.Context, user *domain.User) error {
	var email, phone, password, firstName, lastName interface{}
	if getString(user.Email) != "" {
		email = user.Email
	}
	if getString(user.Phone) != "" {
		phone = user.Phone
	}
	if getString(user.PasswordHash) != "" {
		password = user.PasswordHash
	}
	if getString(user.FirstName) != "" {
		firstName = user.FirstName
	}
	if getString(user.LastName) != "" {
		lastName = user.LastName
	}

	insertQuery := `
		INSERT INTO users (
			id, email, phone, password_hash, is_verified, first_name, last_name,
			account_status, has_password, account_restored, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,'active',$8,FALSE,NOW(),NOW()
		)`

	_, err := r.db.Exec(ctx, insertQuery,
		user.ID, email, phone, password,
		user.IsVerified, firstName, lastName,
		user.HasPassword,
	)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			// Possible unique constraint violation → check if the user is deleted
			var existingID string
			checkQuery := `
				SELECT id FROM users
				WHERE (email = $1 OR phone = $2)
				  AND account_status = 'deleted'
				LIMIT 1
			`
			errCheck := r.db.QueryRow(ctx, checkQuery, getString(user.Email), getString(user.Phone)).Scan(&existingID)
			if errCheck == nil {
				// Reactivate deleted account
				updateQuery := `
					UPDATE users
					SET email = COALESCE($2, email),
						phone = COALESCE($3, phone),
						password_hash = COALESCE($4, password_hash),
						is_verified = $5,
						first_name = COALESCE($6, first_name),
						last_name = COALESCE($7, last_name),
						has_password = $8,
						account_status = 'active',
						account_restored = TRUE,
						updated_at = NOW()
					WHERE id = $1
				`
				_, errUpdate := r.db.Exec(ctx, updateQuery,
					existingID, email, phone, password,
					user.IsVerified, firstName, lastName,
					user.HasPassword,
				)
				if errUpdate != nil {
					return fmt.Errorf("failed to restore deleted user: %w", errUpdate)
				}
				return nil
			}

			// Otherwise → active duplicate
			if strings.Contains(pgErr.Message, "email") {
				return fmt.Errorf("email already in use")
			}
			if strings.Contains(pgErr.Message, "phone") {
				return fmt.Errorf("phone number already in use")
			}
			return fmt.Errorf("user already exists with provided details")
		}

		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}


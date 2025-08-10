package repository

import (
	"auth-service/internal/domain"
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func getString(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

func (r *UserRepository) UpdateEmail(ctx context.Context, userID, newEmail string) error {
	query := `UPDATE users SET email=$1, updated_at=NOW() WHERE id=$2`
	_, err := r.db.Exec(ctx, query, newEmail, userID)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, newPassword string) error {
	query := `UPDATE users SET password=$1, updated_at=NOW() WHERE id=$2`
	_, err := r.db.Exec(ctx, query, newPassword, userID)
	return err
}
func (r *UserRepository) UpdateName(ctx context.Context, userID, firstName, lastName string) error {
	query := `UPDATE users SET first_name=$1, last_name=$2, updated_at=NOW() WHERE id=$3`
	_, err := r.db.Exec(ctx, query, firstName, lastName, userID)
	return err
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
			is_verified, 
			account_status, 
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
		&user.IsVerified,
		&user.AccountStatus,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound // No user found
		}
		return nil, err
	}

	return &user, nil
}

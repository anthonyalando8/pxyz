package repository

import (
	"auth-service/internal/domain"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrUserNotFound = errors.New("user not found")

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, email, phone, password_hash, first_name, last_name,
		       is_verified, account_status, has_password, account_restored,
		       created_at, updated_at
		FROM users
		WHERE email = $1
		  AND account_status != 'deleted'
		LIMIT 1
	`

	var u domain.User
	err := r.db.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.Phone, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.IsVerified,
		&u.AccountStatus, &u.HasPassword, &u.AccountRestored,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (r *UserRepository) GetUserByIdentifier(ctx context.Context, identifier string) (*domain.User, error) {
	const q = `
		SELECT id, email, phone, password_hash, first_name, last_name,
		       is_verified, account_status, has_password, account_restored,
		       created_at, updated_at
		FROM users
		WHERE (email = $1 OR phone = $1)
		  AND account_status != 'deleted'
		LIMIT 1
	`

	var u domain.User
	err := r.db.QueryRow(ctx, q, identifier).Scan(
		&u.ID, &u.Email, &u.Phone, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.IsVerified,
		&u.AccountStatus, &u.HasPassword, &u.AccountRestored,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}


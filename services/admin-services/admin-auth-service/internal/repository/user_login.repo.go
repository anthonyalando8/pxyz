package repository

import (
	"admin-auth-service/internal/domain"
	"context"
	"errors"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
)

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
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
		WHERE email = $1
		  AND account_status != 'deleted'
		LIMIT 1
	`

	var u domain.User
	err := r.db.QueryRow(ctx, q, email).Scan(
		&u.ID,
		&u.Email,
		&u.Phone,
		&u.PasswordHash,
		&u.FirstName,
		&u.LastName,
		&u.IsEmailVerified,
		&u.IsPhoneVerified,
		&u.IsTempPass,
		&u.AccountStatus,
		&u.AccountType,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, xerrors.ErrUserNotFound
	}
	return &u, err
}

func (r *UserRepository) GetUserByIdentifier(ctx context.Context, identifier string) (*domain.User, error) {
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
		WHERE (email = $1 OR phone = $1)
		  AND account_status != 'deleted'
		LIMIT 1
	`

	var u domain.User
	err := r.db.QueryRow(ctx, q, identifier).Scan(
		&u.ID,
		&u.Email,
		&u.Phone,
		&u.PasswordHash,
		&u.FirstName,
		&u.LastName,
		&u.IsEmailVerified,
		&u.IsPhoneVerified,
		&u.IsTempPass,
		&u.AccountStatus,
		&u.AccountType,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, xerrors.ErrUserNotFound
	}
	return &u, err
}



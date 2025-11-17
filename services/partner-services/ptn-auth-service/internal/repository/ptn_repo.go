package repository

import (
	"context"
	"errors"
	"fmt"
	"ptn-auth-service/internal/domain"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
)

func (r *UserRepository) GetUsersByPartnerID(ctx context.Context, partnerID string) ([]domain.User, error) {
	const q = `
		SELECT 
			id,
			partner_id,
			email,
			phone,
			password_hash,
			first_name,
			last_name,
			is_email_verified,
			is_phone_verified,
			is_temp_pass,
			role,
			account_status,
			account_type,
			created_at,
			updated_at
		FROM users
		WHERE partner_id = $1
		  AND account_status != 'deleted'
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, q, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		err := rows.Scan(
			&u.ID,
			&u.PartnerID,
			&u.Email,
			&u.Phone,
			&u.PasswordHash,
			&u.FirstName,
			&u.LastName,
			&u.IsEmailVerified,
			&u.IsPhoneVerified,
			&u.IsTempPass,
			&u.Role,
			&u.AccountStatus,
			&u.AccountType,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if len(users) == 0 {
		return nil, xerrors.ErrUserNotFound
	}

	return users, nil
}


func (r *UserRepository) GetUsersByPartnerIDWithPagination(ctx context.Context, partnerID string, limit, offset int) ([]domain.User, int64, error) {
	const countQuery = `
		SELECT COUNT(*) 
		FROM users 
		WHERE partner_id = $1 
		  AND account_status != 'deleted'
	`
	
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, partnerID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	const q = `
		SELECT 
			id, partner_id, email, phone, password_hash,
			first_name, last_name, is_email_verified, is_phone_verified,
			is_temp_pass, role, account_status, account_type,
			created_at, updated_at
		FROM users
		WHERE partner_id = $1
		  AND account_status != 'deleted'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, q, partnerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		err := rows.Scan(
			&u.ID, &u.PartnerID, &u.Email, &u.Phone, &u.PasswordHash,
			&u.FirstName, &u.LastName, &u.IsEmailVerified, &u.IsPhoneVerified,
			&u.IsTempPass, &u.Role, &u.AccountStatus, &u.AccountType,
			&u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	return users, total, nil
}

func (r *UserRepository) GetPartnerUserStats(ctx context.Context, partnerID string) (*domain.PartnerUserStats, error) {
	const q = `
		SELECT 
			COUNT(*) as total_users,
			COUNT(*) FILTER (WHERE account_status = 'active') as active_users,
			COUNT(*) FILTER (WHERE account_status = 'suspended') as suspended_users,
			COUNT(*) FILTER (WHERE is_email_verified = true) as verified_users,
			COUNT(*) FILTER (WHERE role = 'partner_admin') as admin_users,
			COUNT(*) FILTER (WHERE role = 'partner_user') as regular_users
		FROM users
		WHERE partner_id = $1
		  AND account_status != 'deleted'
	`

	var stats domain.PartnerUserStats
	err := r.db.QueryRow(ctx, q, partnerID).Scan(
		&stats.TotalUsers,
		&stats.ActiveUsers,
		&stats.SuspendedUsers,
		&stats.VerifiedUsers,
		&stats.AdminUsers,
		&stats.RegularUsers,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get partner stats: %w", err)
	}

	stats.PartnerID = partnerID
	return &stats, nil
}

func (r *UserRepository) UpdateUserStatus(ctx context.Context, userID, partnerID, status string) error {
	// Verify user belongs to partner before updating
	const q = `
		UPDATE users
		SET account_status = $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND partner_id = $3
		  AND account_status != 'deleted'
	`

	cmdTag, err := r.db.Exec(ctx, q, status, userID, partnerID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdateUserRole(ctx context.Context, userID, partnerID, role string) error {
	const q = `
		UPDATE users
		SET role = $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND partner_id = $3
		  AND account_status != 'deleted'
	`

	cmdTag, err := r.db.Exec(ctx, q, role, userID, partnerID)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) SearchPartnerUsers(ctx context.Context, partnerID, searchTerm string, limit, offset int) ([]domain.User, error) {
	const q = `
		SELECT 
			id, partner_id, email, phone, password_hash,
			first_name, last_name, is_email_verified, is_phone_verified,
			is_temp_pass, role, account_status, account_type,
			created_at, updated_at
		FROM users
		WHERE partner_id = $1
		  AND account_status != 'deleted'
		  AND (
		      email ILIKE '%' || $2 || '%'
		      OR phone ILIKE '%' || $2 || '%'
		      OR first_name ILIKE '%' || $2 || '%'
		      OR last_name ILIKE '%' || $2 || '%'
		  )
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Query(ctx, q, partnerID, searchTerm, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		err := rows.Scan(
			&u.ID, &u.PartnerID, &u.Email, &u.Phone, &u.PasswordHash,
			&u.FirstName, &u.LastName, &u.IsEmailVerified, &u.IsPhoneVerified,
			&u.IsTempPass, &u.Role, &u.AccountStatus, &u.AccountType,
			&u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, nil
}

func (r *UserRepository) GetPartnerUserByEmail(ctx context.Context, partnerID, email string) (*domain.User, error) {
	const q = `
		SELECT 
			id, partner_id, email, phone, password_hash,
			first_name, last_name, is_email_verified, is_phone_verified,
			is_temp_pass, role, account_status, account_type,
			created_at, updated_at
		FROM users
		WHERE partner_id = $1
		  AND email = $2
		  AND account_status != 'deleted'
		LIMIT 1
	`

	var user domain.User
	err := r.db.QueryRow(ctx, q, partnerID, email).Scan(
		&user.ID, &user.PartnerID, &user.Email, &user.Phone, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.IsEmailVerified, &user.IsPhoneVerified,
		&user.IsTempPass, &user.Role, &user.AccountStatus, &user.AccountType,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) BulkUpdateUserStatus(ctx context.Context, partnerID string, userIDs []string, status string) error {
	const q = `
		UPDATE users
		SET account_status = $1,
		    updated_at = NOW()
		WHERE partner_id = $2
		  AND id = ANY($3)
		  AND account_status != 'deleted'
	`

	cmdTag, err := r.db.Exec(ctx, q, status, partnerID, userIDs)
	if err != nil {
		return fmt.Errorf("failed to bulk update user status: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return xerrors.ErrUserNotFound
	}
	return nil
}
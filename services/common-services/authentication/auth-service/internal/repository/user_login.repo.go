package repository

import (
	"auth-service/internal/domain"
	"context"
	"errors"
	"fmt"
	"strconv"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
)

// ============================================
// SCAN HELPERS
// ============================================

// users table only
func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var userID int64
	err := row.Scan(
		&userID,
		&u.AccountStatus,
		&u.AccountType,
		&u.AccountRestored,
		&u.Consent,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, xerrors.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.ID = strconv.FormatInt(userID, 10)
	return &u, nil
}

// credentials table only
func scanCredential(row pgx.Row) (*domain.UserCredential, error) {
	var c domain.UserCredential
	var credID, userID int64
	err := row.Scan(
		&credID,
		&userID,
		&c.Email,
		&c.Phone,
		&c.PasswordHash,
		&c.IsEmailVerified,
		&c.IsPhoneVerified,
		&c.Valid,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, xerrors.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	c.ID = strconv.FormatInt(credID, 10)
	c.UserID = strconv.FormatInt(userID, 10)
	return &c, nil
}

// scanUserProfile scans a joined query into UserProfile
func scanUserProfile(row pgx.Row) (*domain.UserProfile, error) {
	var p domain.UserProfile
	var userID int64
	err := row.Scan(
		&userID,
		&p.AccountStatus,
		&p.AccountType,
		&p.Email,
		&p.Phone,
		&p.IsEmailVerified,
		&p.IsPhoneVerified,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, xerrors.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	p.ID = strconv.FormatInt(userID, 10)
	return &p, nil
}

// scanUserWithCredential scans a joined query into UserWithCredential
func scanUserWithCredential(row pgx.Row) (*domain.UserWithCredential, error) {
	var uwc domain.UserWithCredential
	var userID, credID, credUserID int64
	
	err := row.Scan(
		// User fields
		&userID,
		&uwc.User.AccountStatus,
		&uwc.User.AccountType,
		&uwc.User.AccountRestored,
		&uwc.User.Consent,
		&uwc.User.CreatedAt,
		&uwc.User.UpdatedAt,
		// Credential fields
		&credID,
		&credUserID,
		&uwc.Credential.Email,
		&uwc.Credential.Phone,
		&uwc.Credential.PasswordHash,
		&uwc.Credential.IsEmailVerified,
		&uwc.Credential.IsPhoneVerified,
		&uwc.Credential.Valid,
		&uwc.Credential.CreatedAt,
		&uwc.Credential.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, xerrors.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	
	uwc.User.ID = strconv.FormatInt(userID, 10)
	uwc.Credential.ID = strconv.FormatInt(credID, 10)
	uwc.Credential.UserID = strconv.FormatInt(credUserID, 10)
	
	return &uwc, nil
}

// ============================================
// SINGLE OPERATIONS
// ============================================

// GetUserByID fetches user profile by ID (returns UserProfile - safe for API)
func (r *UserRepository) GetUserByID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	profile, err := scanUserProfile(r.db.QueryRow(ctx, `
		SELECT 
			u.id,
			u.account_status,
			u.account_type,
			c.email,
			c.phone,
			c.is_email_verified,
			c.is_phone_verified,
			u.created_at,
			u.updated_at
		FROM users u
		JOIN user_credentials c ON u.id = c.user_id
		WHERE u.id = $1
		  AND u.account_status != 'deleted'
		  AND c.valid = true
		LIMIT 1
	`, uid))
	
	return profile, err
}

// GetUserByIdentifier fetches user by email/phone (returns UserWithCredential - includes password hash)
func (r *UserRepository) GetUserByIdentifier(ctx context.Context, identifier string) (*domain.UserWithCredential, error) {
	uwc, err := scanUserWithCredential(r.db.QueryRow(ctx, `
		SELECT 
			u.id,
			u.account_status,
			u.account_type,
			u.account_restored,
			u.consent,
			u.created_at,
			u.updated_at,
			c.id,
			c.user_id,
			c.email,
			c.phone,
			c.password_hash,
			c.is_email_verified,
			c.is_phone_verified,
			c.valid,
			c.created_at,
			c.updated_at
		FROM users u
		JOIN user_credentials c ON u.id = c.user_id
		WHERE (c.email = $1 OR c.phone = $1)
		  AND c.valid = true
		  AND u.account_status != 'deleted'
		LIMIT 1
	`, identifier))
	
	return uwc, err
}

// ============================================
// BATCH OPERATIONS
// ============================================

// GetUsersByIDs fetches multiple user profiles by IDs (returns UserProfile[] - safe for API)
func (r *UserRepository) GetUsersByIDs(ctx context.Context, userIDs []string) ([]*domain.UserProfile, error) {
	if len(userIDs) == 0 {
		return []*domain.UserProfile{}, nil
	}

	// Convert string IDs to int64
	uids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		uid, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID %s: %w", id, err)
		}
		uids = append(uids, uid)
	}

	rows, err := r.db.Query(ctx, `
		SELECT 
			u.id,
			u.account_status,
			u.account_type,
			c.email,
			c.phone,
			c.is_email_verified,
			c.is_phone_verified,
			u.created_at,
			u.updated_at
		FROM users u
		JOIN user_credentials c ON u.id = c.user_id
		WHERE u.id = ANY($1)
		  AND u.account_status != 'deleted'
		  AND c.valid = true
		ORDER BY u.id
	`, uids)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	profiles := make([]*domain.UserProfile, 0, len(userIDs))
	for rows.Next() {
		profile, err := scanUserProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user profile: %w", err)
		}
		profiles = append(profiles, profile)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return profiles, nil
}

// GetUsersByIdentifiers fetches multiple users by email/phone (returns UserWithCredential[] - includes password hash)
func (r *UserRepository) GetUsersByIdentifiers(ctx context.Context, identifiers []string) ([]*domain.UserWithCredential, error) {
	if len(identifiers) == 0 {
		return []*domain.UserWithCredential{}, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT 
			u.id,
			u.account_status,
			u.account_type,
			u.account_restored,
			u.consent,
			u.created_at,
			u.updated_at,
			c.id,
			c.user_id,
			c.email,
			c.phone,
			c.password_hash,
			c.is_email_verified,
			c.is_phone_verified,
			c.valid,
			c.created_at,
			c.updated_at
		FROM users u
		JOIN user_credentials c ON u.id = c.user_id
		WHERE (c.email = ANY($1) OR c.phone = ANY($1))
		  AND c.valid = true
		  AND u.account_status != 'deleted'
		ORDER BY u.id
	`, identifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.UserWithCredential, 0, len(identifiers))
	for rows.Next() {
		uwc, err := scanUserWithCredential(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user with credential: %w", err)
		}
		users = append(users, uwc)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return users, nil
}

// ============================================
// ADDITIONAL HELPER METHODS
// ============================================

// GetUserWithCredentials fetches user and ALL their credentials (valid and invalid)
// Useful for admin views or account management
func (r *UserRepository) GetUserWithCredentialsByID(ctx context.Context, userID string) (*domain.User, []*domain.UserCredential, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Fetch user
	user, err := scanUser(r.db.QueryRow(ctx, `
		SELECT 
			id,
			account_status,
			account_type,
			account_restored,
			consent,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
		  AND account_status != 'deleted'
		LIMIT 1
	`, uid))
	if err != nil {
		return nil, nil, err
	}

	// Fetch ALL credentials
	rows, err := r.db.Query(ctx, `
		SELECT 
			id,
			user_id,
			email,
			phone,
			password_hash,
			is_email_verified,
			is_phone_verified,
			valid,
			created_at,
			updated_at
		FROM user_credentials
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, uid)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var creds []*domain.UserCredential
	for rows.Next() {
		cred, err := scanCredential(rows)
		if err != nil {
			return nil, nil, err
		}
		creds = append(creds, cred)
	}
	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}

	return user, creds, nil
}

// UserExists checks if a user exists by ID
func (r *UserRepository) UserExistsByID(ctx context.Context, userID string) (bool, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid user ID: %w", err)
	}

	var exists bool
	err = r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users 
			WHERE id = $1 AND account_status != 'deleted'
		)
	`, uid).Scan(&exists)
	
	return exists, err
}

// IdentifierExists checks if email or phone is already taken
func (r *UserRepository) IdentifierExists(ctx context.Context, identifier string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_credentials 
			WHERE (email = $1 OR phone = $1) 
			  AND valid = true
		)
	`, identifier).Scan(&exists)
	
	return exists, err
}
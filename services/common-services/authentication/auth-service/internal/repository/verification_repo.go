package repository

import (
	"context"
	"fmt"
	"strconv"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
)

// ============================================
// SINGLE OPERATIONS - VERIFICATION STATUS
// ============================================

func (r *UserRepository) GetEmailVerificationStatus(ctx context.Context, userID string) (bool, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		SELECT is_email_verified
		FROM user_credentials
		WHERE user_id = $1
		  AND valid = true
		LIMIT 1
	`

	var isVerified bool
	err = r.db.QueryRow(ctx, query, uid).Scan(&isVerified)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, xerrors.ErrNotFound
		}
		return false, fmt.Errorf("failed to fetch email verification status for user %s: %w", userID, err)
	}

	return isVerified, nil
}

func (r *UserRepository) GetPhoneVerificationStatus(ctx context.Context, userID string) (bool, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		SELECT is_phone_verified
		FROM user_credentials
		WHERE user_id = $1
		  AND valid = true
		LIMIT 1
	`

	var isVerified bool
	err = r.db.QueryRow(ctx, query, uid).Scan(&isVerified)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, xerrors.ErrNotFound
		}
		return false, fmt.Errorf("failed to fetch phone verification status for user %s: %w", userID, err)
	}

	return isVerified, nil
}

func (r *UserRepository) UpdateEmailVerificationStatus(ctx context.Context, userID string, verified bool) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		UPDATE user_credentials
		SET 
			is_email_verified = $1,
			updated_at = NOW()
		WHERE user_id = $2 AND valid = true
	`
	
	result, err := r.db.Exec(ctx, query, verified, uid)
	if err != nil {
		return fmt.Errorf("failed to update email verification: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

func (r *UserRepository) UpdatePhoneVerificationStatus(ctx context.Context, userID string, verified bool) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		UPDATE user_credentials
		SET 
			is_phone_verified = $1,
			updated_at = NOW()
		WHERE user_id = $2 AND valid = true
	`
	
	result, err := r.db.Exec(ctx, query, verified, uid)
	if err != nil {
		return fmt.Errorf("failed to update phone verification: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// ============================================
// BATCH OPERATIONS - VERIFICATION STATUS
// ============================================

// VerificationStatus holds verification status for a user
type VerificationStatus struct {
	UserID            string `json:"user_id"`
	IsEmailVerified   bool   `json:"is_email_verified"`
	IsPhoneVerified   bool   `json:"is_phone_verified"`
	Email             *string `json:"email,omitempty"`
	Phone             *string `json:"phone,omitempty"`
}

// GetVerificationStatuses fetches verification status for multiple users
func (r *UserRepository) GetVerificationStatuses(ctx context.Context, userIDs []string) ([]*VerificationStatus, error) {
	if len(userIDs) == 0 {
		return []*VerificationStatus{}, nil
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

	query := `
		SELECT 
			user_id,
			is_email_verified,
			is_phone_verified,
			email,
			phone
		FROM user_credentials
		WHERE user_id = ANY($1)
		  AND valid = true
		ORDER BY user_id
	`

	rows, err := r.db.Query(ctx, query, uids)
	if err != nil {
		return nil, fmt.Errorf("failed to query verification statuses: %w", err)
	}
	defer rows.Close()

	statuses := make([]*VerificationStatus, 0, len(userIDs))
	for rows.Next() {
		var status VerificationStatus
		var userID int64
		
		err := rows.Scan(
			&userID,
			&status.IsEmailVerified,
			&status.IsPhoneVerified,
			&status.Email,
			&status.Phone,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan verification status: %w", err)
		}
		
		status.UserID = strconv.FormatInt(userID, 10)
		statuses = append(statuses, &status)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return statuses, nil
}

// UpdateEmailVerificationStatuses updates email verification status for multiple users
func (r *UserRepository) UpdateEmailVerificationStatuses(ctx context.Context, userIDs []string, verified bool) error {
	if len(userIDs) == 0 {
		return nil
	}

	// Convert string IDs to int64
	uids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		uid, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid user ID %s: %w", id, err)
		}
		uids = append(uids, uid)
	}

	query := `
		UPDATE user_credentials
		SET 
			is_email_verified = $1,
			updated_at = NOW()
		WHERE user_id = ANY($2) AND valid = true
	`
	
	result, err := r.db.Exec(ctx, query, verified, uids)
	if err != nil {
		return fmt.Errorf("failed to update email verification statuses: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// UpdatePhoneVerificationStatuses updates phone verification status for multiple users
func (r *UserRepository) UpdatePhoneVerificationStatuses(ctx context.Context, userIDs []string, verified bool) error {
	if len(userIDs) == 0 {
		return nil
	}

	// Convert string IDs to int64
	uids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		uid, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid user ID %s: %w", id, err)
		}
		uids = append(uids, uid)
	}

	query := `
		UPDATE user_credentials
		SET 
			is_phone_verified = $1,
			updated_at = NOW()
		WHERE user_id = ANY($2) AND valid = true
	`
	
	result, err := r.db.Exec(ctx, query, verified, uids)
	if err != nil {
		return fmt.Errorf("failed to update phone verification statuses: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// ============================================
// COMBINED OPERATIONS
// ============================================

// GetBothVerificationStatuses fetches both email and phone verification status
func (r *UserRepository) GetBothVerificationStatuses(ctx context.Context, userID string) (emailVerified, phoneVerified bool, err error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return false, false, fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		SELECT is_email_verified, is_phone_verified
		FROM user_credentials
		WHERE user_id = $1
		  AND valid = true
		LIMIT 1
	`

	err = r.db.QueryRow(ctx, query, uid).Scan(&emailVerified, &phoneVerified)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, false, xerrors.ErrNotFound
		}
		return false, false, fmt.Errorf("failed to fetch verification statuses for user %s: %w", userID, err)
	}

	return emailVerified, phoneVerified, nil
}

// UpdateBothVerificationStatuses updates both email and phone verification status
func (r *UserRepository) updateBothVerificationStatuses(ctx context.Context, userID string, emailVerified, phoneVerified bool) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		UPDATE user_credentials
		SET 
			is_email_verified = $1,
			is_phone_verified = $2,
			updated_at = NOW()
		WHERE user_id = $3 AND valid = true
	`
	
	result, err := r.db.Exec(ctx, query, emailVerified, phoneVerified, uid)
	if err != nil {
		return fmt.Errorf("failed to update verification statuses: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// MarkCredentialAsVerified marks both email and phone as verified (for OAuth/SSO)
func (r *UserRepository) MarkCredentialAsVerified(ctx context.Context, userID string) error {
	return r.updateBothVerificationStatuses(ctx, userID, true, true)
}
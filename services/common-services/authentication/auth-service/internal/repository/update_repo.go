package repository

import (
	"auth-service/internal/domain"
	"context"
	"fmt"
	"strconv"
	xerrors "x/shared/utils/errors"
)

// UpdateEmail updates the user's email and logs the change to credential_history
func (r *UserRepository) UpdateEmail(ctx context.Context, userID, newEmail string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// The trigger will automatically log to credential_history
	// We just need to update the credential
	query := `
		UPDATE user_credentials
		SET 
			email = $1,
			is_email_verified = false,  -- Reset verification on email change
			updated_at = NOW()
		WHERE user_id = $2 AND valid = true
	`
	
	result, err := tx.Exec(ctx, query, newEmail, uid)
	if err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no valid credential found for user %s", userID)
	}

	return tx.Commit(ctx)
}

// UpdatePassword updates the user's password hash
// The credential_history trigger will automatically log the old password
func (r *UserRepository) UpdatePassword(ctx context.Context, userID string, newHash string) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// The trigger trg_log_credential_change will automatically archive the old password
	query := `
		UPDATE user_credentials
		SET 
			password_hash = $1,
			updated_at = NOW()
		WHERE user_id = $2 AND valid = true
	`
	
	result, err := r.db.Exec(ctx, query, newHash, uid)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no valid credential found for user %s", userID)
	}

	return nil
}

// UpdatePhone updates the user's phone number and verification status
func (r *UserRepository) UpdatePhone(ctx context.Context, userID, newPhone string, isPhoneVerified bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// The trigger will automatically log to credential_history
	query := `
		UPDATE user_credentials
		SET 
			phone = $1,
			is_phone_verified = $2,
			updated_at = NOW()
		WHERE user_id = $3 AND valid = true
	`

	result, err := tx.Exec(ctx, query, newPhone, isPhoneVerified, uid)
	if err != nil {
		return fmt.Errorf("failed to update phone: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no valid credential found for user %s", userID)
	}

	return tx.Commit(ctx)
}

// GetCredentialHistory retrieves the change history for a user
func (r *UserRepository) GetCredentialHistory(ctx context.Context, userID string, limit int) ([]*domain.CredentialHistory, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		SELECT id, user_id, old_email, old_phone, changed_at
		FROM credential_history
		WHERE user_id = $1
		ORDER BY changed_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, uid, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query credential history: %w", err)
	}
	defer rows.Close()

	var history []*domain.CredentialHistory
	for rows.Next() {
		var h domain.CredentialHistory
		var historyID, historyUserID int64

		err := rows.Scan(
			&historyID,
			&historyUserID,
			&h.OldEmail,
			&h.OldPhone,
			&h.ChangedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan credential history: %w", err)
		}

		h.ID = strconv.FormatInt(historyID, 10)
		h.UserID = strconv.FormatInt(historyUserID, 10)
		history = append(history, &h)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return history, nil
}

// InvalidateAllCredentials marks all credentials for a user as invalid
// Useful for account deletion or security incidents
func (r *UserRepository) InvalidateAllCredentials(ctx context.Context, userID string) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		UPDATE user_credentials
		SET 
			valid = false,
			updated_at = NOW()
		WHERE user_id = $1
	`
	
	_, err = r.db.Exec(ctx, query, uid)
	if err != nil {
		return fmt.Errorf("failed to invalidate credentials: %w", err)
	}

	return nil
}


// ============================================
// ACCOUNT STATUS MANAGEMENT
// ============================================

// UpdateAccountStatus updates a user's account status
func (r *UserRepository) UpdateAccountStatus(ctx context.Context, userID, status string) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Validate status
	validStatuses := map[string]bool{
		"active":    true,
		"suspended": true,
		"deleted":   true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid account status: %s", status)
	}

	query := `
		UPDATE users
		SET 
			account_status = $1,
			updated_at = NOW()
		WHERE id = $2
	`
	
	result, err := r.db.Exec(ctx, query, status, uid)
	if err != nil {
		return fmt.Errorf("failed to update account status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// UpdateAccountType updates a user's account type
func (r *UserRepository) UpdateAccountType(ctx context.Context, userID, accountType string) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Validate account type
	validTypes := map[string]bool{
		"password": true,
		"social":   true,
		"hybrid":   true,
	}

	if !validTypes[accountType] {
		return fmt.Errorf("invalid account type: %s", accountType)
	}

	query := `
		UPDATE users
		SET 
			account_type = $1,
			updated_at = NOW()
		WHERE id = $2
	`
	
	result, err := r.db.Exec(ctx, query, accountType, uid)
	if err != nil {
		return fmt.Errorf("failed to update account type: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// MarkAccountAsRestored marks an account as restored after deletion
func (r *UserRepository) MarkAccountAsRestored(ctx context.Context, userID string) error {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	query := `
		UPDATE users
		SET 
			account_status = 'active',
			account_restored = true,
			updated_at = NOW()
		WHERE id = $1
	`
	
	result, err := r.db.Exec(ctx, query, uid)
	if err != nil {
		return fmt.Errorf("failed to restore account: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// SuspendAccount suspends a user account
func (r *UserRepository) SuspendAccount(ctx context.Context, userID string) error {
	return r.UpdateAccountStatus(ctx, userID, "suspended")
}

// DeleteAccount soft-deletes a user account
func (r *UserRepository) DeleteAccount(ctx context.Context, userID string) error {
	return r.UpdateAccountStatus(ctx, userID, "deleted")
}

// ActivateAccount activates a user account
func (r *UserRepository) ActivateAccount(ctx context.Context, userID string) error {
	return r.UpdateAccountStatus(ctx, userID, "active")
}

// ============================================
// ACCOUNT TYPE TRANSITIONS
// ============================================

// ConvertToHybridAccount converts a social or password-only account to hybrid
func (r *UserRepository) ConvertToHybridAccount(ctx context.Context, userID string) error {
	return r.UpdateAccountType(ctx, userID, "hybrid")
}

// ============================================
// BATCH OPERATIONS
// ============================================

// UpdateAccountStatuses updates status for multiple users
func (r *UserRepository) UpdateAccountStatuses(ctx context.Context, userIDs []string, status string) error {
	if len(userIDs) == 0 {
		return nil
	}

	// Validate status
	validStatuses := map[string]bool{
		"active":    true,
		"suspended": true,
		"deleted":   true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid account status: %s", status)
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
		UPDATE users
		SET 
			account_status = $1,
			updated_at = NOW()
		WHERE id = ANY($2)
	`
	
	result, err := r.db.Exec(ctx, query, status, uids)
	if err != nil {
		return fmt.Errorf("failed to update account statuses: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}

// SuspendAccounts suspends multiple accounts
func (r *UserRepository) SuspendAccounts(ctx context.Context, userIDs []string) error {
	return r.UpdateAccountStatuses(ctx, userIDs, "suspended")
}

// DeleteAccounts soft-deletes multiple accounts
func (r *UserRepository) DeleteAccounts(ctx context.Context, userIDs []string) error {
	return r.UpdateAccountStatuses(ctx, userIDs, "deleted")
}

// ActivateAccounts activates multiple accounts
func (r *UserRepository) ActivateAccounts(ctx context.Context, userIDs []string) error {
	return r.UpdateAccountStatuses(ctx, userIDs, "active")
}
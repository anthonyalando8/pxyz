package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// DeleteUsers performs a hard delete of multiple users and all related data
func (r *UserRepository) DeleteUsers(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Convert string IDs to int64
	uids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		uid, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid user ID %s: %w", id, err)
		}
		uids = append(uids, uid)
	}

	// Delete password reset tokens
	if _, err = tx.Exec(ctx, `
		DELETE FROM password_reset_tokens 
		WHERE user_id = ANY($1)
	`, uids); err != nil {
		return fmt.Errorf("failed to delete password_reset_tokens: %w", err)
	}

	// Delete email verification tokens
	if _, err = tx.Exec(ctx, `
		DELETE FROM email_verification_tokens 
		WHERE user_id = ANY($1)
	`, uids); err != nil {
		return fmt.Errorf("failed to delete email_verification_tokens: %w", err)
	}

	// Delete phone verification tokens
	if _, err = tx.Exec(ctx, `
		DELETE FROM phone_verification_tokens 
		WHERE user_id = ANY($1)
	`, uids); err != nil {
		return fmt.Errorf("failed to delete phone_verification_tokens: %w", err)
	}

	// Delete user_credentials (will CASCADE to credential_history)
	if _, err = tx.Exec(ctx, `
		DELETE FROM user_credentials 
		WHERE user_id = ANY($1)
	`, uids); err != nil {
		return fmt.Errorf("failed to delete user_credentials: %w", err)
	}

	// Finally, delete the users
	result, err := tx.Exec(ctx, `
		DELETE FROM users 
		WHERE id = ANY($1)
	`, uids)
	if err != nil {
		return fmt.Errorf("failed to delete users: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no users found with provided IDs")
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SoftDeleteUsers marks multiple users as deleted without removing data
func (r *UserRepository) SoftDeleteUsers(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Convert string IDs to int64
	uids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		uid, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid user ID %s: %w", id, err)
		}
		uids = append(uids, uid)
	}

	// Update user status to 'deleted'
	result, err := tx.Exec(ctx, `
		UPDATE users
		SET 
			account_status = 'deleted',
			updated_at = NOW()
		WHERE id = ANY($1) AND account_status != 'deleted'
	`, uids)
	if err != nil {
		return fmt.Errorf("failed to soft delete users: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no active users found with provided IDs")
	}

	// Invalidate all credentials for these users
	_, err = tx.Exec(ctx, `
		UPDATE user_credentials
		SET 
			valid = false,
			updated_at = NOW()
		WHERE user_id = ANY($1) AND valid = true
	`, uids)
	if err != nil {
		return fmt.Errorf("failed to invalidate credentials: %w", err)
	}

	// Delete all active tokens for these users
	_, err = tx.Exec(ctx, `
		DELETE FROM password_reset_tokens 
		WHERE user_id = ANY($1)
	`, uids)
	if err != nil {
		return fmt.Errorf("failed to delete password reset tokens: %w", err)
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM email_verification_tokens 
		WHERE user_id = ANY($1)
	`, uids)
	if err != nil {
		return fmt.Errorf("failed to delete email verification tokens: %w", err)
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM phone_verification_tokens 
		WHERE user_id = ANY($1)
	`, uids)
	if err != nil {
		return fmt.Errorf("failed to delete phone verification tokens: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RestoreUsers restores multiple soft-deleted user accounts
func (r *UserRepository) RestoreUsers(ctx context.Context, userIDs []string) error {
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

	result, err := r.db.Exec(ctx, `
		UPDATE users
		SET 
			account_status = 'active',
			account_restored = true,
			updated_at = NOW()
		WHERE id = ANY($1) AND account_status = 'deleted'
	`, uids)
	if err != nil {
		return fmt.Errorf("failed to restore users: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no deleted users found with provided IDs")
	}

	return nil
}

// PermanentlyDeleteUsers deletes multiple users that have been soft-deleted for a certain period
func (r *UserRepository) PermanentlyDeleteUsers(ctx context.Context, gracePeriodDays int) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Find users eligible for permanent deletion
	cutoffDate := time.Now().Add(-time.Duration(gracePeriodDays) * 24 * time.Hour)

	var uids []int64
	rows, err := tx.Query(ctx, `
		SELECT id 
		FROM users
		WHERE account_status = 'deleted' 
		  AND updated_at < $1
	`, cutoffDate)
	if err != nil {
		return 0, fmt.Errorf("failed to find eligible users: %w", err)
	}

	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return 0, fmt.Errorf("failed to scan user ID: %w", err)
		}
		uids = append(uids, uid)
	}
	rows.Close()

	if rows.Err() != nil {
		return 0, rows.Err()
	}

	if len(uids) == 0 {
		return 0, nil // No users to delete
	}

	// Delete all tokens
	_, err = tx.Exec(ctx, `
		DELETE FROM password_reset_tokens 
		WHERE user_id = ANY($1)
	`, uids)
	if err != nil {
		return 0, fmt.Errorf("failed to delete password_reset_tokens: %w", err)
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM email_verification_tokens 
		WHERE user_id = ANY($1)
	`, uids)
	if err != nil {
		return 0, fmt.Errorf("failed to delete email_verification_tokens: %w", err)
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM phone_verification_tokens 
		WHERE user_id = ANY($1)
	`, uids)
	if err != nil {
		return 0, fmt.Errorf("failed to delete phone_verification_tokens: %w", err)
	}

	// Delete credentials (will CASCADE to credential_history)
	_, err = tx.Exec(ctx, `
		DELETE FROM user_credentials 
		WHERE user_id = ANY($1)
	`, uids)
	if err != nil {
		return 0, fmt.Errorf("failed to delete user_credentials: %w", err)
	}

	// Finally delete the users
	result, err := tx.Exec(ctx, `
		DELETE FROM users 
		WHERE id = ANY($1)
	`, uids)
	if err != nil {
		return 0, fmt.Errorf("failed to delete users: %w", err)
	}

	deletedCount := result.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return deletedCount, nil
}

// DeleteUser - single user wrapper for backward compatibility
func (r *UserRepository) DeleteUser(ctx context.Context, userID string) error {
	return r.DeleteUsers(ctx, []string{userID})
}

// SoftDeleteUser - single user wrapper for backward compatibility
func (r *UserRepository) SoftDeleteUser(ctx context.Context, userID string) error {
	return r.SoftDeleteUsers(ctx, []string{userID})
}

// RestoreUser - single user wrapper for backward compatibility
func (r *UserRepository) RestoreUser(ctx context.Context, userID string) error {
	return r.RestoreUsers(ctx, []string{userID})
}

// CleanupExpiredTokens removes expired tokens (run as a scheduled job)
func (r *UserRepository) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var totalDeleted int64

	// Delete expired password reset tokens
	result, err := tx.Exec(ctx, `
		DELETE FROM password_reset_tokens 
		WHERE expires_at < NOW() OR used = true
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup password_reset_tokens: %w", err)
	}
	totalDeleted += result.RowsAffected()

	// Delete expired email verification tokens
	result, err = tx.Exec(ctx, `
		DELETE FROM email_verification_tokens 
		WHERE expires_at < NOW() OR used = true
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup email_verification_tokens: %w", err)
	}
	totalDeleted += result.RowsAffected()

	// Delete expired phone verification tokens
	result, err = tx.Exec(ctx, `
		DELETE FROM phone_verification_tokens 
		WHERE expires_at < NOW() OR used = true
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup phone_verification_tokens: %w", err)
	}
	totalDeleted += result.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return totalDeleted, nil
}

// BatchDeleteByStatus deletes users in batches by account status (for administrative cleanup)
func (r *UserRepository) BatchDeleteByStatus(ctx context.Context, status string, batchSize int, gracePeriodDays int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}

	cutoffDate := time.Now().Add(-time.Duration(gracePeriodDays) * 24 * time.Hour)
	var totalDeleted int64

	for {
		// Find a batch of users
		var uids []int64
		rows, err := r.db.Query(ctx, `
			SELECT id 
			FROM users
			WHERE account_status = $1 
			  AND updated_at < $2
			LIMIT $3
		`, status, cutoffDate, batchSize)
		if err != nil {
			return totalDeleted, fmt.Errorf("failed to find users: %w", err)
		}

		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err != nil {
				rows.Close()
				return totalDeleted, fmt.Errorf("failed to scan user ID: %w", err)
			}
			uids = append(uids, uid)
		}
		rows.Close()

		if rows.Err() != nil {
			return totalDeleted, rows.Err()
		}

		if len(uids) == 0 {
			break // No more users to delete
		}

		// Convert to strings for DeleteUsers
		userIDs := make([]string, len(uids))
		for i, uid := range uids {
			userIDs[i] = strconv.FormatInt(uid, 10)
		}

		// Delete this batch
		if err := r.DeleteUsers(ctx, userIDs); err != nil {
			return totalDeleted, fmt.Errorf("failed to delete batch: %w", err)
		}

		totalDeleted += int64(len(uids))

		// If we got less than batchSize, we're done
		if len(uids) < batchSize {
			break
		}
	}

	return totalDeleted, nil
}
package repository

import (
	"auth-service/internal/domain"
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (r *UserRepository) CreateUsers(ctx context.Context, users []*domain.User, creds []*domain.UserCredential) ([]*domain.User, []error) {
	if len(users) == 0 {
		return nil, nil
	}

	if len(users) != len(creds) {
		return nil, []error{fmt.Errorf("users and credentials length mismatch")}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, []error{err}
	}
	defer tx.Rollback(ctx)

	var savedUsers []*domain.User
	var errs []error

	// Batch insert for users
	userValues := make([]string, 0, len(users))
	userArgs := make([]interface{}, 0, len(users)*5)
	for i, u := range users {
		start := i*5 + 1
		userValues = append(userValues,
			fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)",
				start, start+1, start+2, start+3, start+4))
		
		// Convert string ID to int64
		userID, err := strconv.ParseInt(u.ID, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid user ID: %w", err))
			continue
		}
		
		userArgs = append(userArgs,
			userID,
			coalesceString(u.AccountStatus, "active"),
			coalesceString(u.AccountType, "password"),
			u.AccountRestored,
			u.Consent,
		)
	}

	if len(errs) > 0 {
		return nil, errs
	}

	userQuery := fmt.Sprintf(`
		INSERT INTO users (id, account_status, account_type, account_restored, consent)
		VALUES %s
		RETURNING id, account_status, account_type, account_restored, consent, created_at, updated_at
	`, strings.Join(userValues, ","))

	rows, err := tx.Query(ctx, userQuery, userArgs...)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to insert users: %w", err)}
	}
	
	for rows.Next() {
		u := new(domain.User)
		var userID int64
		if err := rows.Scan(&userID, &u.AccountStatus, &u.AccountType, &u.AccountRestored, &u.Consent, &u.CreatedAt, &u.UpdatedAt); err != nil {
			errs = append(errs, err)
			continue
		}
		u.ID = strconv.FormatInt(userID, 10)
		savedUsers = append(savedUsers, u)
	}
	rows.Close()

	if rows.Err() != nil {
		return nil, []error{rows.Err()}
	}

	// Batch insert for credentials
	credValues := make([]string, 0, len(creds))
	credArgs := make([]interface{}, 0, len(creds)*8) // Updated to 8 fields
	
	for i, c := range creds {
		start := i*8 + 1
		credValues = append(credValues,
			fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				start, start+1, start+2, start+3, start+4, start+5, start+6, start+7))
		
		// Convert string IDs to int64
		credID, err := strconv.ParseInt(c.ID, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid credential ID: %w", err))
			continue
		}
		
		userID, err := strconv.ParseInt(c.UserID, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid user ID in credential: %w", err))
			continue
		}
		
		credArgs = append(credArgs,
			credID,
			userID,
			nullOrNilPtr(c.Email),
			nullOrNilPtr(c.Phone),
			nullOrNilPtr(c.PasswordHash),
			c.IsEmailVerified,
			c.IsPhoneVerified,
			c.Valid, // Explicitly set valid field
		)
	}

	if len(errs) > 0 {
		return savedUsers, errs
	}

	credQuery := fmt.Sprintf(`
		INSERT INTO user_credentials (id, user_id, email, phone, password_hash, is_email_verified, is_phone_verified, valid)
		VALUES %s
	`, strings.Join(credValues, ","))

	if _, err := tx.Exec(ctx, credQuery, credArgs...); err != nil {
		return savedUsers, []error{fmt.Errorf("failed to insert credentials: %w", err)}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, []error{fmt.Errorf("failed to commit transaction: %w", err)}
	}
	
	return savedUsers, nil
}




// Helper for default strings
func coalesceString(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
func nullOrNilPtr(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}


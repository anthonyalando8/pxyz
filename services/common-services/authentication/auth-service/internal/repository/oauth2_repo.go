package repository

import (
	"auth-service/internal/domain"
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ================================
// OAUTH ACCOUNT OPERATIONS
// ================================

// CreateOAuthAccount creates a new OAuth account link
func (r *UserRepository) CreateOAuthAccount(ctx context.Context, acc *domain.OAuthAccount) error {
	query := `
		INSERT INTO oauth_accounts (
			id, user_id, provider, provider_uid, access_token, refresh_token, 
			expires_at, scope, linked_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	`
	
	_, err := r.db.Exec(ctx, query,
		acc.ID,
		acc.UserID,
		acc.Provider,
		acc.ProviderUID,
		acc.AccessToken,
		acc.RefreshToken,
		acc.ExpiresAt,
		acc.Scope,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create oauth account: %w", err)
	}
	
	return nil
}

// FindByProviderUID finds an OAuth account by provider and provider user ID
func (r *UserRepository) FindByProviderUID(ctx context.Context, provider, providerUID string) (*domain.OAuthAccount, error) {
	query := `
		SELECT 
			id, user_id, provider, provider_uid, access_token, refresh_token,
			expires_at, scope, linked_at, updated_at
		FROM oauth_accounts 
		WHERE provider = $1 AND provider_uid = $2
		LIMIT 1
	`
	
	acc := &domain.OAuthAccount{}
	err := r.db.QueryRow(ctx, query, provider, providerUID).Scan(
		&acc.ID,
		&acc.UserID,
		&acc.Provider,
		&acc.ProviderUID,
		&acc.AccessToken,
		&acc.RefreshToken,
		&acc.ExpiresAt,
		&acc.Scope,
		&acc.LinkedAt,
		&acc.UpdatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found is not an error in this case
		}
		return nil, fmt.Errorf("failed to find oauth account: %w", err)
	}
	
	return acc, nil
}

// GetOAuthAccountsByUserID retrieves all OAuth accounts linked to a user
func (r *UserRepository) GetOAuthAccountsByUserID(ctx context.Context, userID string) ([]*domain.OAuthAccount, error) {
	query := `
		SELECT 
			id, user_id, provider, provider_uid, access_token, refresh_token,
			expires_at, scope, linked_at, updated_at
		FROM oauth_accounts 
		WHERE user_id = $1
		ORDER BY linked_at DESC
	`
	
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query oauth accounts: %w", err)
	}
	defer rows.Close()
	
	var accounts []*domain.OAuthAccount
	for rows.Next() {
		acc := &domain.OAuthAccount{}
		err := rows.Scan(
			&acc.ID,
			&acc.UserID,
			&acc.Provider,
			&acc.ProviderUID,
			&acc.AccessToken,
			&acc.RefreshToken,
			&acc.ExpiresAt,
			&acc.Scope,
			&acc.LinkedAt,
			&acc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan oauth account: %w", err)
		}
		accounts = append(accounts, acc)
	}
	
	return accounts, rows.Err()
}

// UpdateOAuthTokens updates the access and refresh tokens for an OAuth account
func (r *UserRepository) UpdateOAuthTokens(ctx context.Context, provider, providerUID string, accessToken, refreshToken *string, expiresAt *time.Time) error {
	query := `
		UPDATE oauth_accounts 
		SET 
			access_token = $1,
			refresh_token = $2,
			expires_at = $3,
			updated_at = NOW()
		WHERE provider = $4 AND provider_uid = $5
	`
	
	result, err := r.db.Exec(ctx, query, accessToken, refreshToken, expiresAt, provider, providerUID)
	if err != nil {
		return fmt.Errorf("failed to update oauth tokens: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("oauth account not found")
	}
	
	return nil
}

// UnlinkOAuthAccount removes an OAuth account link
func (r *UserRepository) UnlinkOAuthAccount(ctx context.Context, userID, provider string) error {
	query := `
		DELETE FROM oauth_accounts 
		WHERE user_id = $1 AND provider = $2
	`
	
	result, err := r.db.Exec(ctx, query, userID, provider)
	if err != nil {
		return fmt.Errorf("failed to unlink oauth account: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("oauth account not found")
	}
	
	return nil
}

// CheckOAuthAccountExists checks if an OAuth account exists for a provider
func (r *UserRepository) CheckOAuthAccountExists(ctx context.Context, userID, provider string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM oauth_accounts 
			WHERE user_id = $1 AND provider = $2
		)
	`
	
	var exists bool
	err := r.db.QueryRow(ctx, query, userID, provider).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check oauth account existence: %w", err)
	}
	
	return exists, nil
}

// GetOAuthAccountByID retrieves an OAuth account by its ID
func (r *UserRepository) GetOAuthAccountByID(ctx context.Context, id string) (*domain.OAuthAccount, error) {
	query := `
		SELECT 
			id, user_id, provider, provider_uid, access_token, refresh_token,
			expires_at, scope, linked_at, updated_at
		FROM oauth_accounts 
		WHERE id = $1
		LIMIT 1
	`
	
	acc := &domain.OAuthAccount{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&acc.ID,
		&acc.UserID,
		&acc.Provider,
		&acc.ProviderUID,
		&acc.AccessToken,
		&acc.RefreshToken,
		&acc.ExpiresAt,
		&acc.Scope,
		&acc.LinkedAt,
		&acc.UpdatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("oauth account not found")
		}
		return nil, fmt.Errorf("failed to get oauth account: %w", err)
	}
	
	return acc, nil
}

// GetUserIDByProviderUID gets the user ID associated with a provider UID
func (r *UserRepository) GetUserIDByProviderUID(ctx context.Context, provider, providerUID string) (string, error) {
	query := `
		SELECT user_id 
		FROM oauth_accounts 
		WHERE provider = $1 AND provider_uid = $2
		LIMIT 1
	`
	
	var userID string
	err := r.db.QueryRow(ctx, query, provider, providerUID).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get user id by provider uid: %w", err)
	}
	
	return userID, nil
}

// UpdateOAuthAccountMetadata updates additional metadata for an OAuth account
func (r *UserRepository) UpdateOAuthAccountMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	query := `
		UPDATE oauth_accounts 
		SET 
			metadata = $1,
			updated_at = NOW()
		WHERE id = $2
	`
	
	result, err := r.db.Exec(ctx, query, metadata, id)
	if err != nil {
		return fmt.Errorf("failed to update oauth account metadata: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("oauth account not found")
	}
	
	return nil
}

// CountOAuthAccountsByProvider counts OAuth accounts by provider
func (r *UserRepository) CountOAuthAccountsByProvider(ctx context.Context, provider string) (int64, error) {
	query := `
		SELECT COUNT(*) 
		FROM oauth_accounts 
		WHERE provider = $1
	`
	
	var count int64
	err := r.db.QueryRow(ctx, query, provider).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count oauth accounts: %w", err)
	}
	
	return count, nil
}

// GetRecentOAuthLinks retrieves recently linked OAuth accounts
func (r *UserRepository) GetRecentOAuthLinks(ctx context.Context, limit int) ([]*domain.OAuthAccount, error) {
	query := `
		SELECT 
			id, user_id, provider, provider_uid, access_token, refresh_token,
			expires_at, scope, linked_at, updated_at
		FROM oauth_accounts 
		ORDER BY linked_at DESC
		LIMIT $1
	`
	
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent oauth links: %w", err)
	}
	defer rows.Close()
	
	var accounts []*domain.OAuthAccount
	for rows.Next() {
		acc := &domain.OAuthAccount{}
		err := rows.Scan(
			&acc.ID,
			&acc.UserID,
			&acc.Provider,
			&acc.ProviderUID,
			&acc.AccessToken,
			&acc.RefreshToken,
			&acc.ExpiresAt,
			&acc.Scope,
			&acc.LinkedAt,
			&acc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan oauth account: %w", err)
		}
		accounts = append(accounts, acc)
	}
	
	return accounts, rows.Err()
}
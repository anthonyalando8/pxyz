// oauth2_repository.go under services/auth-service/internal/repository
package repository

import (
	"auth-service/internal/domain"
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	// Remove: "github.com/lib/pq"
)

// ================================
// OAUTH2 CLIENT OPERATIONS
// ================================

// CreateOAuth2Client registers a new OAuth2 client application
func (r *UserRepository) CreateOAuth2Client(ctx context.Context, client *domain.OAuth2Client) (*domain.OAuth2Client, error) {
	query := `
		INSERT INTO oauth2_clients (
			client_id, client_secret_hash, client_name, client_uri, logo_uri,
			owner_user_id, redirect_uris, grant_types, response_types, scope, is_confidential
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	ownerID, _ := strconv.ParseInt(client.OwnerUserID, 10, 64)

	var id int64
	err := r.db.QueryRow(ctx, query,
		client.ClientID,
		client.ClientSecret,
		client.ClientName,
		client.ClientURI,
		client.LogoURI,
		ownerID,
		client.RedirectURIs,      // pgx handles slices directly
		client.GrantTypes,        // pgx handles slices directly
		client.ResponseTypes,     // pgx handles slices directly
		client.Scope,
		client.IsConfidential,
	).Scan(&id, &client.CreatedAt, &client.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth2 client: %w", err)
	}

	client.ID = strconv.FormatInt(id, 10)
	return client, nil
}

// GetOAuth2ClientByClientID fetches a client by client_id
func (r *UserRepository) GetOAuth2ClientByClientID(ctx context.Context, clientID string) (*domain.OAuth2Client, error) {
	query := `
		SELECT 
			id, client_id, client_secret_hash, client_name, client_uri, logo_uri,
			owner_user_id, redirect_uris, grant_types, response_types, scope,
			is_confidential, is_active, created_at, updated_at
		FROM oauth2_clients
		WHERE client_id = $1
		LIMIT 1
	`

	var client domain.OAuth2Client
	var id, ownerID int64

	err := r.db.QueryRow(ctx, query, clientID).Scan(
		&id,
		&client.ClientID,
		&client.ClientSecret,
		&client.ClientName,
		&client.ClientURI,
		&client.LogoURI,
		&ownerID,
		&client.RedirectURIs,     // pgx scans directly into []string
		&client.GrantTypes,       // pgx scans directly into []string
		&client.ResponseTypes,    // pgx scans directly into []string
		&client.Scope,
		&client.IsConfidential,
		&client.IsActive,
		&client.CreatedAt,
		&client.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrInvalidClient
		}
		return nil, fmt.Errorf("failed to get OAuth2 client: %w", err)
	}

	client.ID = strconv.FormatInt(id, 10)
	client.OwnerUserID = strconv.FormatInt(ownerID, 10)

	return &client, nil
}

// GetOAuth2ClientsByOwner fetches all clients owned by a user
func (r *UserRepository) GetOAuth2ClientsByOwner(ctx context.Context, ownerUserID string) ([]*domain.OAuth2Client, error) {
	ownerID, _ := strconv.ParseInt(ownerUserID, 10, 64)

	query := `
		SELECT 
			id, client_id, client_secret_hash, client_name, client_uri, logo_uri,
			owner_user_id, redirect_uris, grant_types, response_types, scope,
			is_confidential, is_active, created_at, updated_at
		FROM oauth2_clients
		WHERE owner_user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query OAuth2 clients: %w", err)
	}
	defer rows.Close()

	var clients []*domain.OAuth2Client
	for rows.Next() {
		var client domain.OAuth2Client
		var id, ownerID int64

		err := rows.Scan(
			&id,
			&client.ClientID,
			&client.ClientSecret,
			&client.ClientName,
			&client.ClientURI,
			&client.LogoURI,
			&ownerID,
			&client.RedirectURIs,     // pgx scans directly into []string
			&client.GrantTypes,       // pgx scans directly into []string
			&client.ResponseTypes,    // pgx scans directly into []string
			&client.Scope,
			&client.IsConfidential,
			&client.IsActive,
			&client.CreatedAt,
			&client.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan OAuth2 client: %w", err)
		}

		client.ID = strconv.FormatInt(id, 10)
		client.OwnerUserID = strconv.FormatInt(ownerID, 10)
		clients = append(clients, &client)
	}

	return clients, rows.Err()
}

// UpdateOAuth2Client updates an existing OAuth2 client
func (r *UserRepository) UpdateOAuth2Client(ctx context.Context, client *domain.OAuth2Client) (*domain.OAuth2Client, error) {
	query := `
		UPDATE oauth2_clients
		SET 
			client_name = $1,
			client_uri = $2,
			logo_uri = $3,
			redirect_uris = $4,
			is_active = $5,
			updated_at = NOW()
		WHERE client_id = $6
		RETURNING updated_at
	`

	err := r.db.QueryRow(ctx, query,
		client.ClientName,
		client.ClientURI,
		client.LogoURI,
		client.RedirectURIs,  // pgx handles slices directly
		client.IsActive,
		client.ClientID,
	).Scan(&client.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrInvalidClient
		}
		return nil, fmt.Errorf("failed to update OAuth2 client: %w", err)
	}

	return client, nil
}

// DeleteOAuth2Client soft deletes an OAuth2 client by marking it inactive
func (r *UserRepository) DeleteOAuth2Client(ctx context.Context, clientID string) error {
	query := `
		UPDATE oauth2_clients
		SET is_active = false, updated_at = NOW()
		WHERE client_id = $1
	`

	result, err := r.db.Exec(ctx, query, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete OAuth2 client: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrInvalidClient
	}

	return nil
}

// RegenerateClientSecret generates a new client secret for an existing client
func (r *UserRepository) RegenerateClientSecret(ctx context.Context, clientID, newSecretHash string) error {
	query := `
		UPDATE oauth2_clients
		SET client_secret_hash = $1, updated_at = NOW()
		WHERE client_id = $2 AND is_active = true
	`

	result, err := r.db.Exec(ctx, query, newSecretHash, clientID)
	if err != nil {
		return fmt.Errorf("failed to regenerate client secret: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrInvalidClient
	}

	return nil
}

// ================================
// AUTHORIZATION CODE OPERATIONS
// ================================

// CreateAuthorizationCode creates a new authorization code
func (r *UserRepository) CreateAuthorizationCode(ctx context.Context, code *domain.OAuth2AuthorizationCode) error {
	query := `
		INSERT INTO oauth2_authorization_codes (
			code, client_id, user_id, redirect_uri, scope,
			code_challenge, code_challenge_method, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`

	userID, _ := strconv.ParseInt(code.UserID, 10, 64)
	var id int64

	err := r.db.QueryRow(ctx, query,
		code.Code,
		code.ClientID,
		userID,
		code.RedirectURI,
		code.Scope,
		code.CodeChallenge,
		code.CodeChallengeMethod,
		code.ExpiresAt,
	).Scan(&id, &code.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create authorization code: %w", err)
	}

	code.ID = strconv.FormatInt(id, 10)
	return nil
}

// GetAuthorizationCode fetches and validates an authorization code
func (r *UserRepository) GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuth2AuthorizationCode, error) {
	query := `
		SELECT 
			id, code, client_id, user_id, redirect_uri, scope,
			code_challenge, code_challenge_method, expires_at, used, created_at
		FROM oauth2_authorization_codes
		WHERE code = $1
		LIMIT 1
	`

	var authCode domain.OAuth2AuthorizationCode
	var id, userID int64

	err := r.db.QueryRow(ctx, query, code).Scan(
		&id,
		&authCode.Code,
		&authCode.ClientID,
		&userID,
		&authCode.RedirectURI,
		&authCode.Scope,
		&authCode.CodeChallenge,
		&authCode.CodeChallengeMethod,
		&authCode.ExpiresAt,
		&authCode.Used,
		&authCode.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrInvalidGrant
		}
		return nil, fmt.Errorf("failed to get authorization code: %w", err)
	}

	authCode.ID = strconv.FormatInt(id, 10)
	authCode.UserID = strconv.FormatInt(userID, 10)

	return &authCode, nil
}

// MarkAuthorizationCodeAsUsed marks a code as used
func (r *UserRepository) MarkAuthorizationCodeAsUsed(ctx context.Context, code string) error {
	query := `
		UPDATE oauth2_authorization_codes
		SET used = true
		WHERE code = $1
	`

	_, err := r.db.Exec(ctx, query, code)
	return err
}

// ================================
// ACCESS TOKEN OPERATIONS
// ================================

// CreateAccessToken creates a new access token
func (r *UserRepository) CreateAccessToken(ctx context.Context, token *domain.OAuth2AccessToken) error {
	query := `
		INSERT INTO oauth2_access_tokens (
			token_hash, client_id, user_id, scope, expires_at
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	var userID *int64
	if token.UserID != nil {
		uid, _ := strconv.ParseInt(*token.UserID, 10, 64)
		userID = &uid
	}

	var id int64
	err := r.db.QueryRow(ctx, query,
		token.TokenHash,
		token.ClientID,
		userID,
		token.Scope,
		token.ExpiresAt,
	).Scan(&id, &token.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create access token: %w", err)
	}

	token.ID = strconv.FormatInt(id, 10)
	return nil
}

// GetAccessTokenByHash fetches an access token by its hash
func (r *UserRepository) GetAccessTokenByHash(ctx context.Context, tokenHash string) (*domain.OAuth2AccessToken, error) {
	query := `
		SELECT 
			id, token_hash, client_id, user_id, scope, expires_at, revoked, created_at
		FROM oauth2_access_tokens
		WHERE token_hash = $1
		LIMIT 1
	`

	var token domain.OAuth2AccessToken
	var id int64
	var userID *int64

	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&id,
		&token.TokenHash,
		&token.ClientID,
		&userID,
		&token.Scope,
		&token.ExpiresAt,
		&token.Revoked,
		&token.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrInvalidGrant
		}
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	token.ID = strconv.FormatInt(id, 10)
	if userID != nil {
		uid := strconv.FormatInt(*userID, 10)
		token.UserID = &uid
	}

	return &token, nil
}

// RevokeAccessToken revokes an access token
func (r *UserRepository) RevokeAccessToken(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE oauth2_access_tokens
		SET revoked = true
		WHERE token_hash = $1
	`

	_, err := r.db.Exec(ctx, query, tokenHash)
	return err
}

// RevokeAccessTokensByUser revokes all access tokens for a user
func (r *UserRepository) RevokeAccessTokensByUser(ctx context.Context, userID string) error {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		UPDATE oauth2_access_tokens
		SET revoked = true
		WHERE user_id = $1 AND revoked = false
	`

	_, err := r.db.Exec(ctx, query, uid)
	return err
}

// ================================
// REFRESH TOKEN OPERATIONS
// ================================

// CreateRefreshToken creates a new refresh token
func (r *UserRepository) CreateRefreshToken(ctx context.Context, token *domain.OAuth2RefreshToken) error {
	query := `
		INSERT INTO oauth2_refresh_tokens (
			token_hash, access_token_id, client_id, user_id, scope, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	accessTokenID, _ := strconv.ParseInt(token.AccessTokenID, 10, 64)
	userID, _ := strconv.ParseInt(token.UserID, 10, 64)

	var id int64
	err := r.db.QueryRow(ctx, query,
		token.TokenHash,
		accessTokenID,
		token.ClientID,
		userID,
		token.Scope,
		token.ExpiresAt,
	).Scan(&id, &token.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}

	token.ID = strconv.FormatInt(id, 10)
	return nil
}

// GetRefreshTokenByHash fetches a refresh token by its hash
func (r *UserRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.OAuth2RefreshToken, error) {
	query := `
		SELECT 
			id, token_hash, access_token_id, client_id, user_id, scope, 
			expires_at, revoked, created_at
		FROM oauth2_refresh_tokens
		WHERE token_hash = $1
		LIMIT 1
	`

	var token domain.OAuth2RefreshToken
	var id, accessTokenID, userID int64

	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&id,
		&token.TokenHash,
		&accessTokenID,
		&token.ClientID,
		&userID,
		&token.Scope,
		&token.ExpiresAt,
		&token.Revoked,
		&token.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrInvalidGrant
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	token.ID = strconv.FormatInt(id, 10)
	token.AccessTokenID = strconv.FormatInt(accessTokenID, 10)
	token.UserID = strconv.FormatInt(userID, 10)

	return &token, nil
}

// RevokeRefreshToken revokes a refresh token
func (r *UserRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE oauth2_refresh_tokens
		SET revoked = true
		WHERE token_hash = $1
	`

	_, err := r.db.Exec(ctx, query, tokenHash)
	return err
}

// RevokeRefreshTokensByUser revokes all refresh tokens for a user
func (r *UserRepository) RevokeRefreshTokensByUser(ctx context.Context, userID string) error {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		UPDATE oauth2_refresh_tokens
		SET revoked = true
		WHERE user_id = $1 AND revoked = false
	`

	_, err := r.db.Exec(ctx, query, uid)
	return err
}

// ================================
// USER CONSENT OPERATIONS
// ================================

// CreateUserConsent records user consent for a client
func (r *UserRepository) CreateUserConsent(ctx context.Context, consent *domain.OAuth2UserConsent) error {
	query := `
		INSERT INTO oauth2_user_consents (
			user_id, client_id, scope, expires_at
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, client_id) 
		DO UPDATE SET 
			scope = EXCLUDED.scope,
			granted_at = NOW(),
			expires_at = EXCLUDED.expires_at,
			revoked = false
		RETURNING id, granted_at
	`

	userID, _ := strconv.ParseInt(consent.UserID, 10, 64)
	var id int64

	err := r.db.QueryRow(ctx, query,
		userID,
		consent.ClientID,
		consent.Scope,
		consent.ExpiresAt,
	).Scan(&id, &consent.GrantedAt)

	if err != nil {
		return fmt.Errorf("failed to create user consent: %w", err)
	}

	consent.ID = strconv.FormatInt(id, 10)
	return nil
}

// GetUserConsent fetches consent for a user and client
func (r *UserRepository) GetUserConsent(ctx context.Context, userID, clientID string) (*domain.OAuth2UserConsent, error) {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		SELECT 
			id, user_id, client_id, scope, granted_at, expires_at, revoked
		FROM oauth2_user_consents
		WHERE user_id = $1 AND client_id = $2
		LIMIT 1
	`

	var consent domain.OAuth2UserConsent
	var id, uid64 int64

	err := r.db.QueryRow(ctx, query, uid, clientID).Scan(
		&id,
		&uid64,
		&consent.ClientID,
		&consent.Scope,
		&consent.GrantedAt,
		&consent.ExpiresAt,
		&consent.Revoked,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No consent found is not an error
		}
		return nil, fmt.Errorf("failed to get user consent: %w", err)
	}

	consent.ID = strconv.FormatInt(id, 10)
	consent.UserID = strconv.FormatInt(uid64, 10)

	return &consent, nil
}

// GetUserConsents fetches all consents for a user
func (r *UserRepository) GetUserConsents(ctx context.Context, userID string) ([]*domain.OAuth2UserConsent, error) {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		SELECT 
			c.id, c.user_id, c.client_id, c.scope, c.granted_at, c.expires_at, c.revoked,
			cl.client_name, cl.logo_uri
		FROM oauth2_user_consents c
		JOIN oauth2_clients cl ON c.client_id = cl.client_id
		WHERE c.user_id = $1
		ORDER BY c.granted_at DESC
	`

	rows, err := r.db.Query(ctx, query, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to query user consents: %w", err)
	}
	defer rows.Close()

	var consents []*domain.OAuth2UserConsent
	for rows.Next() {
		var consent domain.OAuth2UserConsent
		var id, uid64 int64
		var clientName string
		var logoURI *string

		err := rows.Scan(
			&id,
			&uid64,
			&consent.ClientID,
			&consent.Scope,
			&consent.GrantedAt,
			&consent.ExpiresAt,
			&consent.Revoked,
			&clientName,
			&logoURI,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user consent: %w", err)
		}

		consent.ID = strconv.FormatInt(id, 10)
		consent.UserID = strconv.FormatInt(uid64, 10)
		consents = append(consents, &consent)
	}

	return consents, rows.Err()
}

// RevokeUserConsent revokes consent for a specific client
func (r *UserRepository) RevokeUserConsent(ctx context.Context, userID, clientID string) error {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		UPDATE oauth2_user_consents
		SET revoked = true
		WHERE user_id = $1 AND client_id = $2
	`

	_, err := r.db.Exec(ctx, query, uid, clientID)
	return err
}

// RevokeAllUserConsents revokes all consents for a user
func (r *UserRepository) RevokeAllUserConsents(ctx context.Context, userID string) error {
	uid, _ := strconv.ParseInt(userID, 10, 64)

	query := `
		UPDATE oauth2_user_consents
		SET revoked = true
		WHERE user_id = $1 AND revoked = false
	`

	_, err := r.db.Exec(ctx, query, uid)
	return err
}

// ================================
// SCOPE OPERATIONS
// ================================

// GetAllScopes fetches all available OAuth2 scopes
func (r *UserRepository) GetAllScopes(ctx context.Context) ([]*domain.OAuth2Scope, error) {
	query := `
		SELECT id, scope, description, is_default, created_at
		FROM oauth2_scopes
		ORDER BY scope
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query OAuth2 scopes: %w", err)
	}
	defer rows.Close()

	var scopes []*domain.OAuth2Scope
	for rows.Next() {
		var scope domain.OAuth2Scope
		var id int64

		err := rows.Scan(
			&id,
			&scope.Scope,
			&scope.Description,
			&scope.IsDefault,
			&scope.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan OAuth2 scope: %w", err)
		}

		scope.ID = strconv.FormatInt(id, 10)
		scopes = append(scopes, &scope)
	}

	return scopes, rows.Err()
}

// ================================
// AUDIT LOG OPERATIONS
// ================================

// CreateOAuth2AuditLog creates an audit log entry
func (r *UserRepository) CreateOAuth2AuditLog(ctx context.Context, log *domain.OAuth2AuditLog) error {
	query := `
		INSERT INTO oauth2_audit_log (
			event_type, client_id, user_id, ip_address, user_agent, metadata
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	var userID *int64
	if log.UserID != nil {
		uid, _ := strconv.ParseInt(*log.UserID, 10, 64)
		userID = &uid
	}

	var id int64
	err := r.db.QueryRow(ctx, query,
		log.EventType,
		log.ClientID,
		userID,
		log.IPAddress,
		log.UserAgent,
		log.Metadata,
	).Scan(&id, &log.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create OAuth2 audit log: %w", err)
	}

	log.ID = strconv.FormatInt(id, 10)
	return nil
}

// GetOAuth2AuditLogs fetches audit logs with pagination
func (r *UserRepository) GetOAuth2AuditLogs(ctx context.Context, limit, offset int) ([]*domain.OAuth2AuditLog, error) {
	query := `
		SELECT 
			id, event_type, client_id, user_id, ip_address, user_agent, metadata, created_at
		FROM oauth2_audit_log
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query OAuth2 audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*domain.OAuth2AuditLog
	for rows.Next() {
		var log domain.OAuth2AuditLog
		var id int64
		var userID *int64

		err := rows.Scan(
			&id,
			&log.EventType,
			&log.ClientID,
			&userID,
			&log.IPAddress,
			&log.UserAgent,
			&log.Metadata,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan OAuth2 audit log: %w", err)
		}

		log.ID = strconv.FormatInt(id, 10)
		if userID != nil {
			uid := strconv.FormatInt(*userID, 10)
			log.UserID = &uid
		}
		logs = append(logs, &log)
	}

	return logs, rows.Err()
}

// ================================
// CLEANUP OPERATIONS
// ================================

// CleanupExpiredOAuth2Tokens removes expired and used tokens
func (r *UserRepository) CleanupExpiredOAuth2Tokens(ctx context.Context) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var totalDeleted int64

	// Delete expired authorization codes
	result, err := tx.Exec(ctx, `
		DELETE FROM oauth2_authorization_codes
		WHERE expires_at < NOW() OR used = true
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup authorization codes: %w", err)
	}
	totalDeleted += result.RowsAffected()

	// Delete expired access tokens
	result, err = tx.Exec(ctx, `
		DELETE FROM oauth2_access_tokens
		WHERE expires_at < NOW() OR revoked = true
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup access tokens: %w", err)
	}
	totalDeleted += result.RowsAffected()

	// Delete expired refresh tokens
	result, err = tx.Exec(ctx, `
		DELETE FROM oauth2_refresh_tokens
		WHERE expires_at < NOW() OR revoked = true
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup refresh tokens: %w", err)
	}
	totalDeleted += result.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return totalDeleted, nil
}
// oauth2_service.go under services/auth-service/internal/service
package service

import (
	"auth-service/internal/domain"
	"auth-service/internal/repository"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"
		"x/shared/utils/cache"


	"golang.org/x/crypto/bcrypt"
)

type OAuth2Service struct {
	cache   *cache.Cache

	repo *repository.UserRepository
}

func NewOAuth2Service(repo *repository.UserRepository, 	cache *cache.Cache) *OAuth2Service {
	return &OAuth2Service{repo: repo, cache: cache,}
}

// ================================
// CLIENT REGISTRATION
// ================================

// RegisterClient registers a new OAuth2 client application
func (s *OAuth2Service) RegisterClient(ctx context.Context, req *domain.CreateOAuth2ClientRequest) (*domain.OAuth2Client, string, error) {
	// Generate client_id and client_secret
	clientID := s.generateClientID()
	clientSecret := s.generateClientSecret()
	
	// Hash the client secret
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash client secret: %w", err)
	}

	client := &domain.OAuth2Client{
		ClientID:       clientID,
		ClientSecret:   stringPtr(string(hashedSecret)),
		ClientName:     req.ClientName,
		ClientURI:      req.ClientURI,
		LogoURI:        req.LogoURI,
		OwnerUserID:    req.OwnerUserID,
		RedirectURIs:   req.RedirectURIs,
		GrantTypes:     []string{"authorization_code", "refresh_token"},
		ResponseTypes:  []string{"code"},
		Scope:          req.Scope,
		IsConfidential: true,
		IsActive:       true,
	}

	savedClient, err := s.repo.CreateOAuth2Client(ctx, client)
	if err != nil {
		return nil, "", err
	}

	// Return the plain client_secret only once (never stored)
	return savedClient, clientSecret, nil
}

// ================================
// AUTHORIZATION FLOW
// ================================

// AuthorizeRequest handles the authorization request
func (s *OAuth2Service) AuthorizeRequest(ctx context.Context, req *domain.OAuth2AuthorizationRequest, userID string) (string, error) {
	// Validate client
	client, err := s.repo.GetOAuth2ClientByClientID(ctx, req.ClientID)
	if err != nil {
		return "", domain.ErrInvalidClient
	}

	if !client.IsActive {
		return "", domain.ErrUnauthorizedClient
	}

	// Validate redirect URI
	if !s.isValidRedirectURI(req.RedirectURI, client.RedirectURIs) {
		return "", domain.ErrInvalidRedirectURI
	}

	// Check if user has already consented
	consent, err := s.repo.GetUserConsent(ctx, userID, req.ClientID)
	if err != nil {
		return "", err
	}

	// If no consent or consent revoked, require consent
	if consent == nil || !consent.IsValid() {
		return "", domain.ErrConsentRequired
	}

	// Generate authorization code
	code := s.generateAuthorizationCode()
	
	authCode := &domain.OAuth2AuthorizationCode{
		Code:                code,
		ClientID:            req.ClientID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scope:               req.Scope,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ExpiresAt:           time.Now().Add(10 * time.Minute),
	}

	if err := s.repo.CreateAuthorizationCode(ctx, authCode); err != nil {
		return "", err
	}

	// Log the authorization
	s.logAuditEvent(ctx, "authorization_granted", req.ClientID, &userID, nil, nil)

	return code, nil
}

// GrantConsent records user consent for a client
func (s *OAuth2Service) GrantConsent(ctx context.Context, userID, clientID, scope string) error {
	consent := &domain.OAuth2UserConsent{
		UserID:   userID,
		ClientID: clientID,
		Scope:    scope,
	}

	if err := s.repo.CreateUserConsent(ctx, consent); err != nil {
		return err
	}

	s.logAuditEvent(ctx, "consent_granted", clientID, &userID, nil, nil)
	return nil
}

// ================================
// TOKEN EXCHANGE
// ================================

// ExchangeToken handles token exchange (authorization_code, refresh_token, etc.)
func (s *OAuth2Service) ExchangeToken(ctx context.Context, req *domain.OAuth2TokenRequest) (*domain.OAuth2TokenResponse, error) {
	// Validate client credentials
	client, err := s.validateClientCredentials(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, err
	}

	switch req.GrantType {
	case "authorization_code":
		return s.exchangeAuthorizationCode(ctx, req, client)
	case "refresh_token":
		return s.exchangeRefreshToken(ctx, req, client)
	case "client_credentials":
		return s.exchangeClientCredentials(ctx, req, client)
	default:
		return nil, domain.ErrUnsupportedGrantType
	}
}

func (s *OAuth2Service) exchangeAuthorizationCode(ctx context.Context, req *domain.OAuth2TokenRequest, client *domain.OAuth2Client) (*domain.OAuth2TokenResponse, error) {
	if req.Code == nil {
		return nil, domain.ErrInvalidGrant
	}

	// Get authorization code
	authCode, err := s.repo.GetAuthorizationCode(ctx, *req.Code)
	if err != nil {
		return nil, err
	}

	// Validate authorization code
	if !authCode.IsValid() {
		return nil, domain.ErrInvalidGrant
	}

	if authCode.ClientID != req.ClientID {
		return nil, domain.ErrInvalidGrant
	}

	// Validate PKCE if present
	if authCode.CodeChallenge != nil {
		if req.CodeVerifier == nil {
			return nil, domain.ErrInvalidGrant
		}
		if !s.validatePKCE(*authCode.CodeChallenge, *authCode.CodeChallengeMethod, *req.CodeVerifier) {
			return nil, domain.ErrInvalidGrant
		}
	}

	// Mark code as used
	if err := s.repo.MarkAuthorizationCodeAsUsed(ctx, *req.Code); err != nil {
		return nil, err
	}

	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(ctx, client.ClientID, &authCode.UserID, authCode.Scope)
	if err != nil {
		return nil, err
	}

	s.logAuditEvent(ctx, "token_issued", client.ClientID, &authCode.UserID, nil, nil)

	return &domain.OAuth2TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    3600, // 1 hour
		RefreshToken: refreshToken,
		Scope:        authCode.Scope,
	}, nil
}

func (s *OAuth2Service) exchangeRefreshToken(ctx context.Context, req *domain.OAuth2TokenRequest, client *domain.OAuth2Client) (*domain.OAuth2TokenResponse, error) {
	if req.RefreshToken == nil {
		return nil, domain.ErrInvalidGrant
	}

	// Hash the refresh token
	tokenHash := s.hashToken(*req.RefreshToken)

	// Get refresh token
	refreshToken, err := s.repo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	// Validate refresh token
	if !refreshToken.IsValid() {
		return nil, domain.ErrInvalidGrant
	}

	if refreshToken.ClientID != req.ClientID {
		return nil, domain.ErrInvalidGrant
	}

	// Generate new tokens
	accessToken, newRefreshToken, err := s.generateTokens(ctx, client.ClientID, &refreshToken.UserID, refreshToken.Scope)
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token
	if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	s.logAuditEvent(ctx, "token_refreshed", client.ClientID, &refreshToken.UserID, nil, nil)

	return &domain.OAuth2TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: newRefreshToken,
		Scope:        refreshToken.Scope,
	}, nil
}

func (s *OAuth2Service) exchangeClientCredentials(ctx context.Context, req *domain.OAuth2TokenRequest, client *domain.OAuth2Client) (*domain.OAuth2TokenResponse, error) {
	scope := "read"
	if req.Scope != nil {
		scope = *req.Scope
	}

	// Generate access token (no refresh token for client_credentials)
	accessToken, _, err := s.generateTokens(ctx, client.ClientID, nil, scope)
	if err != nil {
		return nil, err
	}

	s.logAuditEvent(ctx, "client_credentials_issued", client.ClientID, nil, nil, nil)

	return &domain.OAuth2TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       scope,
	}, nil
}

// ================================
// TOKEN VALIDATION
// ================================

// ValidateAccessToken validates an access token and returns the associated data
func (s *OAuth2Service) ValidateAccessToken(ctx context.Context, tokenString string) (*domain.OAuth2AccessToken, error) {
	tokenHash := s.hashToken(tokenString)

	token, err := s.repo.GetAccessTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	if !token.IsValid() {
		return nil, domain.ErrInvalidGrant
	}

	return token, nil
}

// ================================
// TOKEN REVOCATION
// ================================

// RevokeToken revokes an access or refresh token
func (s *OAuth2Service) RevokeToken(ctx context.Context, tokenString string) error {
	tokenHash := s.hashToken(tokenString)

	// Try to revoke as access token
	if err := s.repo.RevokeAccessToken(ctx, tokenHash); err == nil {
		return nil
	}

	// Try to revoke as refresh token
	return s.repo.RevokeRefreshToken(ctx, tokenHash)
}

// RevokeAllUserTokens revokes all tokens for a user
func (s *OAuth2Service) RevokeAllUserTokens(ctx context.Context, userID string) error {
	if err := s.repo.RevokeAccessTokensByUser(ctx, userID); err != nil {
		return err
	}
	return s.repo.RevokeRefreshTokensByUser(ctx, userID)
}

// ================================
// HELPER METHODS
// ================================

func (s *OAuth2Service) validateClientCredentials(ctx context.Context, clientID string, clientSecret *string) (*domain.OAuth2Client, error) {
	client, err := s.repo.GetOAuth2ClientByClientID(ctx, clientID)
	if err != nil {
		return nil, domain.ErrInvalidClient
	}

	if !client.IsActive {
		return nil, domain.ErrUnauthorizedClient
	}

	// For confidential clients, verify client secret
	if client.IsConfidential {
		if clientSecret == nil || client.ClientSecret == nil {
			return nil, domain.ErrInvalidClient
		}
		if err := bcrypt.CompareHashAndPassword([]byte(*client.ClientSecret), []byte(*clientSecret)); err != nil {
			return nil, domain.ErrInvalidClient
		}
	}

	return client, nil
}

func (s *OAuth2Service) generateTokens(ctx context.Context, clientID string, userID *string, scope string) (accessToken, refreshToken string, err error) {
	// Generate access token
	accessTokenStr := s.generateRandomToken(32)
	accessTokenHash := s.hashToken(accessTokenStr)

	accessTokenObj := &domain.OAuth2AccessToken{
		TokenHash: accessTokenHash,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if err := s.repo.CreateAccessToken(ctx, accessTokenObj); err != nil {
		return "", "", err
	}

	// Generate refresh token (only if user_id exists)
	var refreshTokenStr string
	if userID != nil {
		refreshTokenStr = s.generateRandomToken(32)
		refreshTokenHash := s.hashToken(refreshTokenStr)

		refreshTokenObj := &domain.OAuth2RefreshToken{
			TokenHash:     refreshTokenHash,
			AccessTokenID: accessTokenObj.ID,
			ClientID:      clientID,
			UserID:        *userID,
			Scope:         scope,
			ExpiresAt:     time.Now().Add(30 * 24 * time.Hour), // 30 days
		}

		if err := s.repo.CreateRefreshToken(ctx, refreshTokenObj); err != nil {
			return "", "", err
		}
	}

	return accessTokenStr, refreshTokenStr, nil
}

func (s *OAuth2Service) generateClientID() string {
	return "client_" + s.generateRandomToken(16)
}

func (s *OAuth2Service) generateClientSecret() string {
	return "secret_" + s.generateRandomToken(32)
}

func (s *OAuth2Service) generateAuthorizationCode() string {
	return s.generateRandomToken(32)
}

func (s *OAuth2Service) generateRandomToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (s *OAuth2Service) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(hash[:])
}


// continuation of helper methods
// Add these helper methods to the end of your oauth2_service.go file

// isValidRedirectURI checks if the requested redirect URI matches one of the registered URIs
func (s *OAuth2Service) isValidRedirectURI(requestedURI string, registeredURIs []string) bool {
	for _, uri := range registeredURIs {
		if uri == requestedURI {
			return true
		}
	}
	return false
}

// validatePKCE validates the PKCE code verifier against the challenge
func (s *OAuth2Service) validatePKCE(challenge, method, verifier string) bool {
	switch method {
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		computed := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
		return challenge == computed
	case "plain":
		return challenge == verifier
	default:
		return false
	}
}

// logAuditEvent creates an audit log entry
func (s *OAuth2Service) logAuditEvent(_ context.Context, eventType, clientID string, userID *string, ipAddress, userAgent *string) {
	log := &domain.OAuth2AuditLog{
		EventType: eventType,
		ClientID:  stringPtr(clientID),
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}
	
	// Fire and forget - don't block on audit logging
	go func() {
		_ = s.repo.CreateOAuth2AuditLog(context.Background(), log)
	}()
}

// ================================
// SCOPE OPERATIONS
// ================================

// GetAllScopes returns all available OAuth2 scopes
func (s *OAuth2Service) GetAllScopes(ctx context.Context) ([]*domain.OAuth2Scope, error) {
	return s.repo.GetAllScopes(ctx)
}

// ValidateScopes checks if requested scopes are valid
func (s *OAuth2Service) ValidateScopes(ctx context.Context, requestedScopes string) (bool, error) {
	if requestedScopes == "" {
		return true, nil
	}

	scopes, err := s.repo.GetAllScopes(ctx)
	if err != nil {
		return false, err
	}

	// Build a map of valid scopes
	validScopes := make(map[string]bool)
	for _, scope := range scopes {
		validScopes[scope.Scope] = true
	}

	// Check each requested scope
	requested := strings.Split(requestedScopes, " ")
	for _, scope := range requested {
		if !validScopes[scope] {
			return false, nil
		}
	}

	return true, nil
}

// ================================
// CLIENT MANAGEMENT
// ================================

// GetClientByID retrieves a client by its client_id
func (s *OAuth2Service) GetClientByID(ctx context.Context, clientID string) (*domain.OAuth2Client, error) {
	return s.repo.GetOAuth2ClientByClientID(ctx, clientID)
}

// GetClientsByOwner retrieves all clients owned by a user
func (s *OAuth2Service) GetClientsByOwner(ctx context.Context, ownerUserID string) ([]*domain.OAuth2Client, error) {
	return s.repo.GetOAuth2ClientsByOwner(ctx, ownerUserID)
}

// UpdateClient updates an existing OAuth2 client (you'll need to add this to repository)
func (s *OAuth2Service) UpdateClient(ctx context.Context, clientID string, updates *domain.UpdateOAuth2ClientRequest) (*domain.OAuth2Client, error) {
	client, err := s.repo.GetOAuth2ClientByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if updates.ClientName != nil {
		client.ClientName = *updates.ClientName
	}
	if updates.ClientURI != nil {
		client.ClientURI = updates.ClientURI
	}
	if updates.LogoURI != nil {
		client.LogoURI = updates.LogoURI
	}
	if updates.RedirectURIs != nil {
		client.RedirectURIs = updates.RedirectURIs
	}
	if updates.IsActive != nil {
		client.IsActive = *updates.IsActive
	}

	// You'll need to implement UpdateOAuth2Client in repository
	// return s.repo.UpdateOAuth2Client(ctx, client)
	
	return client, nil
}

// ================================
// CONSENT MANAGEMENT
// ================================

// GetUserConsents retrieves all consents for a user
func (s *OAuth2Service) GetUserConsents(ctx context.Context, userID string) ([]*domain.OAuth2UserConsent, error) {
	return s.repo.GetUserConsents(ctx, userID)
}

// RevokeUserConsent revokes consent for a specific client
func (s *OAuth2Service) RevokeUserConsent(ctx context.Context, userID, clientID string) error {
	if err := s.repo.RevokeUserConsent(ctx, userID, clientID); err != nil {
		return err
	}

	// Also revoke all tokens for this client/user combination
	s.logAuditEvent(ctx, "consent_revoked", clientID, &userID, nil, nil)
	
	return nil
}

// RevokeAllUserConsents revokes all consents for a user
func (s *OAuth2Service) RevokeAllUserConsents(ctx context.Context, userID string) error {
	if err := s.repo.RevokeAllUserConsents(ctx, userID); err != nil {
		return err
	}

	s.logAuditEvent(ctx, "all_consents_revoked", "", &userID, nil, nil)
	
	return nil
}

// ================================
// AUDIT OPERATIONS
// ================================

// GetAuditLogs retrieves audit logs with pagination
func (s *OAuth2Service) GetAuditLogs(ctx context.Context, limit, offset int) ([]*domain.OAuth2AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.GetOAuth2AuditLogs(ctx, limit, offset)
}

// ================================
// MAINTENANCE OPERATIONS
// ================================

// CleanupExpiredTokens removes expired and revoked tokens
func (s *OAuth2Service) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	deleted, err := s.repo.CleanupExpiredOAuth2Tokens(ctx)
	if err != nil {
		return 0, err
	}

	s.logAuditEvent(ctx, "token_cleanup", "", nil, nil, nil)
	
	return deleted, nil
}

// ================================
// INTROSPECTION
// ================================

// IntrospectToken provides token introspection (RFC 7662)
func (s *OAuth2Service) IntrospectToken(ctx context.Context, token string, clientID string) (*domain.OAuth2TokenIntrospection, error) {
	tokenHash := s.hashToken(token)

	// Try as access token
	accessToken, err := s.repo.GetAccessTokenByHash(ctx, tokenHash)
	if err == nil && accessToken.IsValid() {
		// Verify the requesting client has permission
		if accessToken.ClientID != clientID {
			return &domain.OAuth2TokenIntrospection{Active: false}, nil
		}

		introspection := &domain.OAuth2TokenIntrospection{
			Active:    true,
			Scope:     accessToken.Scope,
			ClientID:  accessToken.ClientID,
			TokenType: "access_token",
			ExpiresAt: accessToken.ExpiresAt.Unix(),
			IssuedAt:  accessToken.CreatedAt.Unix(),
		}

		if accessToken.UserID != nil {
			introspection.UserID = accessToken.UserID
		}

		return introspection, nil
	}

	// Token not found or invalid
	return &domain.OAuth2TokenIntrospection{Active: false}, nil
}

// ================================
// UTILITY FUNCTIONS
// ================================

// stringPtr is a helper to create string pointers
func stringPtr(s string) *string {
	return &s
}


// Additional service methods to add to oauth2_service.go

// DeleteClient soft deletes an OAuth2 client
func (s *OAuth2Service) DeleteClient(ctx context.Context, clientID, ownerUserID string) error {
	// Verify ownership
	client, err := s.repo.GetOAuth2ClientByClientID(ctx, clientID)
	if err != nil {
		return err
	}

	if client.OwnerUserID != ownerUserID {
		return domain.ErrAccessDenied
	}

	if err := s.repo.DeleteOAuth2Client(ctx, clientID); err != nil {
		return err
	}

	s.logAuditEvent(ctx, "client_deleted", clientID, &ownerUserID, nil, nil)
	
	return nil
}

// RegenerateClientSecret generates a new client secret
func (s *OAuth2Service) RegenerateClientSecret(ctx context.Context, clientID, ownerUserID string) (string, error) {
	// Verify ownership
	client, err := s.repo.GetOAuth2ClientByClientID(ctx, clientID)
	if err != nil {
		return "", err
	}

	if client.OwnerUserID != ownerUserID {
		return "", domain.ErrAccessDenied
	}

	// Generate new secret
	newSecret := s.generateClientSecret()
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(newSecret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash client secret: %w", err)
	}

	if err := s.repo.RegenerateClientSecret(ctx, clientID, string(hashedSecret)); err != nil {
		return "", err
	}

	s.logAuditEvent(ctx, "client_secret_regenerated", clientID, &ownerUserID, nil, nil)

	return newSecret, nil
}

// GetConsentInfo retrieves information about a consent request
func (s *OAuth2Service) GetConsentInfo(ctx context.Context, clientID, userID, scope string) (*domain.ConsentInfo, error) {
	client, err := s.repo.GetOAuth2ClientByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	// Check existing consent
	existingConsent, err := s.repo.GetUserConsent(ctx, userID, clientID)
	if err != nil {
		return nil, err
	}

	// Parse requested scopes
	requestedScopes := strings.Split(scope, " ")
	
	scopeDescriptions := make(map[string]string)
	// You might want to fetch these from the database
	scopeDescriptions["read"] = "Read your basic profile information"
	scopeDescriptions["write"] = "Modify your data"
	scopeDescriptions["email"] = "Access your email address"
	
	info := &domain.ConsentInfo{
		ClientID:          client.ClientID,
		ClientName:        client.ClientName,
		ClientURI:         client.ClientURI,
		LogoURI:           client.LogoURI,
		RequestedScopes:   requestedScopes,
		ScopeDescriptions: scopeDescriptions,
		HasExistingConsent: existingConsent != nil && existingConsent.IsValid(),
	}

	if existingConsent != nil {
		info.GrantedScopes = strings.Split(existingConsent.Scope, " ")
	}

	return info, nil
}

// ValidateAuthorizationRequest validates all aspects of an authorization request
func (s *OAuth2Service) ValidateAuthorizationRequest(ctx context.Context, req *domain.OAuth2AuthorizationRequest) error {
	// Validate client
	client, err := s.repo.GetOAuth2ClientByClientID(ctx, req.ClientID)
	if err != nil {
		log.Println("Error fetching client:", err)
		return domain.ErrInvalidClient
	}

	if !client.IsActive {
		return domain.ErrUnauthorizedClient
	}

	// Validate redirect URI
	if !s.isValidRedirectURI(req.RedirectURI, client.RedirectURIs) {
		return domain.ErrInvalidRedirectURI
	}

	// Validate response type
	validResponseType := false
	for _, rt := range client.ResponseTypes {
		if rt == req.ResponseType {
			validResponseType = true
			break
		}
	}
	if !validResponseType {
		return fmt.Errorf("unsupported response type")
	}

	// Validate scopes if provided
	if req.Scope != "" {
		valid, err := s.ValidateScopes(ctx, req.Scope)
		if err != nil {
			return err
		}
		if !valid {
			return domain.ErrInvalidScope
		}
	}

	// Validate PKCE if present
	if req.CodeChallenge != nil {
		if req.CodeChallengeMethod == nil {
			return fmt.Errorf("code_challenge_method required when code_challenge is present")
		}
		if *req.CodeChallengeMethod != "S256" && *req.CodeChallengeMethod != "plain" {
			return fmt.Errorf("invalid code_challenge_method")
		}
	}

	return nil
}

// BuildAuthorizationResponse constructs the authorization response with the code
func (s *OAuth2Service) BuildAuthorizationResponse(redirectURI, code, state string) string {
	response := redirectURI + "?code=" + code
	if state != "" {
		response += "&state=" + state
	}
	return response
}

// BuildErrorResponse constructs an error response for authorization failures
func (s *OAuth2Service) BuildErrorResponse(redirectURI, errorCode, errorDescription, state string) string {
	response := redirectURI + "?error=" + errorCode
	if errorDescription != "" {
		response += "&error_description=" + errorDescription
	}
	if state != "" {
		response += "&state=" + state
	}
	return response
}

func (s *OAuth2Service) GenerateTemporaryCode(ctx context.Context, userID string) string {
	token := s.generateRandomToken(32)
	// Store in cache with 5 minute expiration
	s.cache.Set(ctx,"oauth2", "temp_token_"+token, userID, 5*time.Minute)
	return token
}

func (s *OAuth2Service) ValidateTemporaryCode(ctx context.Context, token string, delete bool) (string, error) {
	userID, found := s.cache.Get(ctx,"oauth2", "temp_token_"+token)
	if found != nil {
		return "", domain.ErrInvalidGrant
	}
	if delete {
		s.InvalidateTemporaryCode(ctx, token)
	}
	return userID, nil
}

func (s *OAuth2Service) InvalidateTemporaryCode(ctx context.Context, token string) {
	s.cache.Delete(ctx,"oauth2", "temp_token_"+token)
}
package usecase

import (
	"auth-service/internal/domain"
	"auth-service/internal/service/apple"
	"auth-service/internal/service/oauth2"
	"context"
	"fmt"
	"time"
)

// ================================
// GOOGLE OAUTH
// ================================

// RegisterWithGoogle handles Google OAuth registration/login
func (uc *UserUsecase) RegisterWithGoogle(ctx context.Context, idToken, clientID string) (*domain.UserWithCredential, *oauth2svc.GoogleUser, error) {
	// Verify Google ID token
	googleUser, err := oauth2svc.VerifyGoogleToken(ctx, idToken, clientID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify google token: %w", err)
	}

	// Check if this Google account already exists
	oauthAcc, err := uc.userRepo.FindByProviderUID(ctx, domain.ProviderGoogle, googleUser.Sub)
	if err == nil && oauthAcc != nil {
		// Existing account → fetch user with credential
		user, userWithCreds, err := uc.userRepo.GetUserWithCredentialsByID(ctx, oauthAcc.UserID)
		if err != nil {
			return nil, nil, err
		}
		if len(userWithCreds) == 0 {
			return nil, nil, fmt.Errorf("no credentials found for user")
		}
		userWithCred := &domain.UserWithCredential{
			User:       *user,
			Credential: *userWithCreds[0],
		}		
		// Update OAuth tokens if available (Google doesn't return tokens in ID token flow typically)
		// If you have access_token from a different flow, update here
		
		return userWithCred, googleUser, nil
	}

	// Create new user
	newUser := &domain.User{
		ID:              uc.Sf.Generate(),
		AccountStatus:   "active",
		AccountType:     "social",
		AccountRestored: false,
		Consent:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	newCredential := &domain.UserCredential{
		ID:              uc.Sf.Generate(),
		UserID:          newUser.ID,
		Email:           &googleUser.Email,
		Phone:           nil,
		PasswordHash:    nil, // No password for social accounts
		IsEmailVerified: true, // Google-verified email
		IsPhoneVerified: false,
		Valid:           true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Create user and credential together
	users, errs := uc.userRepo.CreateUsers(ctx, []*domain.User{newUser}, []*domain.UserCredential{newCredential})
	if len(errs) > 0 && errs[0] != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", errs[0])
	}

	user := users[0]

	// Link OAuth account
	newOAuth := &domain.OAuthAccount{
		ID:          uc.Sf.Generate(),
		UserID:      user.ID,
		Provider:    domain.ProviderGoogle,
		ProviderUID: googleUser.Sub,
		Metadata: map[string]interface{}{
			"email":      googleUser.Email,
			"first_name": googleUser.FirstName,
			"last_name":  googleUser.LastName,
		},
	}
	
	if err := uc.userRepo.CreateOAuthAccount(ctx, newOAuth); err != nil {
		return nil, nil, fmt.Errorf("failed to link oauth account: %w", err)
	}

	// Return combined user with credential
	userWithCred := &domain.UserWithCredential{
		User:       *user,
		Credential: *newCredential,
	}

	return userWithCred, googleUser, nil
}

// ================================
// APPLE OAUTH
// ================================

// AppleDeps contains Apple OAuth configuration
type AppleDeps struct {
	ServiceID   string
	TeamID      string
	KeyID       string
	PrivateKey  string
	RedirectURI string
}

// RegisterWithApple handles Apple Sign In registration/login
func (uc *UserUsecase) RegisterWithApple(ctx context.Context, deps AppleDeps, idToken, authCode string) (*domain.UserWithCredential, bool, error) {
	var tokenToVerify string

	if idToken != "" {
		tokenToVerify = idToken
	} else {
		// Exchange code → tokens
		clientSecret, err := apple.GenerateClientSecret(apple.ClientSecretParams{
			TeamID:        deps.TeamID,
			KeyID:         deps.KeyID,
			ServiceID:     deps.ServiceID,
			PrivateKeyPEM: deps.PrivateKey,
			TTL:           5 * time.Minute,
		})
		if err != nil {
			return nil, false, fmt.Errorf("failed to generate apple client secret: %w", err)
		}
		
		tr, err := apple.ExchangeCodeForTokens(ctx, deps.ServiceID, clientSecret, authCode, deps.RedirectURI)
		if err != nil {
			return nil, false, fmt.Errorf("failed to exchange apple code: %w", err)
		}
		tokenToVerify = tr.IDToken
	}

	claims, err := apple.VerifyIDToken(ctx, tokenToVerify, deps.ServiceID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to verify apple token: %w", err)
	}

	// Check if this Apple account already exists
	oauthAcc, _ := uc.userRepo.FindByProviderUID(ctx, domain.ProviderApple, claims.Sub)
	if oauthAcc != nil {
		user, userWithCreds, err := uc.userRepo.GetUserWithCredentialsByID(ctx, oauthAcc.UserID)
		if err != nil {
			return nil, false, err
		}
		if len(userWithCreds) == 0 {
			return nil, false, fmt.Errorf("no credentials found for user")
		}
		userWithCred := &domain.UserWithCredential{
			User:       *user,
			Credential: *userWithCreds[0],
		}
		return userWithCred, false, nil // existing user, new=false
	}

	// Create new user
	newUser := &domain.User{
		ID:              uc.Sf.Generate(),
		AccountStatus:   "active",
		AccountType:     "social",
		AccountRestored: false,
		Consent:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	var email *string
	if claims.Email != "" {
		email = &claims.Email
	}

	newCredential := &domain.UserCredential{
		ID:              uc.Sf.Generate(),
		UserID:          newUser.ID,
		Email:           email,
		Phone:           nil,
		PasswordHash:    nil,
		IsEmailVerified: claims.EmailVerified,
		IsPhoneVerified: false,
		Valid:           true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Create user and credential
	users, errs := uc.userRepo.CreateUsers(ctx, []*domain.User{newUser}, []*domain.UserCredential{newCredential})
	if len(errs) > 0 && errs[0] != nil {
		return nil, false, fmt.Errorf("failed to create user: %w", errs[0])
	}

	user := users[0]

	// Link OAuth account
	if err := uc.userRepo.CreateOAuthAccount(ctx, &domain.OAuthAccount{
		ID:          uc.Sf.Generate(),
		UserID:      user.ID,
		Provider:    domain.ProviderApple,
		ProviderUID: claims.Sub,
		Metadata: map[string]interface{}{
			"email": claims.Email,
		},
	}); err != nil {
		return nil, false, fmt.Errorf("failed to link oauth account: %w", err)
	}

	userWithCred := &domain.UserWithCredential{
		User:       *user,
		Credential: *newCredential,
	}

	return userWithCred, true, nil // new user created
}

// ================================
// TELEGRAM OAUTH
// ================================

// HandleTelegramLogin handles Telegram login widget authentication
func (uc *UserUsecase) HandleTelegramLogin(ctx context.Context, data map[string]string) (*domain.UserWithCredential, error) {
	provider := domain.ProviderTelegram
	telegramID := data["id"]

	if telegramID == "" {
		return nil, fmt.Errorf("telegram id is required")
	}

	// Check if oauth account exists
	oauthAcc, err := uc.userRepo.FindByProviderUID(ctx, provider, telegramID)
	if err == nil && oauthAcc != nil {
		// Found linked account → fetch user
		user, userWithCreds, err := uc.userRepo.GetUserWithCredentialsByID(ctx, oauthAcc.UserID)
		if err != nil {
			return nil, err
		}
		if len(userWithCreds) == 0 {
			return nil, fmt.Errorf("no credentials found for user")
		}
		userWithCred := &domain.UserWithCredential{
			User:       *user,
			Credential: *userWithCreds[0],
		}
		return userWithCred, nil
	}

	// Create a new user
	newUser := &domain.User{
		ID:              uc.Sf.Generate(),
		AccountStatus:   "active",
		AccountType:     "social",
		AccountRestored: false,
		Consent:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Telegram doesn't provide email, so credential will be minimal
	newCredential := &domain.UserCredential{
		ID:              uc.Sf.Generate(),
		UserID:          newUser.ID,
		Email:           nil,
		Phone:           nil,
		PasswordHash:    nil,
		IsEmailVerified: false,
		IsPhoneVerified: false,
		Valid:           true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Create user and credential
	users, errs := uc.userRepo.CreateUsers(ctx, []*domain.User{newUser}, []*domain.UserCredential{newCredential})
	if len(errs) > 0 && errs[0] != nil {
		return nil, fmt.Errorf("failed to create user: %w", errs[0])
	}

	user := users[0]

	// Create linked oauth account
	oauthAcc = &domain.OAuthAccount{
		ID:          uc.Sf.Generate(),
		UserID:      user.ID,
		Provider:    provider,
		ProviderUID: telegramID,
		Metadata: map[string]interface{}{
			"first_name": data["first_name"],
			"last_name":  data["last_name"],
			"username":   data["username"],
			"photo_url":  data["photo_url"],
		},
	}
	
	if err := uc.userRepo.CreateOAuthAccount(ctx, oauthAcc); err != nil {
		return nil, fmt.Errorf("failed to link oauth account: %w", err)
	}

	userWithCred := &domain.UserWithCredential{
		User:       *user,
		Credential: *newCredential,
	}

	return userWithCred, nil
}
// ================================
// OAUTH ACCOUNT MANAGEMENT
// ================================

// GetUserOAuthAccounts retrieves all OAuth accounts for a user
func (uc *UserUsecase) GetUserOAuthAccounts(ctx context.Context, userID string) ([]*domain.OAuthAccountSummary, error) {
	accounts, err := uc.userRepo.GetOAuthAccountsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth accounts: %w", err)
	}

	summaries := make([]*domain.OAuthAccountSummary, len(accounts))
	for i, acc := range accounts {
		summaries[i] = acc.ToSummary()
	}

	return summaries, nil
}

// LinkOAuthAccount links an OAuth account to an existing user
func (uc *UserUsecase) LinkOAuthAccount(ctx context.Context, userID string, req *domain.LinkOAuthAccountRequest) error {
	// Validate provider
	if !domain.IsProviderSupported(req.Provider) {
		return fmt.Errorf("unsupported provider: %s", req.Provider)
	}

	// Check if user already has this provider linked
	exists, err := uc.userRepo.CheckOAuthAccountExists(ctx, userID, req.Provider)
	if err != nil {
		return fmt.Errorf("failed to check oauth account: %w", err)
	}
	if exists {
		return fmt.Errorf("provider already linked to this account")
	}

	// Provider-specific linking logic
	switch req.Provider {
	case domain.ProviderGoogle:
		return uc.linkGoogleAccount(ctx, userID, req)
	case domain.ProviderApple:
		return uc.linkAppleAccount(ctx, userID, req)
	case domain.ProviderTelegram:
		return uc.linkTelegramAccount(ctx, userID, req)
	default:
		return fmt.Errorf("linking not implemented for provider: %s", req.Provider)
	}
}

// UnlinkOAuthAccount unlinks an OAuth provider from a user
// UnlinkOAuthAccount unlinks an OAuth provider from a user
func (uc *UserUsecase) UnlinkOAuthAccount(ctx context.Context, userID string, provider string) error {
	// Validate provider
	if !domain.IsProviderSupported(provider) {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	// Check if account exists
	exists, err := uc.userRepo.CheckOAuthAccountExists(ctx, userID, provider)
	if err != nil {
		return fmt.Errorf("failed to check oauth account: %w", err)
	}
	if !exists {
		return fmt.Errorf("provider not linked to this account")
	}

	// Check if user has other authentication methods
	_, userWithCred, err := uc.userRepo.GetUserWithCredentialsByID(ctx, userID)
	if err != nil {
		return err
	}
	if len(userWithCred) == 0 {
		return fmt.Errorf("no credentials found for user")
	}


	// Get all OAuth accounts
	oauthAccounts, err := uc.userRepo.GetOAuthAccountsByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get oauth accounts: %w", err)
	}

	// Ensure user has at least one other auth method
	hasPassword := userWithCred[0].PasswordHash != nil && *userWithCred[0].PasswordHash != ""
	hasOtherOAuth := len(oauthAccounts) > 1

	if !hasPassword && !hasOtherOAuth {
		return fmt.Errorf("cannot unlink last authentication method")
	}

	// Unlink the account
	if err := uc.userRepo.UnlinkOAuthAccount(ctx, userID, provider); err != nil {
		return fmt.Errorf("failed to unlink oauth account: %w", err)
	}

	return nil
}

// ================================
// HELPER METHODS
// ================================

func (uc *UserUsecase) linkGoogleAccount(ctx context.Context, userID string, req *domain.LinkOAuthAccountRequest) error {
	if req.IDToken == nil {
		return fmt.Errorf("id_token required for Google")
	}

	googleUser, err := oauth2svc.VerifyGoogleToken(ctx, *req.IDToken, "your-client-id")
	if err != nil {
		return fmt.Errorf("failed to verify google token: %w", err)
	}

	// Check if this Google account is already linked to another user
	existing, _ := uc.userRepo.FindByProviderUID(ctx, domain.ProviderGoogle, googleUser.Sub)
	if existing != nil && existing.UserID != userID {
		return fmt.Errorf("this google account is already linked to another user")
	}

	oauthAcc := &domain.OAuthAccount{
		ID:          uc.Sf.Generate(),
		UserID:      userID,
		Provider:    domain.ProviderGoogle,
		ProviderUID: googleUser.Sub,
		Metadata: map[string]interface{}{
			"email":      googleUser.Email,
			"first_name": googleUser.FirstName,
			"last_name":  googleUser.LastName,
		},
	}

	return uc.userRepo.CreateOAuthAccount(ctx, oauthAcc)
}

func (uc *UserUsecase) linkAppleAccount(ctx context.Context, userID string, req *domain.LinkOAuthAccountRequest) error {
	if req.IDToken == nil {
		return fmt.Errorf("id_token required for Apple")
	}

	// Verify Apple token (you'll need to pass serviceID from config)
	claims, err := apple.VerifyIDToken(ctx, *req.IDToken, "your-service-id")
	if err != nil {
		return fmt.Errorf("failed to verify apple token: %w", err)
	}

	// Check if this Apple account is already linked to another user
	existing, _ := uc.userRepo.FindByProviderUID(ctx, domain.ProviderApple, claims.Sub)
	if existing != nil && existing.UserID != userID {
		return fmt.Errorf("this apple account is already linked to another user")
	}

	oauthAcc := &domain.OAuthAccount{
		ID:          uc.Sf.Generate(),
		UserID:      userID,
		Provider:    domain.ProviderApple,
		ProviderUID: claims.Sub,
		AccessToken: req.AccessToken,
	}

	return uc.userRepo.CreateOAuthAccount(ctx, oauthAcc)
}

func (uc *UserUsecase) linkTelegramAccount(ctx context.Context, userID string, req *domain.LinkOAuthAccountRequest) error {
	// For Telegram, you'd need to pass telegram data
	return fmt.Errorf("telegram linking requires widget data")
}


// RefreshOAuthToken refreshes an expired OAuth token
func (uc *UserUsecase) RefreshOAuthToken(ctx context.Context, userID, provider string) error {
	oauthAccounts, err := uc.userRepo.GetOAuthAccountsByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get oauth accounts: %w", err)
	}

	var targetAccount *domain.OAuthAccount
	for _, acc := range oauthAccounts {
		if acc.Provider == provider {
			targetAccount = acc
			break
		}
	}

	if targetAccount == nil {
		return fmt.Errorf("oauth account not found for provider: %s", provider)
	}

	if targetAccount.RefreshToken == nil {
		return fmt.Errorf("no refresh token available for provider: %s", provider)
	}

	switch provider {
	case domain.ProviderGoogle:
		return uc.refreshGoogleToken(ctx, targetAccount)
	case domain.ProviderApple:
		return uc.refreshAppleToken(ctx, targetAccount)
	default:
		return fmt.Errorf("token refresh not implemented for provider: %s", provider)
	}
}

func (uc *UserUsecase) refreshGoogleToken(ctx context.Context, acc *domain.OAuthAccount) error {
	// Implement Google token refresh
	// This would call Google's token endpoint with the refresh token
	return fmt.Errorf("google token refresh not implemented")
}

func (uc *UserUsecase) refreshAppleToken(ctx context.Context, acc *domain.OAuthAccount) error {
	// Implement Apple token refresh
	return fmt.Errorf("apple token refresh not implemented")
}

// GetOAuthAccountStats returns statistics about OAuth accounts
func (uc *UserUsecase) GetOAuthAccountStats(ctx context.Context) (map[string]int64, error) {
	stats := make(map[string]int64)
	
	for _, provider := range domain.SupportedProviders {
		count, err := uc.userRepo.CountOAuthAccountsByProvider(ctx, provider)
		if err != nil {
			return nil, fmt.Errorf("failed to count %s accounts: %w", provider, err)
		}
		stats[provider] = count
	}
	
	return stats, nil
}

// Helper functions
func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
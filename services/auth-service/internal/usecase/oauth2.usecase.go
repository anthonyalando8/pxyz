package usecase

import (
	"auth-service/internal/domain"
	"auth-service/internal/service/apple"
	"auth-service/internal/service/oauth2"
	"context"
	"time"
)

func (uc *UserUsecase) RegisterWithGoogle(ctx context.Context, idToken, clientID string) (*domain.User, error) {
	googleUser, err := oauth2svc.VerifyGoogleToken(ctx, idToken, clientID)
	if err != nil {
		return nil, err
	}

	// Check if this Google account already exists
	oauthAcc, err := uc.userRepo.FindByProviderUID(ctx, "google", googleUser.Sub)
	if err == nil && oauthAcc != nil {
		// Existing account → fetch user
		return uc.userRepo.GetUserByID(ctx, oauthAcc.UserID)
	}

	// Otherwise → new user
	newUser := &domain.User{
		ID:           uc.Sf.Generate(),
		Email:        toPtr(googleUser.Email),
		FirstName:    toPtr(googleUser.FirstName),
		LastName:     toPtr(googleUser.LastName),
		SignupStage:  "complete",
		AccountType:  "social",
		IsEmailVerified: true,
	}

	user, err := uc.userRepo.CreateUser(ctx, newUser)
	if err != nil {
		return nil, err
	}

	// Link OAuth account
	newOAuth := &domain.OAuthAccount{
		ID:          uc.Sf.Generate(),
		UserID:      user.ID,
		Provider:    "google",
		ProviderUID: googleUser.Sub,
	}
	if err := uc.userRepo.CreateOAuthAccount(ctx, newOAuth); err != nil {
		return nil, err
	}

	return user, nil
}

type AppleDeps struct {
	ServiceID   string
	TeamID      string
	KeyID       string
	PrivateKey  string
	RedirectURI string
}

func (uc *UserUsecase) RegisterWithApple(ctx context.Context, deps AppleDeps, idToken, authCode string) (*domain.User, bool, error) {
	var tokenToVerify string

	if idToken != "" {
		tokenToVerify = idToken
	} else {
		// Exchange code -> tokens
		clientSecret, err := apple.GenerateClientSecret(apple.ClientSecretParams{
			TeamID: deps.TeamID, KeyID: deps.KeyID, ServiceID: deps.ServiceID,
			PrivateKeyPEM: deps.PrivateKey, TTL: 5 * time.Minute,
		})
		if err != nil {
			return nil, false, err
		}
		tr, err := apple.ExchangeCodeForTokens(ctx, deps.ServiceID, clientSecret, authCode, deps.RedirectURI)
		if err != nil {
			return nil, false, err
		}
		tokenToVerify = tr.IDToken
	}

	claims, err := apple.VerifyIDToken(ctx, tokenToVerify, deps.ServiceID)
	if err != nil {
		return nil, false, err
	}

	// Check if this Apple account already exists
	oauthAcc, _ := uc.userRepo.FindByProviderUID(ctx, "apple", claims.Sub)
	if oauthAcc != nil {
		u, err := uc.userRepo.GetUserByID(ctx, oauthAcc.UserID)
		return u, false, err // existing user, new=false
	}

	// Create new user
	newUser := &domain.User{
		ID:              uc.Sf.Generate(),
		Email:           emptyToNil(claims.Email),
		FirstName:       nil, // Apple only provides name once via JS; handle separately if provided by frontend
		LastName:        nil,
		IsEmailVerified: claims.EmailVerified,
		SignupStage:     "complete",
		AccountType:     "social",
	}

	u, err := uc.userRepo.CreateUser(ctx, newUser)
	if err != nil {
		return nil, false, err
	}

	// Link OAuth account
	if err := uc.userRepo.CreateOAuthAccount(ctx, &domain.OAuthAccount{
		ID:          uc.Sf.Generate(),
		UserID:      u.ID,
		Provider:    "apple",
		ProviderUID: claims.Sub,
	}); err != nil {
		return nil, false, err
	}

	return u, true, nil // new user created
}

func emptyToNil(s string) *string {
	if s == "" { return nil }
	return &s
}

func (uc *UserUsecase) HandleTelegramLogin(ctx context.Context, data map[string]string) (*domain.User, error) {
	provider := "telegram"
	telegramID := data["id"]

	// 1. Check if oauth account exists
	oauthAcc, err := uc.userRepo.FindByProviderUID(ctx, provider, telegramID)
	if err == nil && oauthAcc != nil {
		// Found linked account → fetch user
		user, err := uc.userRepo.GetUserByID(ctx, oauthAcc.UserID)
		if err != nil {
			return nil, err
		}
		return user, nil
	}

	// 2. If no linked oauth, create a new user
	newUser := &domain.User{
		ID:        uc.Sf.Generate(),
		FirstName: strOrNil(data["first_name"]),
		LastName:  strOrNil(data["last_name"]),
		// Telegram doesn’t guarantee username/email/phone
		// You can add heuristics if username exists
		AccountType: "oauth", 
		SignupStage: "complete", // since oauth users are "ready"
	}

	createdUser, err := uc.userRepo.CreateUser(ctx, newUser)
	if err != nil {
		return nil, err
	}

	// 3. Create linked oauth account
	oauthAcc = &domain.OAuthAccount{
		ID:           uc.Sf.Generate(),
		UserID:       createdUser.ID,
		Provider:     provider,
		ProviderUID:  telegramID,
		AccessToken:  nil, // Telegram doesn’t provide real tokens
		RefreshToken: nil,
	}
	if err := uc.userRepo.CreateOAuthAccount(ctx, oauthAcc); err != nil {
		return nil, err
	}

	return createdUser, nil
}

// helpers
func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

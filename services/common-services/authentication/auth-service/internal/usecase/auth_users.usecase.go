// usecase/login_usecase.go
package usecase

import (
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"x/shared/genproto/otppb"
)

// SubmitIdentifierRequest represents the first step of login
type SubmitIdentifierRequest struct {
	Identifier string `json:"identifier"` // email or phone
}

// SubmitIdentifierResponse contains the next step and cached data
type SubmitIdentifierResponse struct {
	UserID     string  `json:"user_id"`
	Next       string  `json:"next"`        // "verify_account" | "set_password" | "enter_password" | ""
	AccountType string `json:"account_type"` // "password" | "hybrid" | "social"
	IsNewUser  bool    `json:"is_new_user"`
}

// CachedUserData stores user info in cache during registration/login
type CachedUserData struct {
	UserID          string `json:"user_id"`
	Identifier      string `json:"identifier"`
	AccountType     string `json:"account_type"`
	AccountStatus   string `json:"account_status"`
	IsEmailVerified bool   `json:"is_email_verified"`
	IsPhoneVerified bool   `json:"is_phone_verified"`
	HasPassword     bool   `json:"has_password"`
	IsNewUser       bool   `json:"is_new_user"`
	CreatedAt       int64  `json:"created_at"`
}

const (
	// Cache namespaces
	CacheNamespaceUserData = "pending_auth"
	
	// Cache TTL
	CacheTTL = 15 * time.Minute
)

// SubmitIdentifier handles the first step: identifier submission
func (uc *UserUsecase) SubmitIdentifier(ctx context.Context, identifier string) (*SubmitIdentifierResponse, error) {
	if identifier == "" {
		return nil, errors.New("identifier is required")
	}
	isEmail := isEmailFormat(identifier)
	if !isEmail {
		if after, ok :=strings.CutPrefix(identifier, "+"); ok  {
			identifier = after
		}
	}

	// Try to find existing user
	userWithCred, err := uc.userRepo.GetUserByIdentifier(ctx, identifier)
	
	var response *SubmitIdentifierResponse
	
	if err != nil {
		// User not found - treat as new registration
		response, err = uc.handleNewUser(ctx, identifier)
		if err != nil {
			return nil, fmt.Errorf("failed to handle new user: %w", err)
		}
	} else {
		// Existing user - validate account type and status
		response, err = uc.handleExistingUser(ctx, userWithCred, identifier)
		if err != nil {
			return nil, fmt.Errorf("failed to handle existing user: %w", err)
		}
	}

	return response, nil
}

// handleNewUser processes new user registration flow
func (uc *UserUsecase) handleNewUser(ctx context.Context, identifier string) (*SubmitIdentifierResponse, error) {
	userID := uc.Sf.Generate()
	isEmail := isEmailFormat(identifier)
	_ = isEmail
	// if !isEmail {
	// 	if after, ok :=strings.CutPrefix(identifier, "+"); ok  {
	// 		identifier = after
	// 	}
	// }

	cachedData := &CachedUserData{
		UserID:          userID,
		Identifier:      identifier,
		AccountType:     "password",
		AccountStatus:   "pending",
		IsEmailVerified: false,
		IsPhoneVerified: false,
		HasPassword:     false,
		IsNewUser:       true,
		CreatedAt:       time.Now().Unix(),
	}

	if err := uc.cacheUserData(ctx, userID, cachedData); err != nil {
		return nil, fmt.Errorf("failed to cache user data: %w", err)
	}

	// Send OTP asynchronously
	channel, _ := uc.sendOTPAsync(ctx, userID, identifier, "initial_verification")
	_ = channel // currently unused, but could be logged or returned if needed

	nextStep := "verify_account"

	return &SubmitIdentifierResponse{
		UserID:      userID,
		Next:        nextStep,
		AccountType: "password",
		IsNewUser:   true,
	}, nil
}
// handleExistingUser processes existing user login flow
func (uc *UserUsecase) handleExistingUser(ctx context.Context, userWithCred *domain.UserWithCredential, identifier string) (*SubmitIdentifierResponse, error) {
	user := userWithCred.User
	cred := userWithCred.Credential

	// Check if account is social only
	if user.AccountType == "social" {
		return nil, errors.New("social accounts must login via OAuth provider")
	}

	// Check if account is active
	if user.AccountStatus != "active" {
		return nil, fmt.Errorf("account is %s", user.AccountStatus)
	}

	// Determine verification status
	isEmailVerified := cred.IsEmailVerified
	isPhoneVerified := cred.IsPhoneVerified
	
	// Determine which verification status to check based on identifier
	isVerified := false
	if isEmailFormat(identifier) && cred.Email != nil && *cred.Email == identifier {
		isVerified = isEmailVerified
	} else if cred.Phone != nil && *cred.Phone == identifier {
		isVerified = isPhoneVerified
	}

	// Check if password is set
	hasPassword := cred.PasswordHash != nil && *cred.PasswordHash != ""

	// Cache user data
	cachedData := &CachedUserData{
		UserID:          user.ID,
		Identifier:      identifier,
		AccountType:     user.AccountType,
		AccountStatus:   user.AccountStatus,
		IsEmailVerified: isEmailVerified,
		IsPhoneVerified: isPhoneVerified,
		HasPassword:     hasPassword,
		IsNewUser:       false,
		CreatedAt:       time.Now().Unix(),
	}

	if err := uc.cacheUserData(ctx, user.ID, cachedData); err != nil {
		return nil, fmt.Errorf("failed to cache user data: %w", err)
	}

	// Determine next step
	nextStep := ""
	
	if !isVerified {
		nextStep = "verify_account"
		channel, _ := uc.sendOTPAsync(ctx, user.ID, identifier, "initial_verification")
		_ = channel // currently unused, but could be logged or returned if needed
	} else if !hasPassword {
		nextStep = "set_password"
	} else {
		nextStep = "enter_password"
	}

	return &SubmitIdentifierResponse{
		UserID:      user.ID,
		Next:        nextStep,
		AccountType: user.AccountType,
		IsNewUser:   false,
	}, nil
}

type ResendOTPResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
    Channel string `json:"channel"`
}

func (uc *UserUsecase) ResendOTP(ctx context.Context, userID string) (*ResendOTPResponse, error) {
    // 1. Get cached user data
    cachedData, err := uc.GetCachedUserData(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get cached user data: %w", err)
    }
    if cachedData == nil {
        return nil, errors.New("session expired or no cached user data found")
    }

    // 2. Get identifier (email or phone)
    identifier := cachedData.Identifier
    if identifier == "" {
        return nil, errors.New("no identifier found for user")
    }

    // 3. Trigger OTP resend
    channel, recipient := uc.sendOTPAsync(ctx, cachedData.UserID, identifier, "resend_verification")
	_ = recipient // currently unused, but could be logged or used later

    // 4. (Optional) update timestamp in cache for reference
    // cachedData.LastOTPSentAt = time.Now().Unix()
    // _ = uc.cacheUserData(ctx, cachedData.UserID, cachedData)

    return &ResendOTPResponse{
        Success: true,
        Message: fmt.Sprintf("OTP resent to %s", identifier),
        Channel: channel,
    }, nil
}


// cacheUserData stores user data in cache
func (uc *UserUsecase) cacheUserData(ctx context.Context, userID string, data *CachedUserData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	return uc.cache.Set(ctx, CacheNamespaceUserData, userID, string(jsonData), CacheTTL)
}

// GetCachedUserData retrieves cached user data
func (uc *UserUsecase) GetCachedUserData(ctx context.Context, userID string) (*CachedUserData, error) {
	jsonData, err := uc.cache.Get(ctx, CacheNamespaceUserData, userID)
	if err != nil {
		return nil, fmt.Errorf("user data not found in cache: %w", err)
	}

	var data CachedUserData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	return &data, nil
}

// ClearCachedUserData removes user data from cache
func (uc *UserUsecase) ClearCachedUserData(ctx context.Context, userID string) error {
	return uc.cache.Delete(ctx, CacheNamespaceUserData, userID)
}

// VerifyIdentifier handles account verification step
func (uc *UserUsecase) VerifyIdentifier(ctx context.Context, userID, code string) error {
	cachedData, err := uc.GetCachedUserData(ctx, userID)
	if err != nil {
		return fmt.Errorf("session expired or invalid: %w", err)
	}
	if cachedData == nil {
		return errors.New("no cached session found")
	}

	valid, err := uc.VerifyOtpHelper(ctx, userID, code, "initial_verification")
	if err != nil {
		return fmt.Errorf("failed to verify OTP: %w", err)
	}
	if !valid {
		return errors.New("invalid or expired OTP")
	}

	isEmail := isEmailFormat(cachedData.Identifier)

	//  Step 2: Mark as verified (update cache first)
	if isEmail {
		cachedData.IsEmailVerified = true
	} else {
		cachedData.IsPhoneVerified = true
	}

	//  Step 3: Update cache immediately to reflect verification state
	if err := uc.cacheUserData(ctx, cachedData.UserID, cachedData); err != nil {
		return fmt.Errorf("failed to update cache: %w", err)
	}

	//  Step 4: If new user — create the user after verification is confirmed
	if cachedData.IsNewUser {
		if err := uc.createUserFromCache(ctx, cachedData); err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		// mark as no longer new
		cachedData.IsNewUser = false
		if err := uc.cacheUserData(ctx, cachedData.UserID, cachedData); err != nil {
			return fmt.Errorf("failed to refresh cache after user creation: %w", err)
		}
		return nil
	}

	//  Step 5: Existing user — update DB verification fields
	if isEmail {
		if err := uc.userRepo.UpdateEmailVerificationStatus(ctx, userID, true); err != nil {
			return fmt.Errorf("failed to update email verification: %w", err)
		}
	} else {
		if err := uc.userRepo.UpdatePhoneVerificationStatus(ctx, userID, true); err != nil {
			return fmt.Errorf("failed to update phone verification: %w", err)
		}
	}

	return nil
}



// createUserFromCache publishes a user creation event from cached data
func (uc *UserUsecase) createUserFromCache(ctx context.Context, cachedData *CachedUserData) error {
	isEmail := isEmailFormat(cachedData.Identifier)

	var email, phone *string
	if isEmail {
		email = &cachedData.Identifier
	} else {
		phone = &cachedData.Identifier
	}

	// Map cached data into RegisterUserRequest
	req := RegisterUserRequest{
		UserID: cachedData.UserID,
		Email:           email,
		Phone:           phone,
		Password:        nil, // no password in cached data
		Consent:         true, // safe default, since cached data doesn’t have consent
		IsEmailVerified: cachedData.IsEmailVerified,
		IsPhoneVerified: cachedData.IsPhoneVerified,
	}

	// Wrap in slice because RegisterUserAsyncBulk expects bulk
	_, err := uc.RegisterUserAsyncBulk(ctx, []RegisterUserRequest{req})
	if err != nil {
		return fmt.Errorf("failed to enqueue user registration: %w", err)
	}

	return nil
}

// SetPasswordFromCache sets password for cached user
func (uc *UserUsecase) SetPasswordFromCache(ctx context.Context, userID, password string) error {
	// Get cached data
	cachedData, err := uc.GetCachedUserData(ctx, userID)
	if err != nil {
		return fmt.Errorf("session expired or invalid: %w", err)
	}

	// Validate password
	if valid, err := utils.ValidatePassword(password); !valid {
		return err
	}

	// Hash password
	hash, err := utils.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := uc.userRepo.UpdatePassword(ctx, userID, hash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Update cache
	cachedData.HasPassword = true
	return uc.cacheUserData(ctx, userID, cachedData)
}

// GetUserByIdentifier retrieves user by email or phone (wrapper for repository)
func (uc *UserUsecase) GetUserByIdentifier(ctx context.Context, identifier string) (*domain.UserWithCredential, error) {
	return uc.userRepo.GetUserByIdentifier(ctx, identifier)
}


func (uc *UserUsecase) GetUserByID(ctx context.Context, userID string)(*domain.UserProfile, error){
	return uc.userRepo.GetUserByID(ctx, userID)
}

func (uc *UserUsecase) GetUserWithCredentialsByID(ctx context.Context, userID string) (*domain.User, []*domain.UserCredential, error) {
	return uc.userRepo.GetUserWithCredentialsByID(ctx, userID)
}
// isEmailFormat checks if identifier is email format
func isEmailFormat(identifier string) bool {
	// Simple check - in production use proper email validation
	return len(identifier) > 0 && (identifier[0] != '+' && !isNumeric(identifier))
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// sendOTPAsync handles sending OTPs in the background, auto-detecting the channel.
func (uc *UserUsecase) sendOTPAsync(ctx context.Context, userID, identifier, purpose string) (channel, recipient string) {
	isEmail := isEmailFormat(identifier)

	if isEmail {
		channel = "email"
		recipient = identifier
	} else {
		channel = "sms"
		recipient = identifier
	}

	// Send in background (don’t block user flow)
	go func() {
		_, err := uc.otp.Client.GenerateOTP(
			context.Background(), // detached from main ctx so cancellation doesn't stop OTP
			&otppb.GenerateOTPRequest{
				UserId:    userID,
				Channel:   channel,
				Purpose:   purpose,
				Recipient: recipient,
			},
		)
		if err != nil {
			// Just log — don’t bubble up, since it’s async
			log.Printf("[OTP] ❌ Failed to send OTP user=%s channel=%s err=%v", userID, channel, err)
		} else {
			log.Printf("[OTP]  OTP sent user=%s channel=%s recipient=%s", userID, channel, recipient)
		}
	}()

	return channel, recipient
}

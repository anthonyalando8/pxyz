// usecase/credential_usecase.go
package usecase

import (
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"context"
	"errors"
	"fmt"
)

// ============================================
// EMAIL MANAGEMENT
// ============================================

// ChangeEmail updates a user's email address
// Automatically resets email verification status
func (uc *UserUsecase) ChangeEmail(ctx context.Context, userID, newEmail string) error {
	if newEmail == "" {
		return errors.New("email cannot be empty")
	}

	// Optional: Validate email format
	// if !isValidEmail(newEmail) {
	//     return errors.New("invalid email format")
	// }

	// Update email in repository (automatically logs to credential_history via trigger)
	return uc.userRepo.UpdateEmail(ctx, userID, newEmail)
}

// ============================================
// PHONE MANAGEMENT
// ============================================

// UpdatePhone updates a user's phone number
func (uc *UserUsecase) UpdatePhone(ctx context.Context, userID, newPhone string, isPhoneVerified bool) error {
	if newPhone == "" {
		return errors.New("phone cannot be empty")
	}

	// Optional: Validate phone format
	// if !isValidPhone(newPhone) {
	//     return errors.New("invalid phone format")
	// }

	// Update phone in repository (automatically logs to credential_history via trigger)
	return uc.userRepo.UpdatePhone(ctx, userID, newPhone, isPhoneVerified)
}

// ============================================
// PASSWORD MANAGEMENT
// ============================================

// UpdatePassword updates a user's password with validation
func (uc *UserUsecase) UpdatePassword(ctx context.Context, userID, newPassword string, requireOld bool, oldPassword string) error {
	// Get user with credential
	_, userWithCred, err := uc.userRepo.GetUserWithCredentialsByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// If old password required (for password change vs reset)
	if requireOld {
		if userWithCred[0].PasswordHash == nil {
			return errors.New("no password set for this account")
		}
		
		if !utils.CheckPasswordHash(oldPassword, *userWithCred[0].PasswordHash) {
			return errors.New("invalid old password")
		}
	}

	// Validate new password
	if valid, err := utils.ValidatePassword(newPassword); !valid {
		return err
	}

	// Hash new password
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password (automatically logs to credential_history via trigger)
	return uc.userRepo.UpdatePassword(ctx, userID, hash)
}

// ChangePassword is a convenience method for password changes (requires old password)
func (uc *UserUsecase) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	return uc.UpdatePassword(ctx, userID, newPassword, true, oldPassword)
}

// ResetPassword is a convenience method for password resets (no old password required)
func (uc *UserUsecase) ResetPassword(ctx context.Context, userID, newPassword string) error {
	return uc.UpdatePassword(ctx, userID, newPassword, false, "")
}

// SetInitialPassword sets a password for accounts that don't have one (e.g., social accounts)
func (uc *UserUsecase) SetInitialPassword(ctx context.Context, userID, password string) error {
	// Get user with credential
	_, userWithCred, err := uc.userRepo.GetUserWithCredentialsByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Check if password already exists
	if userWithCred[0].PasswordHash != nil && *userWithCred[0].PasswordHash != "" {
		return errors.New("password already set for this account")
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
		return err
	}

	// Update account type to hybrid (social + password)
	// You may want to add a method to update account type
	// uc.userRepo.UpdateAccountType(ctx, userID, "hybrid")

	return nil
}

// ============================================
// CREDENTIAL HISTORY
// ============================================

// GetCredentialHistory retrieves the change history for a user's credentials
func (uc *UserUsecase) GetCredentialHistory(ctx context.Context, userID string, limit int) ([]*domain.CredentialHistory, error) {
	if limit <= 0 || limit > 100 {
		limit = 10 // Default limit
	}

	return uc.userRepo.GetCredentialHistory(ctx, userID, limit)
}

// ============================================
// CREDENTIAL INVALIDATION
// ============================================

// InvalidateAllCredentials marks all credentials as invalid
// Used for account deletion or security incidents
func (uc *UserUsecase) InvalidateAllCredentials(ctx context.Context, userID string) error {
	return uc.userRepo.InvalidateAllCredentials(ctx, userID)
}

// DeleteUser soft-deletes a user by invalidating credentials and marking account status
// Note: This assumes you have a method to update user status
func (uc *UserUsecase) DeleteUser(ctx context.Context, userID string) error {
	// Invalidate all credentials
	if err := uc.userRepo.InvalidateAllCredentials(ctx, userID); err != nil {
		return fmt.Errorf("failed to invalidate credentials: %w", err)
	}

	// Update user status to deleted
	// You'll need to add this method to your repository:
	// if err := uc.userRepo.UpdateAccountStatus(ctx, userID, "deleted"); err != nil {
	//     return fmt.Errorf("failed to update account status: %w", err)
	// }

	// Optionally: Revoke all OAuth accounts
	// oauthAccounts, _ := uc.userRepo.GetOAuthAccountsByUserID(ctx, userID)
	// for _, acc := range oauthAccounts {
	//     _ = uc.userRepo.UnlinkOAuthAccount(ctx, userID, acc.Provider)
	// }

	return nil
}

// ============================================
// HELPER FUNCTIONS
// ============================================

// ValidateCredentialUpdate checks if a credential update is allowed
func (uc *UserUsecase) ValidateCredentialUpdate(ctx context.Context, userID string) error {
	user, userWithCred, err := uc.userRepo.GetUserWithCredentialsByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Check if user account is active
	if user.AccountStatus != "active" {
		return errors.New("cannot update credentials for non-active account")
	}

	// Check if credential is valid
	if !userWithCred[0].Valid {
		return errors.New("credential is not valid")
	}

	return nil
}
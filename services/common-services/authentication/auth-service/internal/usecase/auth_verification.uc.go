// usecase/verification_usecase.go
package usecase

import (
	"auth-service/internal/repository"
	"context"
)

// ============================================
// SINGLE USER VERIFICATION OPERATIONS
// ============================================

// VerifyEmail marks a user's email as verified
func (uc *UserUsecase) VerifyEmail(ctx context.Context, userID string) error {
	if err := uc.userRepo.UpdateEmailVerificationStatus(ctx, userID, true); err != nil {
		return err
	}
	return nil
}

// UnverifyEmail marks a user's email as unverified
func (uc *UserUsecase) UnverifyEmail(ctx context.Context, userID string) error {
	if err := uc.userRepo.UpdateEmailVerificationStatus(ctx, userID, false); err != nil {
		return err
	}
	return nil
}

// VerifyPhone marks a user's phone as verified
func (uc *UserUsecase) VerifyPhone(ctx context.Context, userID string) error {
	if err := uc.userRepo.UpdatePhoneVerificationStatus(ctx, userID, true); err != nil {
		return err
	}
	return nil
}

// UnverifyPhone marks a user's phone as unverified
func (uc *UserUsecase) UnverifyPhone(ctx context.Context, userID string) error {
	if err := uc.userRepo.UpdatePhoneVerificationStatus(ctx, userID, false); err != nil {
		return err
	}
	return nil
}

// GetEmailVerificationStatus retrieves email verification status for a user
func (uc *UserUsecase) GetEmailVerificationStatus(ctx context.Context, userID string) (bool, error) {
	isVerified, err := uc.userRepo.GetEmailVerificationStatus(ctx, userID)
	if err != nil {
		return false, err
	}
	return isVerified, nil
}

// GetPhoneVerificationStatus retrieves phone verification status for a user
func (uc *UserUsecase) GetPhoneVerificationStatus(ctx context.Context, userID string) (bool, error) {
	isVerified, err := uc.userRepo.GetPhoneVerificationStatus(ctx, userID)
	if err != nil {
		return false, err
	}
	return isVerified, nil
}

// GetBothVerificationStatuses retrieves both email and phone verification status
func (uc *UserUsecase) GetBothVerificationStatuses(ctx context.Context, userID string) (emailVerified, phoneVerified bool, err error) {
	emailVerified, phoneVerified, err = uc.userRepo.GetBothVerificationStatuses(ctx, userID)
	if err != nil {
		return false, false, err
	}
	return emailVerified, phoneVerified, nil
}

// MarkCredentialAsVerified marks both email and phone as verified (useful for OAuth/SSO)
func (uc *UserUsecase) MarkCredentialAsVerified(ctx context.Context, userID string) error {
	return uc.userRepo.MarkCredentialAsVerified(ctx, userID)
}

// ============================================
// BATCH VERIFICATION OPERATIONS
// ============================================

// GetVerificationStatuses retrieves verification status for multiple users
func (uc *UserUsecase) GetVerificationStatuses(ctx context.Context, userIDs []string) ([]*repository.VerificationStatus, error) {
	if len(userIDs) == 0 {
		return []*repository.VerificationStatus{}, nil
	}

	statuses, err := uc.userRepo.GetVerificationStatuses(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	return statuses, nil
}

// VerifyEmailsForUsers marks email as verified for multiple users
func (uc *UserUsecase) VerifyEmailsForUsers(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	if err := uc.userRepo.UpdateEmailVerificationStatuses(ctx, userIDs, true); err != nil {
		return err
	}

	return nil
}

// UnverifyEmailsForUsers marks email as unverified for multiple users
func (uc *UserUsecase) UnverifyEmailsForUsers(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	if err := uc.userRepo.UpdateEmailVerificationStatuses(ctx, userIDs, false); err != nil {
		return err
	}

	return nil
}

// VerifyPhonesForUsers marks phone as verified for multiple users
func (uc *UserUsecase) VerifyPhonesForUsers(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	if err := uc.userRepo.UpdatePhoneVerificationStatuses(ctx, userIDs, true); err != nil {
		return err
	}

	return nil
}

// UnverifyPhonesForUsers marks phone as unverified for multiple users
func (uc *UserUsecase) UnverifyPhonesForUsers(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	if err := uc.userRepo.UpdatePhoneVerificationStatuses(ctx, userIDs, false); err != nil {
		return err
	}

	return nil
}

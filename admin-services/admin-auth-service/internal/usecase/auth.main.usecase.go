// usecase/auth_usecase.go
package usecase

import (
	"admin-auth-service/internal/domain"
	"admin-auth-service/pkg/utils"
	"context"
	"errors"
	"fmt"
)



func (uc *UserUsecase) ChangeEmail(ctx context.Context, userID, newEmail string) error {
	// Update email in repository
	return uc.userRepo.UpdateEmail(ctx, userID, newEmail)
}

func (uc *UserUsecase) UpdatePhone(ctx context.Context, userID, newPhone string, isPhoneVerified bool) error {
	// Update phone in repository
	return uc.userRepo.UpdatePhone(ctx, userID, newPhone, isPhoneVerified)
}

func (uc *UserUsecase) UpdatePassword(ctx context.Context, userID, newPassword string, requireOld bool, oldPassword string, advanceStage bool) error {
	user, err := uc.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// If old password required (for change)
	if requireOld {
		if err := utils.CheckPasswordHash(oldPassword, *user.PasswordHash); !err {
			return errors.New("invalid old password")
		}
	}

	// Validate new password
	if valid, err := utils.ValidatePassword(newPassword); !valid {
		return err
	}

	// Hash
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return uc.userRepo.UpdatePassword(ctx, userID, hash)
}


func (uc *UserUsecase) UpdateName(ctx context.Context, userID, firstName, lastName string) error {
	// Update first and last name in repository
	return uc.userRepo.UpdateName(ctx, userID, firstName, lastName)
}


func (uc *UserUsecase) FindUserById(ctx context.Context, userId string) (*domain.User, error){
	return uc.userRepo.GetUserByID(ctx, userId)
}

func (uc *UserUsecase) FindUserByIdentifier(ctx context.Context, identifier string) (*domain.User, error){
	return uc.userRepo.GetUserByIdentifier(ctx, identifier)
}

// DeleteUser deletes a user and all associated auth records.
func (uc *UserUsecase) DeleteUser(ctx context.Context, userID string) error {
	return uc.userRepo.DeleteUser(ctx, userID)
}
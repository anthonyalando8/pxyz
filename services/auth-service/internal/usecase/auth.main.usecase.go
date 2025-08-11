// usecase/auth_usecase.go
package usecase

import (
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"context"
)



func (uc *UserUsecase) ChangeEmail(ctx context.Context, userID, newEmail string) error {
	// Update email in repository
	return uc.userRepo.UpdateEmail(ctx, userID, newEmail)
}

func (uc *UserUsecase) ChangePassword(ctx context.Context, userID, newPassword string) error {
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	// Update password in repository
	return uc.userRepo.UpdatePassword(ctx, userID, hashedPassword)
}


func (uc *UserUsecase) UpdateName(ctx context.Context, userID, firstName, lastName string) error {
	// Update first and last name in repository
	return uc.userRepo.UpdateName(ctx, userID, firstName, lastName)
}


func (uc *UserUsecase) FindUserById(ctx context.Context, userId string) (*domain.User, error){
	return uc.userRepo.GetUserByID(ctx, userId)
}
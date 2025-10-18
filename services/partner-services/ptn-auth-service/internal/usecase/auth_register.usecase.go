// usecase/auth_usecase.go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ptn-auth-service/internal/domain"
	"ptn-auth-service/internal/repository"
	"ptn-auth-service/pkg/utils"
	"x/shared/utils/id"
)

type UserUsecase struct {
	userRepo *repository.UserRepository
	Sf       *id.Snowflake
}

func NewUserUsecase(userRepo *repository.UserRepository, sf *id.Snowflake) *UserUsecase {
	return &UserUsecase{
		userRepo: userRepo,
		Sf:       sf,
	}
}

func (uc *UserUsecase) RegisterUser(
	ctx context.Context,
	email, password, firstName, lastName, roleName, partnerId string,
) (*domain.User, error) {
	if email == "" || password == "" {
		return nil, errors.New("email and password required")
	}

	// hash password
	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := &domain.User{
		ID:            uc.Sf.Generate(),
		Email:         toPtr(email),
		FirstName:     toPtr(firstName),
		LastName:      toPtr(lastName),
		PasswordHash:  &hash,
		IsTempPass:    true,
		AccountType:   "password",
		AccountStatus: "active",
		Role:          roleName,
		PartnerID:     partnerId,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	createdUser, err := uc.userRepo.CreateUser(ctx, newUser)
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}


func (uc *UserUsecase) VerifyEmail(ctx context.Context, userID string) (bool, error) {
	if err := uc.userRepo.VerifyEmail(ctx, userID); err != nil {
		return false, err
	}
	return true, nil
}

func (uc *UserUsecase) VerifyPhone(ctx context.Context, userID string) (bool, error) {
	if err := uc.userRepo.VerifyPhone(ctx, userID); err != nil {
		return false, err
	}
	return true, nil
}

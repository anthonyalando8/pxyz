// usecase/auth_usecase.go
package usecase

import (
	"context"
	"errors"
	"time"

	"ptn-auth-service/internal/domain"
	"ptn-auth-service/internal/repository"
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

func (uc *UserUsecase) RegisterUser(ctx context.Context, email, password, firstName, lastName, roleName string) (*domain.User, error) {
	if email == "" || password == "" {
		return nil, errors.New("email and password required")
	}

	newUser := &domain.User{
		ID:            uc.Sf.Generate(),
		Email:         toPtr(email),
		LastName:      toPtr(lastName),
		FirstName:     toPtr(firstName),
		IsTempPass:    true,
		AccountType:   "password",
		AccountStatus: "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Save user
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

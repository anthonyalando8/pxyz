// usecase/auth_usecase.go
package usecase

import (
	"context"
	"errors"

	"auth-service/internal/domain"
	"auth-service/internal/repository"
	"auth-service/pkg/utils"
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

func (uc *UserUsecase) RegisterUser(ctx context.Context, email, phone, password, first_name, last_name string) (*domain.User, error) {
	if email == "" && phone == "" {
		return nil, errors.New("email or phone required")
	}
	if password == "" {
		return nil, errors.New("password required")
	}
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	newUser := &domain.User{
		ID:        uc.Sf.Generate(), // TODO: Update to snowlflakes formart
		Email:      toPtr(email),
		Phone:      toPtr(phone),
		PasswordHash:   toPtr(hashedPassword),
		HasPassword: true,
		LastName:   toPtr(last_name),
		FirstName:  toPtr(first_name),
		IsVerified: false,
	}

	// Save to database
	if err := uc.userRepo.CreateUser(ctx, newUser); err != nil {
		return nil, err
	}

	return newUser, nil
}

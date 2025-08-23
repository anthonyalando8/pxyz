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

func (uc *UserUsecase) RegisterUser(ctx context.Context, email, password, first_name, last_name string) (*domain.User, error) {
	if email == ""{
		return nil, errors.New("email required")
	}
	if password == "" {
		return nil, errors.New("password required")
	}
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	newUser := &domain.User{
		ID:        uc.Sf.Generate(),
		Email:      toPtr(email),
		PasswordHash:   toPtr(hashedPassword),
		LastName:   toPtr(last_name),
		FirstName:  toPtr(first_name),
	}

	// Save to database

	return uc.userRepo.CreateUser(ctx, newUser)
}

func (uc *UserUsecase) CreatePartialUser(ctx context.Context, email string) (*domain.User, error){
	newUser := &domain.User{
		ID:        uc.Sf.Generate(),
		Email:      toPtr(email),
	}

	// Save to database
	return uc.userRepo.CreateUser(ctx, newUser)
}

func (uc *UserUsecase) VerifyEmail(ctx context.Context, userID string) (bool, error){
	if err := uc.userRepo.VerifyEmail(ctx, userID); err != nil{
		return false, err
	}
	return true, nil
}

func (uc *UserUsecase) VerifyPhone(ctx context.Context, userID string) (bool, error){
	if err := uc.userRepo.VerifyPhone(ctx, userID); err != nil{
		return false, err
	}
	return true, nil
}
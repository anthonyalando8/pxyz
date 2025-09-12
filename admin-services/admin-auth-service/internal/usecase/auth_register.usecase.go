// usecase/auth_usecase.go
package usecase

import (
	"context"
	"errors"
	"time"

	"admin-auth-service/internal/domain"
	"admin-auth-service/internal/repository"
	"admin-auth-service/pkg/utils"
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

	hashedPassword, err := utils.HashPassword(password)

	if err != nil {
		return nil, err
	}
	newUser := &domain.User{
		ID:        uc.Sf.Generate(),
		Email:     toPtr(email),
		LastName:  toPtr(lastName),
		FirstName: toPtr(firstName),
		IsTempPass: true,
		PasswordHash: &hashedPassword,
		AccountType: "password",
		AccountStatus: "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save user
	createdUser, err := uc.userRepo.CreateUser(ctx, newUser)
	if err != nil {
		return nil, err
	}


	return createdUser, nil
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


// usecase/auth_usecase.go
package usecase

import (
	"context"
	"errors"

	"auth-service/internal/domain"
	"auth-service/internal/repository"
	"auth-service/pkg/utils"
)

func (uc *UserUsecase) LoginUser(ctx context.Context, identifier, password string) (*domain.User, error) {
	if identifier == "" || password == "" {
		return nil, errors.New("identifier and password required")
	}

	user, err := uc.userRepo.GetUserByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}
	if !user.HasPassword{
		return nil, errors.New("identifier linked to a social account")
	}
	if !utils.CheckPasswordHash(password, *user.PasswordHash) { // TODO: Compare hashed passwords
		return nil, errors.New("invalid password")
	}
	return user, nil
}

func (uc *UserUsecase) UserExists(ctx context.Context, identifier string) (bool, error) {
	if identifier == "" {
		return false, errors.New("identifier required")
	}
	_, err := uc.userRepo.GetUserByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return false, nil // User does not exist
		}
		return false, err // Other error
	}
	return true, nil // User exists
}
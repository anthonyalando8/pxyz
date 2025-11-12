// usecase/auth_usecase.go
package usecase

import (
	"context"
	"errors"

	"admin-auth-service/internal/domain"
	"admin-auth-service/pkg/utils"

	"x/shared/utils/errors"
)

func (uc *UserUsecase) LoginUser(ctx context.Context, identifier, password string) (*domain.User, error) {
	if identifier == "" || password == "" {
		return nil, errors.New("identifier and password required")
	}
	user, err := uc.userRepo.GetUserByIdentifier(ctx, identifier)
	if err != nil {
		return nil, err
	}

	if !utils.CheckPasswordHash(password, *user.PasswordHash) {
		return nil, xerrors.ErrInvalidPassword
	}

	return user, nil
}

func (uc *UserUsecase) UserExists(ctx context.Context, identifier string) (bool, error) {
	if identifier == "" {
		return false, errors.New("identifier required")
	}
	_, err := uc.userRepo.GetUserByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, xerrors.ErrUserNotFound) {
			return false, nil // User does not exist
		}
		return false, err // Other error
	}
	return true, nil // User exists
}
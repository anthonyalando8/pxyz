package usecase

import (
	"context"
	"errors"

	"admin-auth-service/internal/domain"
	"x/shared/utils/errors"
)

func (uc *UserUsecase) GetProfile(ctx context.Context, userID string) (*domain.User, error) {
	if userID == "" {
		return nil, errors.New("user ID required")
	}
	user, err := uc.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, xerrors.ErrUserNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return user, nil
}

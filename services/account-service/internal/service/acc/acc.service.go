package service

import (
	"context"
	"errors"
	"time"

	"account-service/internal/domain"
	"account-service/internal/repository"

	"github.com/jackc/pgx/v5"
)

type AccountService struct {
	repo *repository.UserProfileRepository
}

func NewAccountService(repo *repository.UserProfileRepository) *AccountService {
	return &AccountService{repo: repo}
}

// GetOrCreateProfile ensures a profile exists for the given user
func (uc *AccountService) GetOrCreateProfile(ctx context.Context, userID string, firstName, lastName string) (*domain.UserProfile, error) {
	// Try fetch
	profile, err := uc.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// First time user
			sysUsername := GenerateSysUsername(firstName, lastName)

			newProfile := &domain.UserProfile{
				UserID:      userID,
				FirstName:   firstName,
				LastName:    lastName,
				SysUsername: sysUsername,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				FirstTime:   true,
			}

			if err := uc.repo.Create(ctx, newProfile); err != nil {
				return nil, err
			}
			return newProfile, nil
		}
		return nil, err
	}

	// Existing user
	profile.FirstTime = false
	return profile, nil
}

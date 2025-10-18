package accservice

import (
	"context"
	"errors"
	"time"

	"account-service/internal/domain"
	"account-service/internal/repository"

	"github.com/jackc/pgx/v5"

	"x/shared/utils/id"

)

type AccountService struct {
	repo *repository.UserProfileRepository
	sf          *id.Snowflake
}

func NewAccountService(repo *repository.UserProfileRepository, sf *id.Snowflake) *AccountService {
	return &AccountService{repo: repo, sf: sf,}
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


func (uc *AccountService) UpdateProfile(ctx context.Context, profile *domain.UserProfile) error {
	// Update logic here (not implemented in this snippet)
	return uc.repo.Update(ctx, profile)
}

func (uc *AccountService) UpdateProfileImage(ctx context.Context, userID, imageURL string) error {
	// Fetch existing profile
	// profile, err := uc.repo.GetByUserID(ctx, userID)
	// if err != nil {
	// 	return err
	// }

	// Update profile image URL
	// profile.ProfileImageURL = imageURL
	// profile.UpdatedAt = time.Now()

	// Save changes
	return uc.repo.UpdateProfilePicture(ctx, userID, imageURL)
}

func (uc *AccountService) UpdateNationality(ctx context.Context, userID string, nationality *string) error {
	// Fetch existing profile
	_, err := uc.repo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Save changes
	return uc.repo.UpdateNationality(ctx,userID, nationality)
}


func (uc *AccountService) RemoveProfileImage(ctx context.Context, userID string) error {
	// Fetch existing profile
	profile, err := uc.repo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Remove profile image URL
	profile.ProfileImageURL = ""
	profile.UpdatedAt = time.Now()

	// Save changes
	return uc.repo.Update(ctx, profile)
}
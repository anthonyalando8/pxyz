package prefservice

import (
	"context"
	"fmt"
	"time"

	"account-service/internal/domain"
	"account-service/internal/repository"

	"x/shared/utils/errors"
	"x/shared/utils/id"

	"google.golang.org/protobuf/types/known/structpb"
)

type PreferencesService struct {
	repo *repository.PreferencesRepository
	sf   *id.Snowflake
}

// NewPreferencesService creates a new preferences service instance
func NewPreferencesService(repo *repository.PreferencesRepository, sf *id.Snowflake) *PreferencesService {
	return &PreferencesService{repo: repo, sf: sf}
}

// GetOrCreatePrefs fetches preferences for a user, or creates empty ones if not found
func (s *PreferencesService) GetOrCreatePrefs(ctx context.Context, userID string) (*domain.UserPreferences, error) {
	prefs, err := s.repo.GetPreferences(ctx, userID)
	if err != nil {
		if err == xerrors.ErrNotFound {
			// Only create defaults if row doesn't exist
			prefs = &domain.UserPreferences{
				UserID:      userID,
				Preferences: domain.DefaultPreferences(),
				UpdatedAt:   time.Now(),
			}
			if err := s.repo.UpsertPreferences(ctx, prefs); err != nil {
				return nil, fmt.Errorf("failed to create default preferences: %w", err)
			}
			return prefs, nil
		}
		return nil, err
	}
	return prefs, nil
}

// UpdatePreferences updates or adds only the keys provided in prefs.Preferences
func (s *PreferencesService) UpdatePreferences(ctx context.Context, prefs *domain.UserPreferences) error {
	// Ensure user row exists
	if _, err := s.GetOrCreatePrefs(ctx, prefs.UserID); err != nil {
		return err
	}

	// Only update the keys provided in the request
	return s.repo.UpdateMultiplePreferences(ctx, prefs.UserID, prefs.Preferences)
}

// UpdateSinglePreference updates/adds a single key
func (s *PreferencesService) UpdateSinglePreference(ctx context.Context, userID, key string, value *structpb.Value) error {
	err := s.repo.UpdateSinglePreference(ctx, userID, key, value)
	if err != nil {
		if err == xerrors.ErrNotFound {
			// Row doesn't exist, create with the single key
			newPrefs := &domain.UserPreferences{
				UserID:      userID,
				Preferences: map[string]*structpb.Value{key: value},
				UpdatedAt:   time.Now(),
			}
			return s.repo.UpsertPreferences(ctx, newPrefs)
		}
		return err
	}
	return nil
}

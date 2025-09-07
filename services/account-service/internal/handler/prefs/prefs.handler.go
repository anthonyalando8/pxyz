package prefhandler

import (
	"context"
	"fmt"

	"account-service/internal/domain"
	"account-service/internal/service/prefs"
	accountpb "x/shared/genproto/accountpb"

	structpb "google.golang.org/protobuf/types/known/structpb"
)

type PreferencesHandler struct {
	prefService *prefservice.PreferencesService
}

func NewPreferencesHandler(svc *prefservice.PreferencesService) *PreferencesHandler {
	return &PreferencesHandler{prefService: svc}
}

// GetPreferences handles fetching user preferences
func (h *PreferencesHandler) GetPreferences(ctx context.Context, req *accountpb.GetPreferencesRequest) (*accountpb.GetPreferencesResponse, error) {
	prefs, err := h.prefService.GetOrCreatePrefs(ctx, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get preferences: %w", err)
	}

	// If keys are provided, filter
	result := make(map[string]*structpb.Value)
	if len(req.Keys) > 0 {
		for _, key := range req.Keys {
			if val, ok := prefs.Preferences[key]; ok {
				result[key] = val
			}
		}
	} else {
		result = prefs.Preferences
	}

	return &accountpb.GetPreferencesResponse{
		Preferences: result,
	}, nil
}

// UpdatePreferences handles updating multiple user preferences
func (h *PreferencesHandler) UpdatePreferences(ctx context.Context, req *accountpb.UpdatePreferencesRequest) (*accountpb.UpdatePreferencesResponse, error) {
	if len(req.Preferences) == 0 {
		return &accountpb.UpdatePreferencesResponse{
			Success: false,
			Error:   "no preferences provided",
		}, nil
	}

	// Build domain object
	prefs := &domain.UserPreferences{
		UserID:      req.UserId,
		Preferences: req.Preferences,
	}

	if err := h.prefService.UpdatePreferences(ctx, prefs); err != nil {
		return &accountpb.UpdatePreferencesResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to update preferences: %v", err),
		}, nil
	}

	return &accountpb.UpdatePreferencesResponse{
		Success: true,
	}, nil
}

// internal/handler/websocket/p2p_websocket_handler.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"p2p-service/internal/domain"
	
)

// ============================================================================
// PROFILE HANDLERS
// ============================================================================

func (h *P2PWebSocketHandler) handleGetProfile(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		ProfileID int64  `json:"profile_id,omitempty"`
		UserID    string `json:"user_id,omitempty"`
		Username  string `json:"username,omitempty"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("Invalid request format")
		return
	}

	var profile *domain.P2PProfile
	var err error

	// Determine which lookup method to use
	if req.ProfileID > 0 {
		profile, err = h.profileUsecase.GetProfile(ctx, req.ProfileID)
	} else if req.UserID != "" {
		profile, err = h.profileUsecase.GetProfileByUserID(ctx, req.UserID)
	} else if req.Username != "" {
		profile, err = h.profileUsecase.GetProfileByUsername(ctx, req.Username)
	} else {
		// Get own profile
		profile, err = h.profileUsecase.GetProfile(ctx, client.ProfileID)
	}

	if err != nil {
		client.SendError(fmt.Sprintf("Failed to get profile: %v", err))
		return
	}

	client.SendJSON(&WSResponse{
		Type:    "profile.get",
		Success: true,
		Data:    profile,
	})
}

func (h *P2PWebSocketHandler) handleUpdateProfile(ctx context.Context, client *Client, data json.RawMessage) {
	var req domain.UpdateProfileRequest
	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("Invalid request format")
		return
	}

	profile, err := h.profileUsecase.UpdateProfile(ctx, client.ProfileID, &req)
	if err != nil {
		client.SendError(fmt.Sprintf("Failed to update profile: %v", err))
		return
	}

	client.SendJSON(&WSResponse{
		Type:    "profile.update",
		Success: true,
		Data:    profile,
		Message: "Profile updated successfully",
	})
}

func (h *P2PWebSocketHandler) handleGetProfileStats(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		ProfileID int64 `json:"profile_id,omitempty"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("Invalid request format")
		return
	}

	profileID := req.ProfileID
	if profileID == 0 {
		profileID = client.ProfileID
	}

	stats, err := h.profileUsecase.GetProfileStats(ctx, profileID)
	if err != nil {
		client.SendError(fmt.Sprintf("Failed to get stats: %v", err))
		return
	}

	client.SendJSON(&WSResponse{
		Type:    "profile.stats",
		Success: true,
		Data:    stats,
	})
}

func (h *P2PWebSocketHandler) handleSearchProfiles(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		Search      string  `json:"search,omitempty"`
		IsVerified  *bool   `json:"is_verified,omitempty"`
		IsMerchant  *bool   `json:"is_merchant,omitempty"`
		MinRating   *float64 `json:"min_rating,omitempty"`
		Limit       int     `json:"limit,omitempty"`
		Offset      int     `json:"offset,omitempty"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("Invalid request format")
		return
	}

	filter := &domain.ProfileFilter{
		Search:     req.Search,
		IsVerified: req.IsVerified,
		IsMerchant: req.IsMerchant,
		MinRating:  req.MinRating,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	profiles, total, err := h.profileUsecase.ListProfiles(ctx, filter)
	if err != nil {
		client.SendError(fmt.Sprintf("Failed to search profiles: %v", err))
		return
	}

	client.SendJSON(&WSResponse{
		Type:    "profile.search",
		Success: true,
		Data: map[string]interface{}{
			"profiles": profiles,
			"total":    total,
		},
	})
}
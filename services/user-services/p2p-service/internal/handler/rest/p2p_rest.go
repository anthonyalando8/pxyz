// internal/handler/p2p_rest_handler.go
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"p2p-service/internal/domain"
	"p2p-service/internal/usecase"
	"x/shared/auth/middleware"

	"go.uber.org/zap"
)

type P2PRestHandler struct {
	profileUsecase *usecase.P2PProfileUsecase
	logger         *zap.Logger
}

func NewP2PRestHandler(
	profileUsecase *usecase.P2PProfileUsecase,
	logger *zap.Logger,
) *P2PRestHandler {
	return &P2PRestHandler{
		profileUsecase: profileUsecase,
		logger:         logger,
	}
}

// ============================================================================
// PROFILE ENDPOINTS
// ============================================================================

// CheckProfile checks if user has a P2P profile
// GET /api/p2p/profile/check
func (h *P2PRestHandler) CheckProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user ID from context (set by auth middleware)
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	h.logger.Info("Checking P2P profile",
		zap.String("user_id", userID))

	// Check if profile exists
	profile, err := h.profileUsecase.GetProfileByUserID(ctx, userID)
	
	if err != nil {
		// Profile doesn't exist
		h.respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"exists":      false,
				"has_consent": false,
				"can_connect": false,
				"message":     "You have not joined P2P trading yet.",
			},
		})
		return
	}

	// Profile exists
	canConnect := profile.HasConsent && !profile.IsSuspended

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"exists":        true,
			"profile_id":    profile.ID,
			"has_consent":   profile.HasConsent,
			"is_suspended":  profile.IsSuspended,
			"can_connect":   canConnect,
			"profile":       profile,
		},
	})
}

// GetProfile retrieves user's P2P profile
// GET /api/p2p/profile
func (h *P2PRestHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user ID from context
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	h.logger.Info("Getting P2P profile",
		zap.String("user_id", userID))

	profile, err := h.profileUsecase.GetProfileByUserID(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get profile",
			zap.String("user_id", userID),
			zap.Error(err))
		h.respondError(w, http.StatusNotFound, "Profile not found. Please create a P2P profile first.", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"profile": profile,
		},
	})
}

// CreateProfile creates a new P2P profile
// POST /api/p2p/profile/create
func (h *P2PRestHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user ID from context
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Parse request body
	var req struct {
		Username                string   `json:"username,omitempty"`
		PhoneNumber             string   `json:"phone_number,omitempty"`
		Email                   string   `json:"email,omitempty"`
		ProfilePictureURL       string   `json:"profile_picture_url,omitempty"`
		PreferredCurrency       string   `json:"preferred_currency,omitempty"`
		PreferredPaymentMethods []int    `json:"preferred_payment_methods,omitempty"`
		AutoReplyMessage        string   `json:"auto_reply_message,omitempty"`
		HasConsent              bool     `json:"has_consent"` //  Required consent
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	h.logger.Info("Creating P2P profile",
		zap.String("user_id", userID),
		zap.Bool("has_consent", req.HasConsent))

	//  Validate consent
	if !req.HasConsent {
		h.respondError(w, http.StatusBadRequest, "You must accept the terms and conditions to create a P2P profile", nil)
		return
	}

	// Check if profile already exists
	existingProfile, err := h.profileUsecase.GetProfileByUserID(ctx, userID)
	if err == nil && existingProfile != nil {
		h.logger.Warn("Profile already exists",
			zap.String("user_id", userID),
			zap.Int64("profile_id", existingProfile.ID))
		h.respondError(w, http.StatusConflict, "You already have a P2P profile", nil)
		return
	}

	// Create profile
	createReq := &domain.CreateProfileRequest{
		UserID:                  userID,
		Username:                req.Username,
		PhoneNumber:             req.PhoneNumber,
		Email:                   req.Email,
		ProfilePictureURL:       req.ProfilePictureURL,
		PreferredCurrency:       req.PreferredCurrency,
		PreferredPaymentMethods: req.PreferredPaymentMethods,
		AutoReplyMessage:        req.AutoReplyMessage,
		HasConsent:              req.HasConsent, //  Store consent
		ConsentedAt:             timePtr(time.Now()),
	}

	profile, err := h.profileUsecase.CreateProfile(ctx, createReq)
	if err != nil {
		h.logger.Error("Failed to create profile",
			zap.String("user_id", userID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to create profile", err)
		return
	}

	h.logger.Info("P2P profile created successfully",
		zap.Int64("profile_id", profile.ID),
		zap.String("user_id", userID))

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"profile": profile,
		},
		"message": "P2P profile created successfully. You can now connect to the P2P trading platform.",
	})
}

// UpdateProfile updates user's P2P profile
// PUT /api/p2p/profile
func (h *P2PRestHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user ID from context
	userID, ok := ctx.Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Get profile
	profile, err := h.profileUsecase.GetProfileByUserID(ctx, userID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "Profile not found", err)
		return
	}

	// Parse request body
	var req domain.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	h.logger.Info("Updating P2P profile",
		zap.String("user_id", userID),
		zap.Int64("profile_id", profile.ID))

	// Update profile
	updatedProfile, err := h.profileUsecase.UpdateProfile(ctx, profile.ID, &req)
	if err != nil {
		h.logger.Error("Failed to update profile",
			zap.String("user_id", userID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to update profile", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"profile": updatedProfile,
		},
		"message": "Profile updated successfully",
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func (h *P2PRestHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *P2PRestHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Error(message,
		zap.Error(err),
		zap.Int("status", status))

	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}

	if err != nil {
		response["details"] = err.Error()
	}

	h.respondJSON(w, status, response)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
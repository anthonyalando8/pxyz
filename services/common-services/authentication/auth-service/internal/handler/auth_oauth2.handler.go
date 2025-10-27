package handler

import (
	//"auth-service/internal/domain"
	"auth-service/internal/usecase"
	"context"
	"encoding/json"
	"log"
	"net/http"
	
	"x/shared/auth/middleware"
	accountclient "x/shared/genproto/accountpb"
	"x/shared/response"
)

// ================================
// GOOGLE AUTH
// ================================

type GoogleAuthRequest struct {
	IDToken        string  `json:"id_token"`
	OAuth2Context *OAuth2Context `json:"oauth2_context,omitempty"`

	DeviceID       *string `json:"device_id"`
	GeoLocation    *string `json:"geo_location"`
	DeviceMetadata *any    `json:"device_metadata"`
}

func (h *AuthHandler) GoogleAuthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract current role from context
	currentRoleVal := ctx.Value(middleware.ContextRole)
	currentRole, ok := currentRoleVal.(string)
	if !ok || currentRole == "" || currentRole == "temp" {
		currentRole = "any"
	}

	// Parse request
	var req GoogleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.IDToken == "" {
		response.Error(w, http.StatusBadRequest, "id_token is required")
		return
	}

	// Register or fetch user using Google token
	userWithCred, googleUser, err := h.uc.RegisterWithGoogle(ctx, req.IDToken, h.config.GoogleClientID)
	if err != nil {
		log.Printf("[GoogleAuth] Registration failed: %v", err)
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Get user ID from the User struct
	userID := userWithCred.User.ID

	// Attempt role upgrade if current role is "any"
	h.postAccountCreationTask(userID, currentRole) // refresh profile cache in background
	// Check if this is an OAuth2 flow
	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != "" {
		// For OAuth2 flow, redirect to consent screen after setting password
		h.handleOAuth2PostAuth(w, r, userID, req.OAuth2Context)
		return
	}
	// Ensure nationality
	next, _ := /*h.ensureNationality(ctx, userID)*/ "",""

	// Send welcome notification in background
	go h.sendWelcomeNotification(userID)

	// Determine session type and force completion flag
	sessionType := "general"
	forceComplete := false
	if next != "" {
		sessionType = "incomplete_profile"
		forceComplete = true
	}

	// Create session
	session, err := h.createSessionHelper(
		ctx,
		userID,
		forceComplete,
		false, // isSudo
		sessionType,
		nil,
		req.DeviceID,
		req.DeviceMetadata,
		req.GeoLocation,
		r,
	)
	if err != nil {
		log.Printf("[GoogleAuth] Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
		return
	}

	// Build response
	resp := map[string]interface{}{
		"token":  session.AuthToken,
		"device": session.DeviceID,
	}
	if next != "" {
		resp["next"] = next
	}

	response.JSON(w, http.StatusOK, resp)

	// Background profile update
	h.BackgroundUpdateProfile(
		userID,
		toPtr(googleUser.FirstName),
		toPtr(googleUser.LastName),
		nil,
		nil,
	)
}

// ================================
// APPLE AUTH
// ================================

type AppleAuthRequest struct {
	IDToken   *string `json:"id_token,omitempty"`
	Code      *string `json:"code,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`

	OAuth2Context *OAuth2Context `json:"oauth2_context,omitempty"`

	DeviceID       *string `json:"device_id"`
	GeoLocation    *string `json:"geo_location"`
	DeviceMetadata *any    `json:"device_metadata"`
}

func (h *AuthHandler) AppleAuthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract current role from context
	currentRoleVal := ctx.Value(middleware.ContextRole)
	currentRole, ok := currentRoleVal.(string)
	if !ok || currentRole == "" || currentRole == "temp" {
		currentRole = "any"
	}

	var req AppleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var idToken, authCode string
	if req.IDToken != nil {
		idToken = *req.IDToken
	}
	if req.Code != nil {
		authCode = *req.Code
	}

	if idToken == "" && authCode == "" {
		response.Error(w, http.StatusBadRequest, "either id_token or code is required")
		return
	}

	// Build Apple deps
	deps := usecase.AppleDeps{
		ServiceID:   h.config.Apple.ServiceID,
		TeamID:      h.config.Apple.TeamID,
		KeyID:       h.config.Apple.KeyID,
		PrivateKey:  h.config.Apple.PrivateKeyPEM,
		RedirectURI: h.config.Apple.RedirectURI,
	}

	userWithCred, isNew, err := h.uc.RegisterWithApple(ctx, deps, idToken, authCode)
	if err != nil {
		log.Printf("[AppleAuth] Registration failed: %v", err)
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	_ = isNew // currently not used

	userID := userWithCred.User.ID

	// Attempt role upgrade
	h.postAccountCreationTask(userID, currentRole) // refresh profile cache in background
	// Check if this is an OAuth2 flow
	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != "" {
		// For OAuth2 flow, redirect to consent screen after setting password
		h.handleOAuth2PostAuth(w, r, userID, req.OAuth2Context)
		return
	}

	// Persist first/last name if Apple sent them on first sign-in
	// if isNew && (req.FirstName != nil || req.LastName != nil) {
	// 	firstName := ""
	// 	lastName := ""
	// 	if req.FirstName != nil {
	// 		firstName = *req.FirstName
	// 	}
	// 	if req.LastName != nil {
	// 		lastName = *req.LastName
	// 	}
	// 	_ = h.uc.UpdateName(ctx, userID, firstName, lastName)
	// }

	// Ensure nationality
	next, _ := /*h.ensureNationality(ctx, userID)*/ "",""

	go h.sendWelcomeNotification(userID)

	// Determine session type
	sessionType := "general"
	forceComplete := false
	if next != "" {
		sessionType = "incomplete_profile"
		forceComplete = true
	}

	// Create session
	session, err := h.createSessionHelper(
		ctx,
		userID,
		forceComplete,
		false,
		sessionType,
		nil,
		req.DeviceID,
		req.DeviceMetadata,
		req.GeoLocation,
		r,
	)
	if err != nil {
		log.Printf("[AppleAuth] Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
		return
	}

	// Build response
	resp := map[string]interface{}{
		"token":  session.AuthToken,
		"device": session.DeviceID,
	}
	if next != "" {
		resp["next"] = next
	}

	response.JSON(w, http.StatusOK, resp)

	// Background profile update
	h.BackgroundUpdateProfile(
		userID,
		req.FirstName,
		req.LastName,
		nil,
		nil,
	)
}

// ================================
// TELEGRAM AUTH
// ================================

type TelegramLoginRequest struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  string `json:"auth_date"`
	Hash      string `json:"hash"`

	OAuth2Context *OAuth2Context `json:"oauth2_context,omitempty"`

	DeviceID       *string `json:"device_id"`
	GeoLocation    *string `json:"geo_location"`
	DeviceMetadata *any    `json:"device_metadata"`
}

func (h *AuthHandler) TelegramLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract current role from context
	currentRoleVal := ctx.Value(middleware.ContextRole)
	currentRole, ok := currentRoleVal.(string)
	if !ok || currentRole == "" || currentRole == "temp" {
		currentRole = "any"
	}

	var req TelegramLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[TelegramLogin] Invalid request body: %v", err)
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	log.Printf("[TelegramLogin] Received login request for Telegram ID: %s", req.ID)

	// Map for verification
	data := map[string]string{
		"id":         req.ID,
		"first_name": req.FirstName,
		"last_name":  req.LastName,
		"username":   req.Username,
		"photo_url":  req.PhotoURL,
		"auth_date":  req.AuthDate,
		"hash":       req.Hash,
	}

	// Verify Telegram authentication
	if !h.telegramClient.VerifyTelegramAuth(data) {
		log.Printf("[TelegramLogin] Telegram auth verification failed for ID: %s", req.ID)
		response.Error(w, http.StatusUnauthorized, "Telegram verification failed")
		return
	}
	log.Printf("[TelegramLogin] Telegram auth verified successfully for ID: %s", req.ID)

	// Handle user creation/linking
	userWithCred, err := h.uc.HandleTelegramLogin(ctx, data)
	if err != nil {
		log.Printf("[TelegramLogin] Failed to handle Telegram login for ID %s: %v", req.ID, err)
		response.Error(w, http.StatusInternalServerError, "failed to process login")
		return
	}

	userID := userWithCred.User.ID
	log.Printf("[TelegramLogin] User processed successfully: userID=%s", userID)

	// Attempt role upgrade
	h.postAccountCreationTask(userID, currentRole) // refresh profile cache in background

	// Check if this is an OAuth2 flow
	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != "" {
		// For OAuth2 flow, redirect to consent screen after setting password
		h.handleOAuth2PostAuth(w, r, userID, req.OAuth2Context)
		return
	}
	// Ensure nationality
	next, _ := /*h.ensureNationality(ctx, userID)*/ "",""
	if next != "" {
		log.Printf("[TelegramLogin] User %s requires next step: %s", userID, next)
	}

	go h.sendWelcomeNotification(userID)

	// Determine session type
	sessionType := "general"
	forceComplete := false
	if next != "" {
		sessionType = "incomplete_profile"
		forceComplete = true
	}

	// Create session
	session, err := h.createSessionHelper(
		ctx,
		userID,
		forceComplete,
		false,
		sessionType,
		nil,
		req.DeviceID,
		req.DeviceMetadata,
		req.GeoLocation,
		r,
	)
	if err != nil {
		log.Printf("[TelegramLogin] Failed to create session for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
		return
	}
	log.Printf("[TelegramLogin] Session created successfully for user %s, device %v", userID, req.DeviceID)

	// Build response
	resp := map[string]interface{}{
		"token":  session.AuthToken,
		"device": session.DeviceID,
	}
	if next != "" {
		resp["next"] = next
	}

	log.Printf("[TelegramLogin] Sending response for user %s", userID)
	response.JSON(w, http.StatusOK, resp)

	// Background profile update
	h.BackgroundUpdateProfile(
		userID,
		toPtr(req.FirstName),
		toPtr(req.LastName),
		toPtr(req.Username),
		toPtr(req.PhotoURL),
	)
}

// ================================
// HELPER FUNCTIONS
// ================================

// BackgroundUpdateProfile updates the profile and profile picture in the background
func (h *AuthHandler) BackgroundUpdateProfile(userID string, firstName, lastName, username, photoURL *string) {
	go func() {
		// 1. Update main profile fields
		updateReq := &UpdateProfileRequest{
			FirstName:   firstName,
			LastName:    lastName,
			SysUsername: username,
		}

		protoReq, err := BuildUpdateProfileRequest(userID, updateReq)
		if err != nil {
			log.Printf("[BG][UpdateProfile] Failed to build gRPC request for user %s: %v", userID, err)
			return
		}

		if _, err := h.accountClient.Client.UpdateProfile(context.Background(), protoReq); err != nil {
			log.Printf("[BG][UpdateProfile] Failed to update profile for user %s: %v", userID, err)
			return
		}
		log.Printf("[BG][UpdateProfile] Profile updated successfully for user %s", userID)

		// 2. Update profile picture if provided
		if photoURL != nil && *photoURL != "" {
			if _, err := h.accountClient.Client.UpdateProfilePicture(
				context.Background(),
				&accountclient.UpdateProfilePictureRequest{
					UserId:   userID,
					ImageUrl: *photoURL,
				},
			); err != nil {
				log.Printf("[BG][UpdateProfilePicture] Failed to update profile picture for user %s: %v", userID, err)
				return
			}
			log.Printf("[BG][UpdateProfilePicture] Profile picture updated successfully for user %s", userID)
		}
	}()
}

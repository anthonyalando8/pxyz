package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"x/shared/response"
	"auth-service/internal/usecase"
)

type GoogleAuthRequest struct {
	IDToken  string `json:"id_token"`  // Google ID token from frontend
	
	DeviceID       *string     `json:"device_id"`
	GeoLocation    *string     `json:"geo_location"`
	DeviceMetadata *any        `json:"device_metadata"`
}

func (h *AuthHandler) GoogleAuthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req GoogleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w,  http.StatusBadRequest, "invalid request",)
		return
	}

	if req.IDToken == "" {
		response.Error(w, http.StatusBadRequest, "id_token is required")
		return
	}

	user, err := h.uc.RegisterWithGoogle(ctx, req.IDToken, h.config.GoogleClientID)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	session, err := h.createSessionHelper(
		r.Context(),
		user.ID, false, false, "general",
		nil, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
	)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"token":  session.AuthToken,
		"device": session.DeviceID,
	})
}

type AppleAuthRequest struct {
	IDToken string `json:"id_token,omitempty"` // if using Apple JS directly
	Code    string `json:"code,omitempty"`     // if using OAuth redirect/code flow
	// Optionally capture first/last name if your frontend receives them on first sign-in
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`

	DeviceID       *string     `json:"device_id"`
	GeoLocation    *string     `json:"geo_location"`
	DeviceMetadata *any        `json:"device_metadata"`
}

func (h *AuthHandler) AppleAuthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req AppleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.IDToken == "" && req.Code == "" {
		http.Error(w, "either id_token or code is required", http.StatusBadRequest)
		return
	}

	// Build deps from your config/env
	deps := usecase.AppleDeps{
		ServiceID:   h.config.Apple.ServiceID,
		TeamID:      h.config.Apple.TeamID,
		KeyID:       h.config.Apple.KeyID,
		PrivateKey:  h.config.Apple.PrivateKeyPEM,
		RedirectURI: h.config.Apple.RedirectURI,
	}

	user, isNew, err := h.uc.RegisterWithApple(ctx, deps, req.IDToken, req.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// If Apple sent name only once (first time) via your frontend, you can persist here
	if isNew && (req.FirstName != nil || req.LastName != nil) {
		firstName := ""
		lastName := ""
		if req.FirstName != nil {
			firstName = *req.FirstName
		}
		if req.LastName != nil {
			lastName = *req.LastName
		}
		_ = h.uc.UpdateName(ctx, user.ID, firstName, lastName)
	}

	session, err := h.createSessionHelper(
		r.Context(),
		user.ID, false, false, "general",
		nil, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
	)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"token":  session.AuthToken,
		"device": session.DeviceID,
	})
}


type TelegramLoginRequest struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  string `json:"auth_date"`
	Hash      string `json:"hash"`
}

func (h *AuthHandler) TelegramLogin(w http.ResponseWriter, r *http.Request) {
	var req TelegramLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Convert request to map for verification
	data := map[string]string{
		"id":         req.ID,
		"first_name": req.FirstName,
		"last_name":  req.LastName,
		"username":   req.Username,
		"photo_url":  req.PhotoURL,
		"auth_date":  req.AuthDate,
		"hash":       req.Hash,
	}

	if !h.telegramClient.VerifyTelegramAuth(data) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// At this point, the Telegram login is valid.
	// Now you can link/create user in DB using your usecase
	user, err := h.uc.HandleTelegramLogin(r.Context(), data)
	if err != nil {
		http.Error(w, "failed to process login", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}
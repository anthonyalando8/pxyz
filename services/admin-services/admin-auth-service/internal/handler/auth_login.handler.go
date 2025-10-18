package handler

import (
	"admin-auth-service/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"


	"x/shared/response"
	"x/shared/utils/errors"
)

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Step 1: Parse request
	req, err := h.parseLoginRequest(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Step 2: Authenticate user
	user, err := h.uc.LoginUser(r.Context(), req.Identifier, req.Password)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	// Step 5: Handle incomplete signup flows
	// if user.SignupStage != "complete" && user.SignupStage != "password_set" {
	// 	h.handleIncompleteProfile(w, r, user, req)
	// 	return
	// }

	h.handleSuccessfulLogin(w, r, user, req)
}

// 1. Parse and validate request
func (h *AuthHandler) parseLoginRequest(r *http.Request) (*LoginRequest, error) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}
	if req.Identifier == "" || req.Password == "" {
		return nil, fmt.Errorf("identifier and password are required")
	}
	return &req, nil
}

// 2. Handle errors from authentication
func (h *AuthHandler) handleAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, xerrors.ErrUserNotFound) {
		response.Error(w, http.StatusUnauthorized, "invalid credentials")
	} else {
		response.Error(w, http.StatusInternalServerError, "unexpected error occurred")
	}
}


// 6. Handle successful login
func (h *AuthHandler) handleSuccessfulLogin(w http.ResponseWriter, r *http.Request, user *domain.User, req *LoginRequest) {
	// ---- Nationality check via helper ----
	next, _ := "", ""

	// ---- Determine session type ----
	sessionType := "general"
	forceComplete := false
	if next != "" {
		sessionType = "incomplete_profile"
		forceComplete = true
	}

	// ---- Create session ----
	session, err := h.createSessionHelper(
		r.Context(),
		user.ID,
		forceComplete, // forceProfileCompletion
		false,         // isSudo
		sessionType,
		nil,
		req.DeviceID,
		req.DeviceMetadata,
		req.GeoLocation,
		r,
	)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
		return
	}

	// ---- Build response ----
	resp := map[string]interface{}{
		"token":  session.AuthToken,
		"device": session.DeviceID,
	}
	if next != "" {
		resp["next"] = next
	}

	response.JSON(w, http.StatusOK, resp)
}


// ensureNationality checks if user has nationality set.
// If missing, it tries to ensure profile exists (logs errors only),
// and returns the next action string ("set_nationality") if required.

func (h *AuthHandler) HandleUserExists(w http.ResponseWriter, r *http.Request) {
	var req UserExistsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Identifier == "" {
		response.Error(w, http.StatusBadRequest, "Identifier is required")
		return
	}

	exists, err := h.uc.UserExists(r.Context(), req.Identifier)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to check user existence")
		return
	}
	if exists {
		response.JSON(w, http.StatusOK, map[string]bool{"exists": true})
	} else {
		response.JSON(w, http.StatusOK, map[string]bool{"exists": false})
	}
}


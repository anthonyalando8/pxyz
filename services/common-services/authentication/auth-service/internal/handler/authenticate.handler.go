// handler/login_handler.go
package handler

import (
	//"auth-service/internal/usecase"
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"x/shared/auth/middleware"
	"x/shared/response"
)

// SubmitIdentifier handles the first step: identifier submission
// POST /api/v1/auth/submit-identifier
func (h *AuthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (h *AuthHandler) ServeLoginUI(w http.ResponseWriter, r *http.Request) {
    // Build the path to your UI folder
    uiDir := "./ui" // adjust if needed; relative to where binary runs
    file := filepath.Join(uiDir, "screen/login.html")

    http.ServeFile(w, r, file)
}
func (h *AuthHandler) SubmitIdentifier(w http.ResponseWriter, r *http.Request) {
	var req SubmitIdentifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	result, err := h.uc.SubmitIdentifier(r.Context(), req.Identifier)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Store OAuth2 context in session metadata if present
	var metadata map[string]interface{}
	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != "" {
		oauth2Data, _ := json.Marshal(req.OAuth2Context)
		metadata = map[string]interface{}{
			"oauth2_context": string(oauth2Data),
		}
	}

	session, err := h.createSessionHelper(
		r.Context(), result.UserID, true, false, "init_account", metadata,
		req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
	)
	if err != nil {
		log.Printf("[SubmitID] ❌ Failed to create temp session user=%s err=%v", result.UserID, err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	resp := map[string]interface{}{
		"user_id":      result.UserID,
		"next":         result.Next,
		"account_type": result.AccountType,
		"is_new_user":  result.IsNewUser,
		"token":        session.AuthToken,
		"device":       session.DeviceID,
	}

	// Include OAuth2 context in response if present
	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != "" {
		resp["oauth2_context"] = req.OAuth2Context
	}

	response.JSON(w, http.StatusOK, resp)
}

// VerifyIdentifier handles account verification
// POST /api/v1/auth/verify-identifier
func (h *AuthHandler) VerifyIdentifier(w http.ResponseWriter, r *http.Request) {
	var req VerifyIdentifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	userID, _, ok := h.getUserFromContext(r)
	if !ok {
		log.Printf("unauthorized: user not found in context")
		response.Error(w, http.StatusUnauthorized, "unauthorized: user not found in context")
		return
	}

	if err := h.uc.VerifyIdentifier(r.Context(), userID, req.Code); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	cachedData, err := h.uc.GetCachedUserData(r.Context(), userID)
	if err != nil {
		log.Printf("failed to get cached user data for user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "failed to get user data")
		return
	}

	nextStep := ""
	if !cachedData.HasPassword {
		nextStep = "set_password"
	} else {
		nextStep = "enter_password"
	}

	resp := map[string]interface{}{
		"message": "verification successful",
		"next":    nextStep,
	}

	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != "" {
		resp["oauth2_context"] = req.OAuth2Context
	}

	response.JSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) ResendOTP(w http.ResponseWriter, r *http.Request) {
    userID, _, ok := h.getUserFromContext(r)
    if !ok {
        response.Error(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    res, err := h.uc.ResendOTP(r.Context(), userID)
    if err != nil {
        response.Error(w, http.StatusBadRequest, err.Error())
        return
    }

    response.JSON(w, http.StatusOK, map[string]interface{}{
        "success": res.Success,
        "message": res.Message,
        "channel": res.Channel,
    })
}


// SetPassword handles password setting for new users or password recovery
// POST /api/v1/auth/set-password
func (h *AuthHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	var req SetPasswordRequest
	ctx := r.Context()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	userID, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	currentRoleVal := ctx.Value(middleware.ContextRole)
	currentRole, ok := currentRoleVal.(string)
	if !ok || currentRole == "" || currentRole == "temp" {
		currentRole = "any"
	}

	if err := h.uc.SetPasswordFromCache(r.Context(), userID, req.Password); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.postAccountCreationTask(userID, currentRole)

	// Check if this is an OAuth2 flow
	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != "" {
		// For OAuth2 flow, redirect to consent screen after setting password
		h.handleOAuth2PostAuth(w, r, userID, req.OAuth2Context)
		return
	}

	// Regular flow - create session and return token
	deviceID := h.getDeviceIDFromContext(r)
	session, err := h.createSessionHelper(
		r.Context(), userID, false, false, "general", nil, toPtr(deviceID), nil, nil, r,
	)
	if err != nil {
		log.Printf("[SetPassword] ❌ Failed to create main session user=%s err=%v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "password set successfully",
		"token":   session.AuthToken,
		"device":  session.DeviceID,
	})
}

// LoginWithPassword handles the final login step with password
// POST /api/v1/auth/login-password
func (h *AuthHandler) LoginWithPassword(w http.ResponseWriter, r *http.Request) {
	var req LoginPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	userID, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	cachedData, err := h.uc.GetCachedUserData(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "session expired")
		return
	}

	userWithCred, err := h.uc.GetUserByIdentifier(r.Context(), cachedData.Identifier)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if userWithCred.Credential.PasswordHash == nil {
		response.Error(w, http.StatusUnauthorized, "no password set")
		return
	}

	if !utils.CheckPasswordHash(req.Password, *userWithCred.Credential.PasswordHash) {
		response.Error(w, http.StatusUnauthorized, "invalid password")
		return
	}

	_ = h.uc.ClearCachedUserData(r.Context(), userID)

	// Check if this is an OAuth2 flow
	if req.OAuth2Context != nil && req.OAuth2Context.ClientID != ""{
		h.handleOAuth2PostAuth(w, r, userID, req.OAuth2Context)
		return
	}

	// Regular flow - create session and return token
	deviceID := h.getDeviceIDFromContext(r)
	session, err := h.createSessionHelper(
		r.Context(), userID, false, false, "general", nil, toPtr(deviceID), nil, nil, r,
	)
	if err != nil {
		log.Printf("[Login] ❌ Failed to create main session user=%s err=%v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "login successful",
		"user_id": userID,
		"token":   session.AuthToken,
		"device":  session.DeviceID,
	})
}

func (h *AuthHandler) handleOAuth2PostAuth(w http.ResponseWriter, r *http.Request, userID string, oauth2Ctx *OAuth2Context) {
	ctx := r.Context()

	// Check if consent already exists
	consent, err := h.oauth2Svc.GetConsentInfo(ctx, oauth2Ctx.ClientID, userID, oauth2Ctx.Scope)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to check consent")
		return
	}

	if consent.HasExistingConsent {
		// User already granted consent, generate code directly
		authReq := &domain.OAuth2AuthorizationRequest{
			ClientID:            oauth2Ctx.ClientID,
			RedirectURI:         oauth2Ctx.RedirectURI,
			Scope:               oauth2Ctx.Scope,
			State:               oauth2Ctx.State,
			CodeChallenge:       oauth2Ctx.CodeChallenge,
			CodeChallengeMethod: oauth2Ctx.CodeChallengeMethod,
		}

		code, err := h.oauth2Svc.AuthorizeRequest(ctx, authReq, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}

		redirectURL := h.oauth2Svc.BuildAuthorizationResponse(oauth2Ctx.RedirectURI, code, oauth2Ctx.State)
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"redirect_url": redirectURL,
			"requires_consent": false,
		})
		return
	}

	// User needs to grant consent
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"requires_consent": true,
		"consent_info":     consent,
		"oauth2_context":   oauth2Ctx,
	})
}

// GetCachedUserStatus retrieves cached user data (for debugging or frontend state)
// GET /api/v1/auth/cached-status/:user_id
func (h *AuthHandler) GetCachedUserStatus(w http.ResponseWriter, r *http.Request) {
	 // Extract userID from context (ignore client-provided one)
    userID, _, ok := h.getUserFromContext(r)
    if !ok {
        response.Error(w, http.StatusUnauthorized, "unauthorized")
        return
    }

	cachedData, err := h.uc.GetCachedUserData(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no cached data found")
		return
	}

	// Determine next step based on cached data
	nextStep := ""
	if !cachedData.IsEmailVerified && !cachedData.IsPhoneVerified {
		nextStep = "verify_account"
	} else if !cachedData.HasPassword {
		nextStep = "set_password"
	} else {
		nextStep = "enter_password"
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"user_id":          cachedData.UserID,
		"account_type":     cachedData.AccountType,
		"is_new_user":      cachedData.IsNewUser,
		"is_verified":      cachedData.IsEmailVerified || cachedData.IsPhoneVerified,
		"has_password":     cachedData.HasPassword,
		"next":             nextStep,
	})
}
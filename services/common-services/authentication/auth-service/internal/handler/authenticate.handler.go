// handler/login_handler.go
package handler

import (
	//"auth-service/internal/usecase"
	"auth-service/pkg/utils"
	"encoding/json"
	"log"
	"net/http"
	"x/shared/response"

)

// SubmitIdentifierRequest matches the usecase type
type SubmitIdentifierRequest struct {
	Identifier string `json:"identifier" binding:"required"`

	DeviceID       *string     `json:"device_id"`
	GeoLocation    *string     `json:"geo_location"`
	DeviceMetadata *any        `json:"device_metadata"`
}

// VerifyIdentifierRequest for verification step
type VerifyIdentifierRequest struct {
	Code   string `json:"code" binding:"required"`
}

// LoginPasswordRequest for final login step
type LoginPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// SubmitIdentifier handles the first step: identifier submission
// POST /api/v1/auth/submit-identifier
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

	session, err := h.createSessionHelper(
		r.Context(),result.UserID,true,false,"init_account",nil,req.DeviceID,req.DeviceMetadata, req.GeoLocation, r,
	)
	if err != nil {
		log.Printf("[SubmitID] ❌ Failed to create temp session user=%s err=%v", result.UserID, err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"user_id":      result.UserID,
		"next":         result.Next,
		"account_type": result.AccountType,
		"is_new_user":  result.IsNewUser,
		"token": session.AuthToken,
		"device": session.DeviceID,
	})
}

// VerifyIdentifier handles account verification
// POST /api/v1/auth/verify-identifier
func (h *AuthHandler) VerifyIdentifier(w http.ResponseWriter, r *http.Request) {
	var req VerifyIdentifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	 // Extract userID from context (ignore client-provided one)
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

	// Get updated cached data to determine next step
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

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "verification successful",
		"next":    nextStep,
	})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	userID, _, ok := h.getUserFromContext(r)
    if !ok {
        response.Error(w, http.StatusUnauthorized, "unauthorized")
        return
    }

	if err := h.uc.SetPasswordFromCache(r.Context(), userID, req.Password); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	deviceID := h.getDeviceIDFromContext(r)
	session, err := h.createSessionHelper(
		r.Context(),userID,false,false,"general",nil,toPtr(deviceID),nil, nil, r,
	)
	if err != nil {
		log.Printf("[SetPassword] ❌ Failed to create main session user=%s err=%v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}
	// generate main token afterwards if needed
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "password set successfully",
		"token":    session.AuthToken,
		"device": session.DeviceID,
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
	 // Extract userID from context (ignore client-provided one)
    userID, _, ok := h.getUserFromContext(r)
    if !ok {
        response.Error(w, http.StatusUnauthorized, "unauthorized")
        return
    }
	// Get cached data
	cachedData, err := h.uc.GetCachedUserData(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "session expired")
		return
	}

	// Get user with credentials
	userWithCred, err := h.uc.GetUserByIdentifier(r.Context(), cachedData.Identifier)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Verify password
	if userWithCred.Credential.PasswordHash == nil {
		response.Error(w, http.StatusUnauthorized, "no password set")
		return
	}

	if !utils.CheckPasswordHash(req.Password, *userWithCred.Credential.PasswordHash) {
		response.Error(w, http.StatusUnauthorized, "invalid password")
		return
	}

	// Clear cached data
	_ = h.uc.ClearCachedUserData(r.Context(), userID)

	deviceID := h.getDeviceIDFromContext(r)
	session, err := h.createSessionHelper(
		r.Context(),userID,false,false,"general",nil,toPtr(deviceID),nil, nil, r,
	)
	if err != nil {
		log.Printf("[Login] ❌ Failed to create main session user=%s err=%v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "login successful",
		"user_id": userID,
		 "token":  session.AuthToken,
		 "device": session.DeviceID,
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
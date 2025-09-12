package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"ptn-auth-service/internal/domain"
	"x/shared/auth/middleware"
	"x/shared/response"
)
type Enable2FARequest struct {
	Method string `json:"method"`
	Secret string `json:"secret"`
	Code   string `json:"totp_code"`
}

type Disable2FARequest struct {
	Method     string `json:"method"`
	Code       string `json:"totp_code"`
	BackupCode string `json:"backup_code"`
}

type Verify2FARequest struct {
	Method     string `json:"method"`
	Code       string `json:"totp_code"`
	BackupCode string `json:"backup_code"`
}

// --- Helper: Extract user ID and user object from context ---
func (h *AuthHandler) getUserFromContext(r *http.Request) (string, *domain.User, bool) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		return "", nil, false
	}
	user, err := h.uc.FindUserById(r.Context(), userID)
	if err != nil {
		return "", nil, false
	}
	return userID, user, true
}

// --- Helper: Determine communication target (email/phone) ---
func getComTarget(user *domain.User, emailOnly bool) (string, error) {
	if user.Email != nil && *user.Email != "" {
		return *user.Email, nil
	}
	if !emailOnly {
		if user.Phone != nil && *user.Phone != "" {
			return *user.Phone, nil
		}
	}

	return "", fmt.Errorf("no valid identifier available")
}

// --- Helper: Decode request body into target struct ---
func decodeRequestBody(r *http.Request, target interface{}) error {
	return json.NewDecoder(r.Body).Decode(target)
}

func (h *AuthHandler) HandleInitiate2FA(w http.ResponseWriter, r *http.Request) {
	_, user, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	_, err := getComTarget(user, false)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"totp_url":    "",
		"totp_secret": "",
		"next":        "scan_code",
	})
}

func (h *AuthHandler) HandleEnable2FA(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req Enable2FARequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" || req.Secret == "" {
		response.Error(w, http.StatusBadRequest, "Code and secret required")
		return
	}

	//comTarget, _ := getComTarget(user, true) // optional, empty string allowed
	

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":     "2FA added successfully.",
		"backup_code": "resp.BackupCodes",
	})
}

func (h *AuthHandler) HandleDisable2FA(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req Disable2FARequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" && req.BackupCode == "" {
		response.Error(w, http.StatusBadRequest, "TOTP code or backup codes required")
		return
	}

	//comTarget, _ := getComTarget(user, true) // optional, empty string allowed


	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "2FA disabled successfully.",
	})
}

func (h *AuthHandler) HandleVerify2FA(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req Disable2FARequest
	if err := decodeRequestBody(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" && req.BackupCode == "" {
		response.Error(w, http.StatusBadRequest, "TOTP code or backup codes required")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Success",
		"valid":   "false",
	})
}

func (h *AuthHandler) Handle2FAStatus(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.getUserFromContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Success",
		"valid":   "resp.IsEnabled",
	})
}

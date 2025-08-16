package handler

import (
	"auth-service/pkg/utils"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"x/shared/auth/middleware"
	"x/shared/response"
)

// Change password (requires old + new)
func (h *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.uc.UpdatePassword(r.Context(), userID, req.NewPassword, true, req.OldPassword, false); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Password updated"})
}

// Reset password (via OTP/email link)
func (h *AuthHandler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.uc.UpdatePassword(r.Context(), userID, req.NewPassword, false, "", false); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Password reset successful"})
}

// Set password (signup flow)
func (h *AuthHandler) HandleSetPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	deviceID, ok2 := r.Context().Value(middleware.ContextDeviceID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	if !ok2 || deviceID == "" || deviceID == "unknown"{
		response.Error(w, http.StatusUnauthorized, "Unauthorized device")
		return
	}

	var req SetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.uc.UpdatePassword(r.Context(), userID, req.NewPassword, false, "", true); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	session, sessErr := h.createSessionHelper(
		r.Context(),
		userID, false, false, "general",
		nil, &deviceID, nil, nil, r,
	)
	if sessErr != nil {
		log.Printf("Failed to create temp session: %v", sessErr)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Password set successfully", "token": session.AuthToken})
}


func (h *AuthHandler) HandleChangeEmail(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ChangeEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	
	req.UserID = requestedUserID
	if req.NewEmail == "" {
		response.Error(w, http.StatusBadRequest, "New email required")
		return
	}

	if valid := utils.ValidateEmail(req.NewEmail); !valid {
		response.Error(w, http.StatusBadRequest, "Invalid email format")
		return
	}


	err := h.uc.ChangeEmail(r.Context(), req.UserID, req.NewEmail)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to change email: %v", err))
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Email updated successfully"})
}


func (h *AuthHandler) HandleUpdateName(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req UpdateNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		response.Error(w, http.StatusBadRequest, "First name and last name are required")
		return
	}
	if len(req.FirstName) < 3 || len(req.LastName) < 3 {
		response.Error(w, http.StatusBadRequest, "First name and last name must be at least 2 characters long")
		return
	}
	req.UserID = requestedUserID
	err := h.uc.UpdateName(r.Context(), req.UserID, req.FirstName, req.LastName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update name: %v", err))
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"message": "Name updated successfully"})
}

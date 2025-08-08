package handler

import (
	"auth-service/pkg/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"x/shared/auth/middleware"
	"x/shared/response"
)

func (h *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if req.NewPassword == "" {
		response.Error(w, http.StatusBadRequest, "New password required")
		return
	}
	
	if valid, err := utils.ValidatePassword(req.NewPassword); !valid {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	
	req.UserID = requestedUserID
	err := h.uc.ChangePassword(r.Context(), req.UserID, req.NewPassword)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to change password: %v", err))
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
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

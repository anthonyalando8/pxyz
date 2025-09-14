package handler

import (
	"encoding/json"
	"net/http"

	"admin-auth-service/pkg/utils"
	"admin-auth-service/internal/service/email"
	"x/shared/response"
)

// HandleRegister handles registration of a super admin user
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "All fields (email, password) are required")
		return
	}

	if valid := utils.ValidateEmail(req.Email); req.Email != "" && !valid {
		response.Error(w, http.StatusBadRequest, "invalid email format")
		return
	}

	if valid, err := utils.ValidatePassword(req.Password); !valid {
		response.Error(w, http.StatusBadRequest, "weak password: "+err.Error())
		return
	}

	// Register user with "super_admin" role
	user, err := h.uc.RegisterUser(r.Context(), req.Email, req.Password, req.FirstName, req.LastName, "super_admin")
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Send welcome email in background
	go func(email, password, userID string) {
		helper := emailhelper.NewAdminEmailHelper(h.emailClient)
		helper.SendAdminAccountCreated(r.Context(), userID, email, email, password)
	}(req.Email, req.Password, user.ID)


	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Admin registered successfully",
		"user":    user,
	})
}


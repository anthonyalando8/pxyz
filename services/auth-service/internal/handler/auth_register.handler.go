package handler

import (
	"encoding/json"
	"net/http"
	"x/shared/response"
	"auth-service/pkg/utils"
)

func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if (req.Email == "" && req.Phone == "") || req.Password == ""/* || req.FirstName == "" || req.LastName == "" */{
		response.Error(w, http.StatusBadRequest, "All fields (email or phone, password) are required")
		return
	}

	if valid := utils.ValidateEmail(req.Email); req.Email != "" && !valid {
		response.Error(w, http.StatusBadRequest, "invalid email format")
		return
	}
	if req.Phone != "" && !utils.ValidatePhone(req.Phone) {
		response.Error(w, http.StatusBadRequest, "invalid phone format")
		return
	}
	
	if valid, err := utils.ValidatePassword(req.Password); !valid {
		response.Error(w, http.StatusBadRequest, "weak password: " + err.Error())
		return
	}

	user, err := h.uc.RegisterUser(r.Context(), req.Email, req.Phone, req.Password, req.FirstName, req.LastName)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	session, err := h.createSessionHelper(r.Context(), user.ID, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"token":      session.AuthToken,
		"device":     session.DeviceID,
		"user_id":    user.ID,
		"email":      *user.Email,
		"first_name": *user.FirstName,
		"last_name":  *user.LastName,
	})
}

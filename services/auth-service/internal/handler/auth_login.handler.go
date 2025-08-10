package handler

import (
	"encoding/json"
	"net/http"
	"x/shared/response"
)

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Identifier == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "Identifier and password are required")
		return
	}

	user, err := h.uc.LoginUser(r.Context(), req.Identifier, req.Password)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	session, err := h.createSessionHelper(r.Context(), user.ID, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"token":      session.AuthToken,
		"device":     session.DeviceID,
		"user_id":    user.ID,
		"email":      *user.Email,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
	})
}

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


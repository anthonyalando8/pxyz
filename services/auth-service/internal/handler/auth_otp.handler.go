package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"x/shared/genproto/otppb"
	"x/shared/response"
)

func (h *AuthHandler) HandleRequestOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"`
		Purpose    string `json:"purpose"` // e.g. "login", "password_reset"
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Identifier == "" {
		response.Error(w, http.StatusBadRequest, "Identifier is required")
		return
	}
	if req.Purpose == ""{
		response.Error(w, http.StatusBadRequest, "OTP Purpose required")
		return	
	}

	// Lookup user by identifier (user_id)
	user, err := h.uc.FindUserById(r.Context(), req.Identifier)
	if err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	// Request OTP from OTP service
	resp, err := h.otp.Client.GenerateOTP(context.Background(), &otppb.GenerateOTPRequest{
		UserId:    user.ID,
		Channel:   "email", // could be "sms"
		Purpose:   req.Purpose,
		Recipient: *user.Email,
	})
	if err != nil {
		log.Printf("Failed to generate OTP: %v", err)
		response.Error(w, http.StatusInternalServerError, "Failed to generate OTP")
		return
	}

	if !resp.Ok {
		response.Error(w, http.StatusInternalServerError, resp.Error)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "OTP code sent successfully",
	})
}


func (h *AuthHandler) HandleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"`
		OtpCode    string `json:"otp_code"`
		Purpose    string `json:"purpose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Identifier == "" || req.OtpCode == "" {
		response.Error(w, http.StatusBadRequest, "Identifier and OTP code are required")
		return
	}

	// Lookup user by identifier (user_id)
	user, err := h.uc.FindUserById(r.Context(), req.Identifier)
	if err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	idInt, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Something went wrong!")
		return
	}
	// Verify OTP with OTP service
	resp, err := h.otp.Client.VerifyOTP(context.Background(), &otppb.VerifyOTPRequest{
		UserId:  idInt,
		Purpose: req.Purpose,
		Code:    req.OtpCode,
	})
	if err != nil {
		log.Printf("Failed to verify OTP: %v", err)
		response.Error(w, http.StatusInternalServerError, "Failed to verify OTP")
		return
	}

	if !resp.Valid {
		response.Error(w, http.StatusUnauthorized, resp.Error)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "OTP verified successfully",
	})
}


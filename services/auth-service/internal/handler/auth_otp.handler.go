package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"x/shared/genproto/otppb"
	"x/shared/response"
	"x/shared/auth/middleware"
)

func (h *AuthHandler) HandleRequestOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Purpose string `json:"purpose"` // e.g. "login", "password_reset", "verify_email"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Purpose == "" {
		response.Error(w, http.StatusBadRequest, "OTP purpose is required")
		return
	}

	// Extract user ID from context (middleware must have set it)
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Fetch user (we need email/phone to know recipient & channel)
	user, err := h.uc.FindUserById(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	// Determine channel dynamically (fallback to email if phone missing)
	var channel, recipient string
	if user.Phone != nil && *user.Phone != "" {
		channel = "sms"
		recipient = *user.Phone
	} else if user.Email != nil && *user.Email != "" {
		channel = "email"
		recipient = *user.Email
	} else {
		response.Error(w, http.StatusBadRequest, "No valid recipient available for OTP")
		return
	}

	// Request OTP from OTP service
	resp, err := h.otp.Client.GenerateOTP(
		r.Context(),
		&otppb.GenerateOTPRequest{
			UserId:    user.ID,
			Channel:   channel,
			Purpose:   req.Purpose,
			Recipient: recipient,
		},
	)
	if err != nil {
		log.Printf("Failed to generate OTP for user %s: %v", user.ID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to generate OTP")
		return
	}
	if !resp.Ok {
		response.Error(w, http.StatusInternalServerError, resp.Error)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "OTP sent successfully",
		"channel": channel,
	})
}


func (h *AuthHandler) HandleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OtpCode string `json:"otp_code"`
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.OtpCode == "" {
		response.Error(w, http.StatusBadRequest, "OTP code is required")
		return
	}

	// Get userId from context (set by middleware)
	userId, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userId == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Convert userId to int64 for OTP service
	idInt, err := strconv.ParseInt(userId, 10, 64)
	if err != nil {
		log.Printf("Invalid user ID in context: %v", err)
		response.Error(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Verify OTP via service
	resp, err := h.otp.Client.VerifyOTP(r.Context(), &otppb.VerifyOTPRequest{
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
		response.Error(w, http.StatusUnauthorized, "Invalid or expired OTP")
		return
	}

	// Handle post-verification actions ONLY if extra data exists
	action := ""
	if extra, ok := r.Context().Value(middleware.ContextExtraData).(map[string]string); ok && extra != nil {
		if val, ok := extra["next"]; ok && val != "" {
			action = val
			switch action {
			case "verify_email":
				if ok, err := h.uc.VerifyEmail(r.Context(), userId); err != nil || !ok {
					response.Error(w, http.StatusInternalServerError, "Email verification failed")
					return
				}
			case "verify_phone":
				if ok, err := h.uc.VerifyPhone(r.Context(), userId); err != nil || !ok {
					response.Error(w, http.StatusInternalServerError, "Phone verification failed")
					return
				}
			default:
				log.Printf("Unknown action: %s", action)
			}
		}
	}

	// Respond
	response.JSON(w, http.StatusOK, map[string]string{
		"message": "OTP verified successfully",
		"action":  action, // empty if none
	})
}




package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"x/shared/auth/middleware"
	"x/shared/genproto/otppb"
	"x/shared/response"
)

func (h *AuthHandler) HandleRequestOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Purpose string `json:"purpose"` // e.g. "login", "password_reset", "verify_email"
		Channel string `json:"channel"` // optional, e.g. "sms", "email"
		Target  string `json:"target"`  // optional override
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// --- Allowed purposes & channels ---
	allowedPurposes := map[string]bool{
		"login":          true,
		"password_reset": true,
		"verify_email":   true,
		"verify_phone":   true,
		"email_change":   true,
		"register":       true,
	}
	allowedChannels := map[string]bool{
		"sms":   true,
		"email": true,
	}

	// --- Validate purpose ---
	if req.Purpose == "" || !allowedPurposes[req.Purpose] {
		response.Error(w, http.StatusBadRequest, "Invalid or unsupported OTP purpose")
		return
	}

	// --- Extract user ID from context ---
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// --- Fetch user ---
	user, err := h.uc.FindUserById(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	// --- Determine channel & recipient ---
	var channel, recipient string

	// --- If target explicitly provided, prefer it ---
	if req.Target != "" {
		recipient = req.Target

		// If channel not specified, infer from target
		if req.Channel == "" {
			if strings.Contains(recipient, "@") {
				channel = "email"
			} else {
				channel = "sms"
			}
		} else {
			if !allowedChannels[req.Channel] {
				response.Error(w, http.StatusBadRequest, "Invalid channel")
				return
			}
			channel = req.Channel
		}
	} else {
		// --- Fallback: no target provided, use stored contact ---
		if req.Channel != "" {
			// Validate explicitly requested channel
			if !allowedChannels[req.Channel] {
				response.Error(w, http.StatusBadRequest, "Invalid channel")
				return
			}
			switch req.Channel {
			case "sms":
				if user.Phone == nil || *user.Phone == "" {
					response.Error(w, http.StatusBadRequest, "User does not have a valid phone for SMS")
					return
				}
				channel = "sms"
				recipient = *user.Phone
			case "email":
				if user.Email == nil || *user.Email == "" {
					response.Error(w, http.StatusBadRequest, "User does not have a valid email for Email OTP")
					return
				}
				channel = "email"
				recipient = *user.Email
			}
		} else {
			// No channel provided → auto-fallback
			if user.Email != nil && *user.Email != "" {
				channel = "email"
				recipient = *user.Email
			}else if user.Phone != nil && *user.Phone != "" {
				channel = "sms"
				recipient = *user.Phone
			} else {
				response.Error(w, http.StatusBadRequest, "No valid recipient available for OTP")
				return
			}
		}
	}

	// --- Request OTP from OTP service ---
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
		"target":  recipient,
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

	if !h.VerifyOtpHelper(w, r.Context(), userId, req.OtpCode, req.Purpose) {
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

func (h *AuthHandler) VerifyOtpHelper(w http.ResponseWriter, ctx context.Context, userId, otpCode, purpose string) bool {
	idInt, err := strconv.ParseInt(userId, 10, 64)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Internal error")
		return false
	}

	resp, err := h.otp.Client.VerifyOTP(ctx, &otppb.VerifyOTPRequest{
		UserId:  idInt,
		Purpose: purpose,
		Code:    otpCode,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to verify OTP")
		return false
	}
	if !resp.Valid {
		response.Error(w, http.StatusUnauthorized, "Invalid or expired OTP")
		return false
	}
	return true
}


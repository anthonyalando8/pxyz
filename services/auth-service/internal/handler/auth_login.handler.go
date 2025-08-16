package handler

import (
	"auth-service/pkg/utils"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"x/shared/genproto/otppb"
	"x/shared/response"
	"x/shared/utils/errors"
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
		if errors.Is(err, xerrors.ErrUserNotFound) {
			response.Error(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Unexpected error occured")
		return
	}
	if user.AccountType == "social"{
		response.Error(w, http.StatusUnauthorized, "identifier linked to a social account")
		return
	}

	// Handle incomplete profile cases
	if user.SignupStage != "complete" && user.SignupStage != "password_set" {
		// Decide next step
		next := ""
		channel := ""
		target := ""

		switch user.SignupStage {
		case "email_or_phone_submitted":
			// still need OTP verification
			if user.Email != nil && *user.Email != "" {
				channel = "email"
				target = *user.Email
			} else if user.Phone != nil && *user.Phone != "" {
				channel = "sms"
				target = *user.Phone
			}
			next = "verify_otp"
		case "otp_verified":
			next = "set_password"
		case "password_set":
			next = "set_name"
		}

		// create register session
		extra := map[string]string{"next": next}
		session, sessErr := h.createSessionHelper(
			r.Context(),
			user.ID, true, false, "register",
			extra, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
		)
		if sessErr != nil {
			log.Printf("Failed to create session: %v", sessErr)
			response.Error(w, http.StatusInternalServerError, "Session creation failed")
			return
		}

		// If OTP stage → trigger OTP send
		if next == "verify_otp" {
			otpResp, otpErr := h.otp.Client.GenerateOTP(
				r.Context(),
				&otppb.GenerateOTPRequest{
					UserId:    user.ID,
					Channel:   channel,
					Purpose:   "register",
					Recipient: target,
				},
			)
			if otpErr != nil || otpResp == nil || !otpResp.Ok {
				log.Printf("OTP generation failed: %v, resp: %v", otpErr, otpResp)
				// don’t block login; user can retry OTP later
			}
		}

		// structured response for incomplete flow
		response.JSON(w, http.StatusConflict, map[string]interface{}{
			"error":       "incomplete_profile",
			"stage":       user.SignupStage,
			"next_stage":  next,
			"token":       session.AuthToken,
			"device":      session.DeviceID,
		})
		return
	}

	// Password / Hybrid account must have a password
	if user.AccountType == "password" || user.AccountType == "hybrid" {
		if user.PasswordHash == nil {
			// Create a short-lived session so user can set password
			//extra := map[string]string{"next": "set_password"}
			session, sessErr := h.createSessionHelper(
				r.Context(),
				user.ID, true, false, "register",
				nil, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
			)
			if sessErr != nil {
				log.Printf("Failed to create temp session: %v", sessErr)
				response.Error(w, http.StatusInternalServerError, "Temporary session creation failed")
				return
			}

			response.JSON(w, http.StatusConflict, map[string]interface{}{
				"error":      "no_password_set",
				"next_stage": "set_password_email",
				"token":      session.AuthToken,
				"device":     session.DeviceID,
			})
			return
		}

		// Check password hash
		if !utils.CheckPasswordHash(req.Password, *user.PasswordHash) {
			response.Error(w, http.StatusUnauthorized, "invalid password")
			return
		}
	}

	// Normal flow
	session, err := h.createSessionHelper(r.Context(), user.ID, false, false, "general", nil, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"token":  session.AuthToken,
		"device": session.DeviceID,
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


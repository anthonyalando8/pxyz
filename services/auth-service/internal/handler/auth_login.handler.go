package handler

import (
	"auth-service/pkg/utils"
	"auth-service/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"x/shared/genproto/otppb"
	"x/shared/response"
	"x/shared/utils/errors"
)

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Step 1: Parse request
	req, err := h.parseLoginRequest(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Step 2: Authenticate user
	user, err := h.uc.LoginUser(r.Context(), req.Identifier, req.Password)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	// Step 3: Reject social-only accounts
	if err := h.ensureNotSocial(user, w); err != nil {
		return
	}

	// Step 4: Handle incomplete signup flows
	if user.SignupStage != "complete" && user.SignupStage != "password_set" {
		h.handleIncompleteProfile(w, r, user, req)
		return
	}

	// Step 5: Ensure password accounts have a password
	if user.AccountType == "password" || user.AccountType == "hybrid" {
		if err := h.ensurePasswordSet(w, r, user, req); err != nil {
			return
		}
	}

	// Step 6: Normal login flow
	h.handleSuccessfulLogin(w, r, user, req)
}

// 1. Parse and validate request
func (h *AuthHandler) parseLoginRequest(r *http.Request) (*LoginRequest, error) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}
	if req.Identifier == "" || req.Password == "" {
		return nil, fmt.Errorf("identifier and password are required")
	}
	return &req, nil
}

// 2. Handle errors from authentication
func (h *AuthHandler) handleAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, xerrors.ErrUserNotFound) {
		response.Error(w, http.StatusUnauthorized, "invalid credentials")
	} else {
		response.Error(w, http.StatusInternalServerError, "unexpected error occurred")
	}
}

// 3. Ensure user is not a social-only account
func (h *AuthHandler) ensureNotSocial(user *domain.User, w http.ResponseWriter) error {
	if user.AccountType == "social" {
		response.Error(w, http.StatusUnauthorized, "identifier linked to a social account")
		return errors.New("social login not allowed")
	}
	return nil
}

// 4. Handle incomplete signup flows
func (h *AuthHandler) handleIncompleteProfile(
	w http.ResponseWriter,
	r *http.Request,
	user *domain.User,
	req *LoginRequest,
) {
	stage := user.SignupStage
	next := ""
	channel := ""
	target := ""

	if user.Email != nil && *user.Email != "" {
		channel = "email"
		target = *user.Email
	} else if user.Phone != nil && *user.Phone != "" {
		channel = "sms"
		target = *user.Phone
	}

	// Determine next stage
	switch stage {
	case "email_or_phone_submitted":
		next = "verify_otp"

	case "otp_verified":
		next = "set_password"

	// case "password_set":
	// 	next = "set_name"
	}

	// Match signup handler semantics
	var sessionPurpose string
	extra := map[string]string{
		"stage":      stage,
		"next_stage": next,
	}

	if next == "verify_otp" {
		sessionPurpose = "register"
		extra["next"] = fmt.Sprintf("verify_%s", channel)
	} else {
		sessionPurpose = "incomplete_profile"
		extra["next"] = "incomplete_profile"
	}

	// Create TEMP session
	session, err := h.createSessionHelper(
		r.Context(),
		user.ID,
		true,  // temp session
		false, // not refresh
		sessionPurpose,
		extra,
		req.DeviceID,
		req.DeviceMetadata,
		req.GeoLocation,
		r,
	)
	if err != nil {
		log.Printf("[handleIncompleteProfile] ❌ Failed to create session for user=%s, err=%v", user.ID, err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
		return
	}

	// Always send OTP (even if not the real next)
	if channel != "" && target != "" {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			r.Context(),
			&otppb.GenerateOTPRequest{
				UserId:    user.ID,
				Channel:   channel,
				Purpose:   sessionPurpose,
				Recipient: target,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[handleIncompleteProfile] ❌ OTP generation failed for user=%s: err=%v resp=%v",
				user.ID, otpErr, otpResp.GetError(),
			)
			response.Error(w, http.StatusInternalServerError, "OTP generation failed")
			return
		}
	}

	// Response → always enforce OTP as first step, but keep real_next
	response.JSON(w, http.StatusConflict, map[string]interface{}{
		"error":       "incomplete_profile",
		"stage":       stage,
		"next":  "verify-otp", // immediate requirement
		"purpose":    sessionPurpose,
		"otp_channel": channel,
		"token":  session.AuthToken,
		"device":      session.DeviceID,
	})
}


// 5. Ensure password is set for password/hybrid accounts
func (h *AuthHandler) ensurePasswordSet(w http.ResponseWriter, r *http.Request, user *domain.User, req *LoginRequest) error {
	if user.PasswordHash == nil {
		// Temp session for setting password
		session, err := h.createSessionHelper(
			r.Context(),
			user.ID, true, false, "register",
			nil, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
		)
		if err != nil {
			log.Printf("Failed to create temp session: %v", err)
			response.Error(w, http.StatusInternalServerError, "temporary session creation failed")
			return err
		}
		response.JSON(w, http.StatusConflict, map[string]interface{}{
			"error":      "no_password_set",
			"next": "set_password_email",
			"token":      session.AuthToken,
			"device":     session.DeviceID,
		})
		return errors.New("password not set")
	}

	// Validate password hash
	if !utils.CheckPasswordHash(req.Password, *user.PasswordHash) {
		response.Error(w, http.StatusUnauthorized, "invalid password")
		return errors.New("invalid password")
	}
	return nil
}

// 6. Handle successful login
func (h *AuthHandler) handleSuccessfulLogin(w http.ResponseWriter, r *http.Request, user *domain.User, req *LoginRequest) {
	session, err := h.createSessionHelper(
		r.Context(),
		user.ID, false, false, "general",
		nil, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
	)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "session creation failed")
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


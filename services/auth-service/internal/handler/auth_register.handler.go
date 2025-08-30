package handler

import (
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"x/shared/genproto/otppb"
	"x/shared/response"
	"x/shared/utils/errors"
)


func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if (req.Email == "") || req.Password == ""/* || req.FirstName == "" || req.LastName == "" */{
		response.Error(w, http.StatusBadRequest, "All fields (email, password) are required")
		return
	}

	if valid := utils.ValidateEmail(req.Email); req.Email != "" && !valid {
		response.Error(w, http.StatusBadRequest, "invalid email format")
		return
	}
	
	if valid, err := utils.ValidatePassword(req.Password); !valid {
		response.Error(w, http.StatusBadRequest, "weak password: " + err.Error())
		return
	}

	user, err := h.uc.RegisterUser(r.Context(), req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	session, err := h.createSessionHelper(r.Context(), user.ID, false, false, "general",nil, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		response.Error(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"token":      session.AuthToken,
		"device":     session.DeviceID,
	})
}

func (h *AuthHandler) HandleInitSignup(w http.ResponseWriter, r *http.Request) {
	var req RegisterInit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Step 1: validate input
	if err := validateSignupRequest(req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Step 2: create or fetch partial user
	user, err := h.uc.CreatePartialUser(r.Context(), req.Email)
	if err != nil {
		h.handlePartialUserError(w, r, err, req, user)
		return
	}

	// Step 3: fresh user → OTP + session
	if err := h.handleFreshUserSignup(w, r, req, user); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func validateSignupRequest(req RegisterInit) error {
	if req.Email != "" && !utils.ValidateEmail(req.Email) {
		return errors.New("invalid email format")
	}
	if !req.AcceptTerms{
		return errors.New("you must accept terms and conditions to register")
	} 
	return nil
}
func (h *AuthHandler) handlePartialUserError(
	w http.ResponseWriter, r *http.Request,
	err error, req RegisterInit, user *domain.User,
) {
	if user == nil {
		log.Printf("[handlePartialUserError] ❌ Nil user received, err=%v", err)
		response.Error(w, http.StatusInternalServerError, "Internal server error: user not found")
		return
	}

	if signupErr, ok := err.(*xerrors.SignupError); ok {
		channel, target := detectChannel(req)

		var sessionPurpose string
		extraData := map[string]string{
			"stage":       signupErr.Stage,
			"next_stage":  signupErr.NextStage,
		}

		if signupErr.NextStage == "verify_otp" {
			// 🔑 Normal registration
			sessionPurpose = "register"
			extraData["next"] = fmt.Sprintf("verify_%s", channel)
		} else {
			// 🔑 Other stages (password, profile, etc.)
			sessionPurpose = "incomplete_profile"
			extraData["next"] = "incomplete_profile"
		}

		session, sessErr := h.createSessionHelper(
			r.Context(),
			user.ID,
			true, // temp session if incomplete profile
			false, // isRefresh
			sessionPurpose,
			extraData,
			req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
		)
		if sessErr != nil {
			log.Printf("[handlePartialUserError] ❌ Session creation failed userID=%v, err=%v", user.ID, sessErr)
			response.Error(w, http.StatusInternalServerError, "Session creation failed: "+sessErr.Error())
			return
		}

		// Always send OTP
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
			log.Printf("[handlePartialUserError] ❌ OTP generation failed userID=%v, otpErr=%v, serviceErr=%v",
				user.ID, otpErr, otpResp.GetError(),
			)
			response.Error(w, http.StatusInternalServerError, "OTP generation failed")
			return
		}

		// Response → next_stage always "verify_otp"
		resp := map[string]interface{}{
			"error":       "incomplete_profile",
			"stage":       signupErr.Stage,
			"next":  "verify-otp",
			"purpose":    sessionPurpose,
			"token":  session.AuthToken,
			"device":      session.DeviceID,
			"otp_channel": channel,
		}

		response.JSON(w, http.StatusConflict, resp)
		return
	}

	// Already exists
	if errors.Is(err, xerrors.ErrEmailAlreadyInUse) {
		response.Error(w, http.StatusConflict, "Email already in use")
		return
	}
	if errors.Is(err, xerrors.ErrPhoneAlreadyInUse) {
		response.Error(w, http.StatusConflict, "Phone already in use")
		return
	}

	// Fallback
	response.Error(w, http.StatusInternalServerError, "Unexpected error: "+err.Error())
}



func (h *AuthHandler) handleFreshUserSignup(
	w http.ResponseWriter, r *http.Request,
	req RegisterInit, user *domain.User,
) error {
	channel, target := detectChannel(req)

	var wg sync.WaitGroup
	wg.Add(2)

	var otpResp *otppb.GenerateOTPResponse
	var otpErr error
	var session *domain.Session
	var sessErr error

	go func() {
		defer wg.Done()
		otpResp, otpErr = h.otp.Client.GenerateOTP(
			r.Context(),
			&otppb.GenerateOTPRequest{
				UserId:    user.ID,
				Channel:   channel,
				Purpose:   "register",
				Recipient: target,
			},
		)
	}()

	go func() {
		defer wg.Done()
		extraData := map[string]string{"next": "verify_" + channel}
		session, sessErr = h.createSessionHelper(
			r.Context(),
			user.ID, true, false, "register",
			extraData, req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
		)
	}()

	wg.Wait()

	if otpErr != nil || otpResp == nil || !otpResp.Ok {
		return fmt.Errorf("OTP generation failed: %v", otpErr)
	}
	if sessErr != nil {
		return fmt.Errorf("session creation failed: %v", sessErr)
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":     "Signup initiated. Verify OTP to continue.",
		"next":        "verify-otp",
		"otp_channel": channel,
		"token":       session.AuthToken,
		"device":      session.DeviceID,
	})
	return nil
}

func detectChannel(req RegisterInit) (string, string) {
	if req.Phone != "" {
		return "sms", req.Phone
	}
	return "email", req.Email
}



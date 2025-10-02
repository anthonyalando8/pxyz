package handler

import (
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"x/shared/genproto/otppb"
	"x/shared/response"
	"x/shared/utils/errors"
)

// Step 1: Receive email and determine whether it's login or signup
// func (h *AuthHandler) HandleIdentify(w http.ResponseWriter, r *http.Request) {
//     var req struct {
//         Email string `json:"email"`
//     }

//     if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
//         response.Error(w, http.StatusBadRequest, "Invalid request body")
//         return
//     }

//     if req.Email == "" {
//         response.Error(w, http.StatusBadRequest, "Email is required")
//         return
//     }

//     if !utils.ValidateEmail(req.Email) {
//         response.Error(w, http.StatusBadRequest, "Invalid email format")
//         return
//     }

//     // Check if user exists
//     user, err := h.uc.FindUserByIdentifier(r.Context(), req.Email)
//     if err != nil && err != sql.ErrNoRows {
//         response.Error(w, http.StatusInternalServerError, "Database error")
//         return
//     }

//     action := "signup"
//     if user != nil {
//         action = "login"
//     }

//     response.JSON(w, http.StatusOK, map[string]interface{}{
//         "action": action, // "login" or "signup"
//         "email":  req.Email,
//     })
// }


func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("HandleRegister: failed to decode request body: %v", err)
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		log.Printf("HandleRegister: missing required fields, email: %q, password length: %d", req.Email, len(req.Password))
		response.Error(w, http.StatusBadRequest, "All fields (email, password) are required")
		return
	}

	if !utils.ValidateEmail(req.Email) {
		log.Printf("HandleRegister: invalid email format: %s", req.Email)
		response.Error(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	if valid, err := utils.ValidatePassword(req.Password); !valid {
		log.Printf("HandleRegister: weak password for email %s: %v", req.Email, err)
		response.Error(w, http.StatusBadRequest, "Weak password: "+err.Error())
		return
	}

	// Check if user already exists
	exists, err := h.uc.UserExists(r.Context(), req.Email)
	if err != nil {
		log.Printf("HandleRegister: error checking user existence for email %s: %v", req.Email, err)
		response.Error(w, http.StatusInternalServerError, "Error checking user existence")
		return
	}
	if exists {
		log.Printf("HandleRegister: user already exists: %s", req.Email)
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"message": "User account already exists",
			"email":   req.Email,
		})
		return
	}

	// Register new user
	user, err := h.uc.RegisterUser(r.Context(), req.Email, req.Password, req.FirstName, req.LastName, "any")
	if err != nil {
		log.Printf("HandleRegister: failed to register user %s: %v", req.Email, err)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("HandleRegister: user registered successfully: %s (ID: %s)", req.Email, user.ID)
	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"message": "User registered successfully",
		"user_id": user.ID,
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

	err := h.handleRoleUpgrade(r.Context(), user.ID,"any")
	if err != nil {
		log.Printf("Role upgrade failed for user %s: %v", user.ID, err)
		// Optional: still proceed even if role upgrade fails
	}

	// --- Create session first (synchronous) ---
	extraData := map[string]string{"next": "verify_" + channel}
	session, sessErr := h.createSessionHelper(
		r.Context(),
		user.ID,
		true,  // isNew
		false, // isRestricted
		"register",
		extraData,
		req.DeviceID,
		req.DeviceMetadata,
		req.GeoLocation,
		r,
	)
	if sessErr != nil {
		return fmt.Errorf("session creation failed: %v", sessErr)
	}

	// --- Generate OTP asynchronously (non-blocking) ---
	go func() {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(), // use background to avoid cancelling if HTTP context times out
			&otppb.GenerateOTPRequest{
				UserId:    user.ID,
				Channel:   channel,
				Purpose:   "register",
				Recipient: target,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[WARN] OTP generation failed for user %s on %s: %v", user.ID, channel, otpErr)
		}
	}()

	// --- Respond immediately with session token ---
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



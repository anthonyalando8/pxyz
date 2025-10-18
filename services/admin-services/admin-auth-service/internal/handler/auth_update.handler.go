package handler

import (
	"admin-auth-service/internal/domain"
	"admin-auth-service/pkg/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"x/shared/auth/middleware"

	//"x/shared/genproto/accountpb"
	"x/shared/genproto/corepb"
	"google.golang.org/protobuf/types/known/structpb"
	"x/shared/genproto/shared/notificationpb"
	"github.com/google/uuid"

	"x/shared/genproto/otppb"
	"x/shared/response"
)

// Reset password (via OTP/email link)
func (h *AuthHandler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.uc.UpdatePassword(r.Context(), userID, req.NewPassword, false, "", false); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.sendPasswordChangeNotification(userID, nil)
	// Delete old token in background
	h.logoutSessionBg(r.Context())

	response.JSON(w, http.StatusOK, map[string]string{"message": "Password reset successful"})
}

// Set password (signup flow)
func (h *AuthHandler) HandleSetPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	deviceID, ok2 := r.Context().Value(middleware.ContextDeviceID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	if !ok2 || deviceID == "" || deviceID == "unknown" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized device")
		return
	}

	var req SetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	user, err := h.uc.FindUserById(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	// Check if password already exists
	if user.PasswordHash != nil && *user.PasswordHash != "" {
		response.JSON(w, http.StatusConflict, map[string]interface{}{
			"message": "User has already set a password",
		})
		return
	}

	// --- Password
	if err := h.uc.UpdatePassword(r.Context(), userID, req.NewPassword, false, "", true); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}


	// --- Response: always include next as set_nationality ---
	resp := map[string]interface{}{
		"message": "Password set successfully",
		"next":    "set_nationality",
	}

	response.JSON(w, http.StatusOK, resp)
}


func (h *AuthHandler) HandleChangeEmail(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ChangeEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	req.UserID = requestedUserID
	if req.NewEmail == "" {
		response.Error(w, http.StatusBadRequest, "New email required")
		return
	}

	if valid := utils.ValidateEmail(req.NewEmail); !valid {
		response.Error(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	// --- Step 1: Save pending email with expiry (15 min) ---
	key := fmt.Sprintf("pending_email_change:%s", req.UserID)
	err := h.redisClient.Set(
		r.Context(),
		key,
		req.NewEmail,
		15*time.Minute,
	).Err()
	if err != nil {
		log.Printf("[HandleChangeEmail][Redis] Failed to save pending email for user=%s: %v", req.UserID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to process request")
		return
	}

	// --- Step 2: Generate OTP asynchronously ---
	go func(userID, newEmail string) {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(), // decoupled from request context
			&otppb.GenerateOTPRequest{
				UserId:    userID,
				Channel:   "email",
				Purpose:   "email_change",
				Recipient: newEmail,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[HandleChangeEmail][OTP] Failed to generate/send OTP userID=%s, email=%s, otpErr=%v, serviceErr=%v",
				userID, newEmail, otpErr, otpResp.GetError(),
			)
			return
		}
		log.Printf("[HandleChangeEmail][OTP] OTP generated and sent successfully userID=%s, email=%s", userID, newEmail)
	}(req.UserID, req.NewEmail)

	// --- Step 3: Respond immediately ---
	response.JSON(w, http.StatusOK, map[string]string{
		"message":     "OTP sent to new email. Verify to confirm change.",
		"next":        "verify-otp",
		"otp_purpose": "email_change",
	})
}




func (h *AuthHandler) HandleUpdateName(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req UpdateNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		response.Error(w, http.StatusBadRequest, "First name and last name are required")
		return
	}
	if len(req.FirstName) < 3 || len(req.LastName) < 3 {
		response.Error(w, http.StatusBadRequest, "First name and last name must be at least 2 characters long")
		return
	}
	req.UserID = requestedUserID
	err := h.uc.UpdateName(r.Context(), req.UserID, req.FirstName, req.LastName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update name: %v", err))
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"message": "Name updated successfully"})
}


func (h *AuthHandler) HandleRequestEmailChange(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	deviceID, ok2 := r.Context().Value(middleware.ContextDeviceID).(string)
	if (!ok || userID == "") || (!ok2 || deviceID == "") {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.uc.FindUserById(context.Background(), userID)
	if err != nil {
		log.Printf("[WARN] failed to fetch user %s for email change request: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "User lookup failed")
		return
	}
	masked := maskEmail(*user.Email)


	h.handleRequestChange(r.Context(), w, userID, *user.Email, "email", "request_email_change", masked)
}

func (h *AuthHandler) HandleRequestPhoneChange(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	deviceID, ok2 := r.Context().Value(middleware.ContextDeviceID).(string)
	if (!ok || userID == "") || (!ok2 || deviceID == "") {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// --- Fetch user ---
	user, err := h.uc.FindUserById(context.Background(), userID)
	if err != nil {
		log.Printf("[WARN] failed to fetch user %s for phone change request: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "User lookup failed")
		return
	}
	ctx := r.Context()

	// --- Ensure nationality first ---
	userNatKey := fmt.Sprintf("user_nationality:%s", userID)
	nationality, err := h.redisClient.Get(ctx, userNatKey).Result()
	if err != nil || nationality == "" {
		nextAction, nat := "",""
		if nextAction != "" {
			response.JSON(w, http.StatusOK, map[string]string{
				"message": "Please update your nationality to continue.",
				"next":    nextAction,
			})
			return
		}
		nationality = nat
		_ = h.redisClient.Set(ctx, userNatKey, nationality, 5*time.Minute).Err()
	}

	// --- Determine OTP target ---
	var channel, recipient, masked string

	if user.Phone != nil && *user.Phone != "" && user.IsPhoneVerified {
		channel = "sms"
		recipient = *user.Phone
		masked = maskPhone(recipient)
	} else if user.Email != nil && *user.Email != "" {
		channel = "email"
		recipient = *user.Email
		masked = maskEmail(recipient)
	} else {
		response.Error(w, http.StatusBadRequest, "No valid contact (phone/email) found for OTP")
		return
	}

	// --- Call OTP helper ---
	h.handleRequestChange(ctx, w, userID, recipient, channel, "request_phone_change", masked)
}



func (h *AuthHandler) HandleRequestPasswordChange(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	deviceID, ok2 := r.Context().Value(middleware.ContextDeviceID).(string)
	if (!ok || userID == "") || (!ok2 || deviceID == "") {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.uc.FindUserById(context.Background(), userID)
	if err != nil {
		log.Printf("[WARN] failed to fetch user %s for password change request: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "User lookup failed")
		return
	}

	if user.Email == nil || *user.Email == "" {
		log.Printf("[WARN] user %s has no email set, cannot send OTP", userID)
		response.Error(w, http.StatusBadRequest, "User email not set")
		return
	}
	masked := maskEmail(*user.Email)

	// --- Call helper with email channel ---
	h.handleRequestChange(r.Context(), w, userID, *user.Email, "email", "request_password_change", masked)
}


func (h *AuthHandler) handleRequestChange(
	_ context.Context,
	w http.ResponseWriter,
	userID string,
	recipient string,
	channel string,    // "email" or "sms"
	otpPurpose string,
	maskedTarget string,
) {
	if recipient == "" {
		response.Error(w, http.StatusBadRequest, fmt.Sprintf("User %s has no %s set", userID, channel))
		return
	}

	// Respond immediately (with masked target)
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":     fmt.Sprintf("OTP sent to %s. Verify to continue.", maskedTarget),
		"next":        "verify-otp",
		"otp_purpose": otpPurpose,
		"target":      maskedTarget,
		"channel":     channel,
	})

	// Send OTP asynchronously
	go func() {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(),
			&otppb.GenerateOTPRequest{
				UserId:    userID,
				Channel:   channel,
				Purpose:   otpPurpose,
				Recipient: recipient,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[handleRequestChange] ❌ OTP generation failed userID=%v, otpErr=%v, serviceErr=%v",
				userID, otpErr, otpResp.GetError())
			return
		}
		log.Printf("[handleRequestChange] ✅ OTP sent successfully to %s for userID=%v", recipient, userID)
	}()
}



func (h *AuthHandler) HandlePhoneChange(w http.ResponseWriter, r *http.Request) {
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || requestedUserID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req RequestPhoneChange
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.NewPhone == "" {
		response.Error(w, http.StatusBadRequest, "New phone number required")
		return
	}

	ctx := r.Context()

	// --- Fetch user nationality ---
	userNatKey := fmt.Sprintf("user_nationality:%s", requestedUserID)
	nationality, err := h.redisClient.Get(ctx, userNatKey).Result()
	if err != nil || nationality == "" {
		// not cached → call ensureNationality
		nextAction, nat := "",""
		if nextAction != "" {
			response.JSON(w, http.StatusOK, map[string]string{
				"message": "Please update your nationality to continue.",
				"next":    nextAction,
			})
			return
		}
		nationality = nat
		// cache nationality for 5 min
		if err := h.redisClient.Set(ctx, userNatKey, nationality, 5*time.Minute).Err(); err != nil {
			log.Printf("Failed to cache user nationality %s: %v", requestedUserID, err)
		}
	}

	// --- Get country info from cache or CoreService ---
	countryCacheKey := fmt.Sprintf("country_info:%s", nationality)
	countryInfo := map[string]interface{}{"nationality": nationality} // fallback
	cached, err := h.redisClient.Get(ctx, countryCacheKey).Result()
	if err == nil && cached != "" {
		if err := json.Unmarshal([]byte(cached), &countryInfo); err != nil {
			log.Printf("Failed to unmarshal cached country info for %s: %v", nationality, err)
		}
	} else {
		countryResp, err := h.coreClient.Client.GetCountry(ctx, &corepb.GetCountryRequest{Iso2: nationality})
		if err == nil && countryResp != nil && countryResp.Country != nil {
			c := countryResp.Country
			countryInfo = map[string]interface{}{
				"nationality":   nationality,
				"country_name":  c.Name,
				"phone_code":    c.PhoneCode,
				"currency_code": c.CurrencyCode,
				"currency_name": c.CurrencyName,
				"region":        c.Region,
				"subregion":     c.Subregion,
				"flag_url":      c.FlagUrl,
			}
			data, _ := json.Marshal(countryInfo)
			if err := h.redisClient.Set(ctx, countryCacheKey, data, 5*time.Minute).Err(); err != nil {
				log.Printf("Failed to cache country info for %s: %v", nationality, err)
			}
		} else {
			log.Printf("CoreService GetCountry failed for iso2=%s: %v", nationality, err)
		}
	}

	// --- Validate phone against country phone code ---
	phoneCode := ""
	if pc, ok := countryInfo["phone_code"].(string); ok {
		phoneCode = pc
	}
	if !utils.ValidatePhoneWithCountry(req.NewPhone, phoneCode) {
		response.Error(w, http.StatusBadRequest, fmt.Sprintf("Phone number must start with %s", phoneCode))
		return
	}

	// --- Cache new phone number for 15 minutes ---
	redisKey := fmt.Sprintf("phone_change:%s", requestedUserID)
	if err := h.redisClient.Set(ctx, redisKey, req.NewPhone, 15*time.Minute).Err(); err != nil {
		log.Printf("Failed to cache phone change for user %s: %v", requestedUserID, err)
		response.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// --- Respond to user immediately ---
	response.JSON(w, http.StatusOK, map[string]string{
		"message":     "OTP sent to your new phone. Please verify to continue.",
		"next":        "verify-otp",
		"otp_purpose": "phone_change",
	})

	// --- Send OTP asynchronously ---
	go func() {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(),
			&otppb.GenerateOTPRequest{
				UserId:    requestedUserID,
				Channel:   "sms",
				Purpose:   "phone_change",
				Recipient: req.NewPhone,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("OTP generation failed for phone change user %s: %v, serviceErr=%v",
				requestedUserID, otpErr, otpResp.GetError())
			return
		}
		log.Printf("OTP sent successfully to %s for user %s", req.NewPhone, requestedUserID)
	}()
}

// verify 2fa enabled
	// _2faRes, err := h.accountClient.Client.GetTwoFAStatus(r.Context(), &accountpb.GetTwoFAStatusRequest{
	// 	UserId: requestedUserID,
	// })
	// if err != nil{
	// 	response.Error(w, http.StatusInternalServerError, "Failed to check 2fa status")
	// 	return
	// }
	// if !_2faRes.IsEnabled{
	// 	response.Error(w, http.StatusUnauthorized, "2FA should be enabled to proceed")
	// 	return
	// }
	// var req RequestEmailChange
	// if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	// 	response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
	// 	return
	// }
	// resp, err := h.accountClient.Client.VerifyTwoFA(r.Context(), &accountpb.VerifyTwoFARequest{
	// 	UserId:     requestedUserID,
	// 	Code:       req.TOTP,
	// 	Method:     "totp",
	// })
	// if err != nil {
	// 	response.Error(w, http.StatusInternalServerError, err.Error())
	// 	return
	// }
	// if !resp.Success {
	// 	response.Error(w, http.StatusUnauthorized, "Verification failed. Invalid code.")
	// 	return
	// }


func (h *AuthHandler) HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.Identifier == "" {
		response.Error(w, http.StatusBadRequest, "Email or phone required")
		return
	}

	// Step 1: Try to find user
	user, err := h.uc.FindUserByIdentifier(r.Context(), req.Identifier)
	if err != nil {
		// Lookup failure → still respond generic but log
		log.Printf("[ForgotPassword] user lookup failed for identifier=%s, err=%v", req.Identifier, err)
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"message":     "If an account with the provided identifier exists, an OTP has been sent to its email address.",
			"next":        "verify-otp",
			"otp_purpose": "password_reset",
		})
		return
	}
	if user == nil {
		// User not found → respond generic
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"message":     "If an account with the provided identifier exists, an OTP has been sent to its email address.",
			"next":        "verify-otp",
			"otp_purpose": "password_reset",
		})
		return
	}

	// Step 2: Create temp session tied to this user
	session, sessErr := h.createSessionHelper(
		r.Context(),
		user.ID,
		true,   // temp session
		false,  // isRefresh
		"verify-otp", // purpose
		nil,
		req.DeviceID, req.DeviceMetadata, req.GeoLocation, r,
	)
	if sessErr != nil {
		log.Printf("[HandleForgotPassword] ❌ Session creation failed userID=%v, err=%v", user.ID, sessErr)
		response.Error(w, http.StatusInternalServerError, "Session creation failed: "+sessErr.Error())
		return
	}

	// Step 3: Respond to client
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":     "If an account with the provided identifier exists, an OTP has been sent to its email address.",
		"next":        "verify-otp",
		"otp_purpose": "password_reset",
		"token":       session.AuthToken,
		"device":      session.DeviceID,
	})

	// Step 4: Send OTP in background
	go func(u *domain.User) {
		if u.Email == nil || *u.Email == "" {
			log.Printf("[ForgotPassword] user %v has no email set, skipping OTP send", u.ID)
			return
		}

		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(),
			&otppb.GenerateOTPRequest{
				UserId:    u.ID,
				Channel:   "email",
				Purpose:   "password_reset",
				Recipient: *u.Email,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[ForgotPassword] ❌ OTP generation failed userID=%v, otpErr=%v, serviceErr=%v",
				u.ID, otpErr, otpResp.GetError(),
			)
			return
		}
		log.Printf("[ForgotPassword] ✅ OTP sent successfully to %s for userID=%v", *u.Email, u.ID)
	}(user)
}
func (h *AuthHandler) sendPasswordChangeNotification(userID string, deviceInfo map[string]string) {
	if h.notificationClient == nil {
		return
	}

	go func(uid string, device map[string]string) {
		ctx := context.Background() // background context for async processing

		// Build device details string if available
		deviceDetails := ""
		if len(device) > 0 {
			for k, v := range device {
				deviceDetails += fmt.Sprintf("<li><strong>%s:</strong> %s</li>", k, v)
			}
			deviceDetails = fmt.Sprintf("<ul>%s</ul>", deviceDetails)
		}

		payload := map[string]interface{}{
			"DeviceDetails": deviceDetails,
		}

		_, err := h.notificationClient.Client.CreateNotification(ctx, &notificationpb.CreateNotificationRequest{
			Notification: &notificationpb.Notification{
				RequestId:      uuid.New().String(),
				OwnerType:      "admin",
				OwnerId:        uid,
				EventType:      "PASSWORD_UPDATE",
				Title: "Password Changed",
				Body: "Your password was recently changed! If it was you take no action else consider securing your account.",
				ChannelHint:    []string{"email"},
				Payload: func() *structpb.Struct {
					s, _ := structpb.NewStruct(payload)
					return s
				}(),
				VisibleInApp:   false,
				Priority:       "high",
				Status:         "pending",
			},
		})
		if err != nil {
			log.Printf("[WARN] failed to send password change notification to %s: %v", uid, err)
		}
	}(userID, deviceInfo)
}


// --- Email verification request ---
func (h *AuthHandler) HandleRequestEmailVerification(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	user, err := h.uc.FindUserById(ctx, userID)
	if err != nil {
		log.Printf("[WARN] failed to fetch user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "User lookup failed")
		return
	}

	if user.Email == nil || *user.Email == "" {
		response.JSON(w, http.StatusBadRequest, map[string]interface{}{
			"message": "Email is not set",
		})
		return
	}

	if user.IsEmailVerified {
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"message": "Email is already verified",
		})
		return
	}


	// --- Generate OTP asynchronously ---
	go func() {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(),
			&otppb.GenerateOTPRequest{
				UserId:    user.ID,
				Channel:   "email",
				Purpose:   "verify_email",
				Recipient: *user.Email,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[WARN] OTP generation failed for user %s email: %v", user.ID, otpErr)
		}
	}()

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":    "OTP sent for email verification",
		"otp_channel": "email",
		"otp_purpose": "verify_email",
	})
}

// --- Phone verification request ---
func (h *AuthHandler) HandleRequestPhoneVerification(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx := r.Context()

	user, err := h.uc.FindUserById(ctx, userID)
	if err != nil {
		log.Printf("[WARN] failed to fetch user %s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "User lookup failed")
		return
	}

	if user.Phone == nil || *user.Phone == "" {
		response.JSON(w, http.StatusBadRequest, map[string]interface{}{
			"message": "Phone is not set",
		})
		return
	}

	if user.IsPhoneVerified {
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"message": "Phone is already verified",
		})
		return
	}


	// --- Generate OTP asynchronously ---
	go func() {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(),
			&otppb.GenerateOTPRequest{
				UserId:    user.ID,
				Channel:   "sms",
				Purpose:   "verify_phone",
				Recipient: *user.Phone,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[WARN] OTP generation failed for user %s phone: %v", user.ID, otpErr)
		}
	}()

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":    "OTP sent for phone verification",
		"otp_channel": "sms",
		"otp_purpose": "verify_phone",
	})
}

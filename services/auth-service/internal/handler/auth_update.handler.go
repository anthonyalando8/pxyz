package handler

import (
	"auth-service/internal/domain"
	"auth-service/pkg/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"x/shared/auth/middleware"

	//"x/shared/genproto/accountpb"
	"x/shared/genproto/emailpb"
	"x/shared/genproto/otppb"
	"x/shared/response"
)

// Change password (requires old + new)
func (h *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.uc.UpdatePassword(r.Context(), userID, req.NewPassword, true, req.OldPassword, false); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Password updated"})
}

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

	if err := h.uc.UpdatePassword(r.Context(), userID, req.NewPassword, false, "", true); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	session, sessErr := h.createSessionHelper(
		r.Context(),
		userID, false, false, "general",
		nil, &deviceID, nil, nil, r,
	)
	if sessErr != nil {
		log.Printf("Failed to create temp session: %v", sessErr)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	// --- Send Welcome Email in background ---
	if h.emailClient != nil {
		uid := userID
		go func() {
			subject := "Welcome to Pxyz 🎉"

			body := `
			<!DOCTYPE html>
			<html>
			<head><meta charset="UTF-8"><title>Welcome</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">Welcome to Pxyz</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						Your account has been successfully created and your password set. 
						We’re excited to have you onboard 🚀
					</p>
					<p style="font-size: 16px; color: #333;">
						You can now log in and start exploring the platform. 
						We’ve built Pxyz with your security and experience in mind.
					</p>
					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Team</strong>
					</p>
				</div>
			</body>
			</html>
			`

			user, err := h.uc.FindUserById(context.Background(), userID)
			if err != nil {
				log.Printf("[WARN] failed to fetch user %s for welcome email: %v", uid, err)
				return
			}
			if user == nil || user.Email == nil || *user.Email == "" {
				log.Printf("[WARN] user %s has no email set, cannot send welcome email", uid)
				return
			}
			// Send email

			_, emailErr := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
				UserId:         uid,
				RecipientEmail: *user.Email, // <-- needed to fetch email
				Subject:        subject,
				Body:           body,
				Type:           "welcome",
			})
			if emailErr != nil {
				log.Printf("[WARN] failed to send welcome email to user %s: %v", uid, emailErr)
			}
		}()
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Password set successfully",
		"token":   session.AuthToken,
	})
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

	// Step 1: Save pending email with expiry
	if err := h.uc.SetPendingEmail(r.Context(), req.UserID, req.NewEmail); err != nil {
		log.Printf("[HandleChangeEmail]  Failed to set pending email userID=%s, newEmail=%s, err=%v",
			req.UserID, req.NewEmail, err,
		)
		response.Error(w, http.StatusInternalServerError, "Email change processing failed")
		return
	}

	// Step 2: Generate OTP asynchronously
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
			log.Printf("[HandleChangeEmail][OTP]  Failed to generate/send OTP userID=%s, email=%s, otpErr=%v, serviceErr=%v",
				userID, newEmail, otpErr, otpResp.GetError(),
			)
			return
		}
		log.Printf("[HandleChangeEmail][OTP] OTP generated and sent successfully userID=%s, email=%s", userID, newEmail)
	}(req.UserID, req.NewEmail)

	// Step 3: Respond immediately
	response.JSON(w, http.StatusOK, map[string]string{
		"message":     "OTP sent to new email. Verify to confirm change.",
		"next":  "verify-otp",
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
	requestedUserID, ok := r.Context().Value(middleware.ContextUserID).(string)
	deviceID, ok2 := r.Context().Value(middleware.ContextDeviceID).(string)
	if (!ok || requestedUserID == "") || (!ok2 || deviceID == "") {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.uc.FindUserById(context.Background(), requestedUserID)
	if err != nil {
		log.Printf("[WARN] failed to fetch user %s for email change request: %v", requestedUserID, err)
		response.Error(w, http.StatusInternalServerError, "User lookup failed")
		return
	}
	if user == nil || user.Email == nil || *user.Email == "" {
		log.Printf("[WARN] user %s has no email set, cannot send OTP", requestedUserID)
		response.Error(w, http.StatusBadRequest, "User email not set")
		return
	}

	// Respond first
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Request processed successfully. Verify OTP sent to old email to confirm action.",
		"next":   "verify-otp",
		"otp_purpose":  "request_email_change",
	})

	// Send OTP in background
	go func() {
		otpResp, otpErr := h.otp.Client.GenerateOTP(
			context.Background(),
			&otppb.GenerateOTPRequest{
				UserId:    requestedUserID,
				Channel:   "email",
				Purpose:   "request_email_change",
				Recipient: *user.Email,
			},
		)
		if otpErr != nil || otpResp == nil || !otpResp.Ok {
			log.Printf("[HandleRequestEmailChange] ❌ OTP generation failed userID=%v, otpErr=%v, serviceErr=%v",
				user.ID, otpErr, otpResp.GetError(),
			)
			return
		}
		log.Printf("[HandleRequestEmailChange] OTP sent successfully to %s for userID=%v", *user.Email, user.ID)
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
	if valid := utils.ValidatePhone(req.NewPhone); !valid {
		response.Error(w, http.StatusBadRequest, "Invalid phone number format")
		return
	}
	err := h.uc.UpdatePhone(r.Context(), requestedUserID, req.NewPhone)
	if err != nil {
		log.Printf("[HandleRequestPhoneChange]  Failed to update phone userID=%s, newPhone=%s, err=%v",
			requestedUserID, req.NewPhone, err,
		)
		response.Error(w, http.StatusInternalServerError, "Phone update processing failed")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Phone number updated successfully",
	})
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


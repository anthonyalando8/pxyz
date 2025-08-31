package handler

import (
	"auth-service/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"x/shared/auth/middleware"
	"x/shared/genproto/emailpb"
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
		"whatsapp": true,
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
	// ---------- Step 1: Decode request ----------
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
	if req.Purpose == "" {
		response.Error(w, http.StatusBadRequest, "Purpose is required")
		return
	}
	// ---------- Step 2: Extract user context ----------
	userId, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userId == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	deviceID, _ := r.Context().Value(middleware.ContextDeviceID).(string)

	// ---------- Step 3: Validate OTP ----------
	if !h.VerifyOtpHelper(w, r.Context(), userId, req.OtpCode, req.Purpose) {
		return
	}

	// ---------- Step 4: Base response ----------
	resp := map[string]interface{}{
		"message": "OTP verified successfully",
	}

	// ---------- Step 5: Handle extra data ----------
	extra, _ := r.Context().Value(middleware.ContextExtraData).(map[string]string)

	// Decide what to use as "next"
	var next string
	if extra != nil {
		next = extra["next"]
	}
	if next == "" {
		next = req.Purpose // fallback to purpose
	}

	if next != "" {
		if err := h.handleNextAction(r, w, userId, deviceID, next, extra, resp); err != nil {
			// errors already written inside helper
			return
		}
	}

	// ---------- Step 6: Send response ----------
	response.JSON(w, http.StatusOK, resp)
}

// handleNextAction processes extra["next"] values and mutates resp accordingly.
func (h *AuthHandler) handleNextAction(
	r *http.Request,
	w http.ResponseWriter,
	userId string,
	deviceID string,
	next string,
	extra map[string]string,
	resp map[string]interface{},
) error {
	ctx := r.Context()
	log.Printf("[handleNextAction] ▶️ userId=%s deviceID=%s next=%s extra=%v", userId, deviceID, next, extra)

	// --- Small helper closures ---
	createSession := func(purpose string, temp bool) (*domain.Session, error) {
		return h.createSessionHelper(
			ctx,
			userId,
			temp,
			false, // no single use
			purpose,
			nil,
			&deviceID,
			nil, nil, r,
		)
	}

	verify := func(fn func(context.Context, string) (bool, error), label string) error {
		if ok, err := fn(ctx, userId); err != nil || !ok {
			log.Printf("[handleNextAction] ❌ %s verification failed user=%s err=%v ok=%v", label, userId, err, ok)
			response.Error(w, http.StatusInternalServerError, label+" verification failed")
			return err
		}
		log.Printf("[handleNextAction] ✅ %s verified for user=%s", label, userId)
		resp["action"] = "verify_" + strings.ToLower(label)
		return nil
	}

	// --- Main flow ---
	switch next {
	case "verify_email":
		return verify(h.uc.VerifyEmail, "Email")

	case "verify_phone":
		return verify(h.uc.VerifyPhone, "Phone")

	case "incomplete_profile":
		if err := verify(h.uc.VerifyEmail, "Email"); err != nil {
			return err
		}
		session, err := createSession("register", true)
		if err != nil {
			log.Printf("[handleNextAction] ❌ Failed to create register session user=%s err=%v", userId, err)
			response.Error(w, http.StatusInternalServerError, "Session creation failed")
			return err
		}
		resp["action"] = "incomplete_profile_verified"
		resp["stage"] = extra["stage"]
		resp["next"] = extra["next_stage"]
		resp["token"] = session.AuthToken
		resp["device"] = session.DeviceID

	case "request_email_change":
		session, err := createSession("email_change", true)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to process request")
			return err
		}
		resp["message"] = "Request processed successfully"
		resp["next"] = "send_new_email"
		resp["token"] = session.AuthToken
		resp["device"] = session.DeviceID

	case "email_change":
		user, err := h.uc.FindUserById(ctx, userId)
		oldEmail := ""
		if err == nil && user != nil && user.Email != nil {
			oldEmail = *user.Email
		}
		pendingEmail, err := h.uc.GetPendingEmail(ctx, userId)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to retrieve pending email")
			return err
		}
		if pendingEmail == "" {
			response.Error(w, http.StatusBadRequest, "No pending email change found")
			return errors.New("no pending email change")
		}
		if err := h.uc.ChangeEmail(ctx, userId, pendingEmail); err != nil {
			response.Error(w, http.StatusInternalServerError, "Email change failed")
			return err
		}
		resp["action"] = "new_email_verified"
		resp["new_email"] = pendingEmail
		resp["message"] = "Email changed successfully"
		h.sendEmailChangeNotifications(ctx, userId, oldEmail, pendingEmail)

	case "password_reset":
		session, err := createSession("password_reset", true)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to process request")
			return err
		}
		resp["message"] = "Request processed successfully"
		resp["next"] = "reset_password"
		resp["token"] = session.AuthToken
		resp["device"] = session.DeviceID
	}

	log.Printf("[handleNextAction] ⏹️ Completed next=%s for user=%s", next, userId)
	return nil
}




func (h *AuthHandler) sendEmailChangeNotifications(_ context.Context, userID, oldEmail, newEmail string) {
	if h.emailClient == nil {
		return
	}

	// Send to new email
	if newEmail != "" {
		go func(uid, recipient string) {
			subject := "Your Pxyz account email has been updated"
			body := fmt.Sprintf(`
				<!DOCTYPE html>
				<html><head><meta charset="UTF-8"><title>Email Updated</title></head>
				<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
					<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
						<h2 style="color: #2E86C1;">Primary Email Updated</h2>
						<p style="font-size: 16px; color: #333;">
							Hello,<br><br>
							Your primary email has been successfully changed to <strong>%s</strong>. 
							This email will now be used to log in and receive communication from Pxyz.
						</p>
						<p style="margin-top: 30px; font-size: 14px; color: #999999;">
							Thank you,<br>
							<strong>Pxyz Team</strong>
						</p>
					</div>
				</body>
				</html>`, newEmail)

			_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
				UserId:         uid,
				RecipientEmail: recipient,
				Subject:        subject,
				Body:           body,
				Type:           "email_update_new",
			})
			if err != nil {
				log.Printf("[WARN] failed to send new email notification to %s: %v", recipient, err)
			}
		}(userID, newEmail)
	}

	// Send to old email
	if oldEmail != "" {
		go func(uid, recipient string) {
			subject := "Your Pxyz account email has been changed"
			body := fmt.Sprintf(`
				<!DOCTYPE html>
				<html><head><meta charset="UTF-8"><title>Email Changed</title></head>
				<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
					<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
						<h2 style="color: #C0392B;">Primary Email Changed</h2>
						<p style="font-size: 16px; color: #333;">
							Hello,<br><br>
							Your account’s primary email was changed from this address to <strong>%s</strong>. 
							That new email will now be required for login, feature access, and system communication.
						</p>
						<p style="margin-top: 30px; font-size: 14px; color: #999999;">
							Thank you,<br>
							<strong>Pxyz Team</strong>
						</p>
					</div>
				</body>
				</html>`, newEmail)

			_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
				UserId:         uid,
				RecipientEmail: recipient,
				Subject:        subject,
				Body:           body,
				Type:           "email_update_old",
			})
			if err != nil {
				log.Printf("[WARN] failed to send old email notification to %s: %v", recipient, err)
			}
		}(userID, oldEmail)
	}
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


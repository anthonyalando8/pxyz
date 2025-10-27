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
	"time"
	"x/shared/auth/middleware"
	"x/shared/genproto/corepb"
	"x/shared/genproto/otppb"
	"x/shared/genproto/shared/notificationpb"
	"x/shared/response"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/structpb"
)

func (h *AuthHandler) HandleRequestOTP(w http.ResponseWriter, r *http.Request) {
	var req RequestOTP
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate purpose
	if !isAllowedPurpose(req.Purpose) {
		response.Error(w, http.StatusBadRequest, "Invalid or unsupported OTP purpose")
		return
	}

	// Extract user ID
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Fetch user
	user, err := h.uc.GetUserByID(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	// Determine channel & recipient
	channel, recipient, err := resolveChannelAndRecipient(req, user)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call OTP service
	resp, err := h.otp.Client.GenerateOTP(
		r.Context(),
		&otppb.GenerateOTPRequest{
			UserId:    user.ID,
			Channel:   channel,
			Purpose:   req.Purpose,
			Recipient: recipient,
		},
	)
	if err != nil || !resp.Ok {
		log.Printf("Failed to generate OTP for user %s: %v", user.ID, err)
		msg := "Failed to generate OTP"
		if resp != nil && !resp.Ok {
			msg = resp.Error
		}
		response.Error(w, http.StatusInternalServerError, msg)
		return
	}

	// Respond with masked target
	masked := maskRecipient(channel, recipient)
	response.JSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("OTP sent to %s via %s", masked, channel),
		"channel": channel,
		"purpose": req.Purpose,
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

	verify := func(fn func(context.Context, string) (error), label string) error {
		if err := fn(ctx, userId); err != nil{
			log.Printf("[handleNextAction] ❌ %s verification failed user=%s err=%v", label, userId, err)
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
		// Delete old token in background
		h.logoutSessionBg(ctx)

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
		user, err := h.uc.GetUserByID(ctx, userId)
		oldEmail := ""
		if err == nil && user != nil && user.Email != nil {
			oldEmail = *user.Email
		}
		key := fmt.Sprintf("pending_email_change:%s", userId)
		pendingEmail, err := h.redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			response.Error(w, http.StatusBadRequest, "No pending email change found")
			return errors.New("no pending email change")
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to retrieve pending email")
			return err
		}
		if err := h.uc.ChangeEmail(ctx, userId, pendingEmail); err != nil {
			response.Error(w, http.StatusInternalServerError, "Email change failed")
			return err
		}
		resp["action"] = "new_email_verified"
		resp["new_email"] = pendingEmail
		resp["message"] = "Email changed successfully"
		h.sendEmailChangeNotifications(ctx, userId, oldEmail, pendingEmail)
		// Delete old token in background
		h.logoutSessionBg(ctx)

	case "password_reset","request_password_change":
		session, err := createSession("password_reset", true)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to process request")
			return err
		}
		resp["message"] = "Request processed successfully"
		resp["next"] = "reset_password"
		resp["token"] = session.AuthToken
		resp["device"] = session.DeviceID

		if next == "password_reset"{
			// Delete old token in background
			h.logoutSessionBg(ctx)
		}
			

	case "request_phone_change":
		// --- Ensure nationality exists ---
		nextAction, nationality := h.ensureNationality(ctx, userId)
		if nextAction != "" {
			resp["message"] = "Please update your nationality to continue."
			resp["next"] = nextAction
			response.JSON(w, http.StatusOK, resp)
			return errors.New("update nationality")
		}

		// --- Create temporary session for phone change ---
		session, err := createSession("phone_change", true)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to process request")
			return err
		}

		// --- Cache user nationality for 5 minutes ---
		userNatKey := fmt.Sprintf("user_nationality:%s", userId)
		if err := h.redisClient.Set(ctx, userNatKey, nationality, 5*time.Minute).Err(); err != nil {
			log.Printf("[request_phone_change] failed to cache user nationality for %s: %v", userId, err)
		}

		// --- Attempt to get country info from Redis cache ---
		countryCacheKey := fmt.Sprintf("country_info:%s", nationality)
		phoneInfo := map[string]interface{}{"nationality": nationality} // fallback
		cached, err := h.redisClient.Get(ctx, countryCacheKey).Result()
		if err == nil && cached != "" {
			if err := json.Unmarshal([]byte(cached), &phoneInfo); err != nil {
				log.Printf("[request_phone_change] failed to unmarshal cached country info for %s: %v", nationality, err)
			}
		} else {
			// --- Fetch from CoreService if not cached ---
			countryResp, err := h.coreClient.Client.GetCountry(ctx, &corepb.GetCountryRequest{
				Iso2: nationality,
			})
			if err == nil && countryResp != nil && countryResp.Country != nil {
				c := countryResp.Country
				phoneInfo = map[string]interface{}{
					"nationality":   nationality,
					"country_name":  c.Name,
					"phone_code":    c.PhoneCode,
					"currency_code": c.CurrencyCode,
					"currency_name": c.CurrencyName,
					"region":        c.Region,
					"subregion":     c.Subregion,
					"flag_url":      c.FlagUrl,
				}

				// --- Cache country info for 5 minutes ---
				data, _ := json.Marshal(phoneInfo)
				if err := h.redisClient.Set(ctx, countryCacheKey, data, 5*time.Minute).Err(); err != nil {
					log.Printf("[request_phone_change] failed to cache country info for %s: %v", nationality, err)
				}
			} else {
				log.Printf("[request_phone_change] CoreService GetCountry failed for iso2=%s: %v", nationality, err)
			}
		}

		// --- Build response ---
		resp["message"] = "Request processed successfully"
		resp["next"] = "send_new_phone"
		resp["token"] = session.AuthToken
		resp["device"] = session.DeviceID
		resp["phone_info"] = phoneInfo


	case "phone_change":
		// --- Retrieve cached phone number ---
		redisKey := fmt.Sprintf("phone_change:%s", userId)
		cachedPhone, err := h.redisClient.Get(ctx, redisKey).Result()
		if err == redis.Nil || cachedPhone == "" {
			log.Printf("[handleNextAction] ❌ No cached phone found for user=%s", userId)
			response.Error(w, http.StatusBadRequest, "No pending phone change request found")
			return fmt.Errorf("no cached phone")
		} else if err != nil {
			log.Printf("[handleNextAction] ❌ Failed to get cached phone for user=%s: %v", userId, err)
			response.Error(w, http.StatusInternalServerError, "Internal server error")
			return err
		}

		// --- Update phone in database ---
		phone := cachedPhone
		if after, ok :=strings.CutPrefix(phone, "+"); ok  {
			phone = after
		}

		if err := h.uc.UpdatePhone(ctx, userId, phone, true); err != nil {
			log.Printf("[handleNextAction] ❌ Failed to update phone for user=%s, newPhone=%s, err=%v",
				userId, phone, err,
			)
			response.Error(w, http.StatusInternalServerError, "Phone update processing failed")
			return err
		}

		h.sendPhoneChangeNotification(userId, cachedPhone)
		// Delete old token in background
		h.logoutSessionBg(ctx)
		// --- Delete cached phone ---
		if err := h.redisClient.Del(ctx, redisKey).Err(); err != nil {
			log.Printf("[handleNextAction] ⚠️ Failed to delete cached phone for user=%s: %v", userId, err)
		}
		// --- Respond ---
		resp["message"] = "Phone number updated successfully"

	}
	

	log.Printf("[handleNextAction] ⏹️ Completed next=%s for user=%s", next, userId)
	return nil
}


func (h *AuthHandler) sendEmailChangeNotifications(_ context.Context, userID, oldEmail, newEmail string) {
	if h.notificationClient == nil {
		return
	}

	send := func(recipientEmail string, eventType string, payload map[string]interface{}) {
		go func() {
			ctx := context.Background() // background context for long-running email service

			_, err := h.notificationClient.Client.CreateNotification(ctx, &notificationpb.CreateNotificationsRequest{
				Notifications: []*notificationpb.Notification{
					{
						RequestId:      uuid.New().String(),
						OwnerType:      "user",
						OwnerId:        userID,
						EventType:      eventType,
						Title: "Email Changed",
						Body: "Your email was recently updated to a new email!",
						ChannelHint:    []string{"email"},
						Payload: func() *structpb.Struct {
							s, _ := structpb.NewStruct(payload)
							return s
						}(),
						VisibleInApp:   false,
						RecipientEmail: recipientEmail,
						RecipientName:  "",
						Priority:       "high",
						Status:         "pending",
					},
				},
			})
			if err != nil {
				log.Printf("[WARN] failed to send %s notification to %s: %v", eventType, recipientEmail, err)
			}
		}()
	}

	// Send to new email
	if newEmail != "" {
		send(newEmail, "EMAIL_UPDATE_NEW", map[string]interface{}{
			"NewEmail": newEmail,
		})
	}

	// Send to old email
	if oldEmail != "" {
		send(oldEmail, "EMAIL_UPDATE_OLD", map[string]interface{}{
			"NewEmail": newEmail,
		})
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


func (h *AuthHandler) sendPhoneChangeNotification(userID, newPhone string) {
	if h.notificationClient == nil || newPhone == "" {
		return
	}

	go func(uid, phone string) {
		ctx := context.Background() // background context for async processing

		// Fetch user details
		user, err := h.uc.GetUserByID(ctx, uid)
		if err != nil {
			log.Printf("[WARN] failed to fetch user %s for phone change notification: %v", uid, err)
			return
		}

		if user.Email == nil || *user.Email == "" {
			log.Printf("[WARN] user %s has no email, skipping phone change notification", uid)
			return
		}

		payload := map[string]interface{}{
			"NewPhone": phone,
		}

		_, err = h.notificationClient.Client.CreateNotification(ctx, &notificationpb.CreateNotificationsRequest{
			Notifications: []*notificationpb.Notification{
				{
					RequestId:      uuid.New().String(),
					OwnerType:      "user",
					OwnerId:        uid,
					EventType:      "PHONE_UPDATE",
					ChannelHint:    []string{"email"},
					Title: "Phone Number Changed",
					Body:"Your phone number was recently updated to a new number.",
					Payload: func() *structpb.Struct {
						s, _ := structpb.NewStruct(payload)
						return s
					}(),
					VisibleInApp:   false,
					RecipientEmail: *user.Email,
					//RecipientName:  safeString(user.FirstName),
					Priority:       "high",
					Status:         "pending",
				},
			},
		})
		if err != nil {
			log.Printf("[WARN] failed to send phone change notification to %s: %v", *user.Email, err)
		}
	}(userID, newPhone)
}



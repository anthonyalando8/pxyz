// handler/verification.go
package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"x/shared/genproto/accountpb"
	"x/shared/genproto/otppb"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// Cache prefixes
	VerificationTokenPrefix = "verify:token:"
	OTPVerificationPrefix   = "verify:otp:"

	// Expiration times
	VerificationTokenTTL = 5 * time.Minute
	OTPSessionTTL        = 3 * time.Minute
)

// VerificationMethod types
const (
	VerificationMethodTOTP        = "totp"
	VerificationMethodOTPEmail    = "otp_email"
	VerificationMethodOTPSMS      = "otp_sms"
	VerificationMethodOTPWhatsApp = "otp_whatsapp"
	VerificationMethodAuto        = "auto" //  NEW - auto-select based on user profile
)

// Verification purposes
const (
	VerificationPurposeWithdrawal = "withdrawal"
	VerificationPurposeTransfer   = "transfer"
	VerificationPurposeSensitive  = "sensitive_operation"
)

// handleVerificationRequest initiates verification process
func (h *PaymentHandler) handleVerificationRequest(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		Method  string `json:"method,omitempty"` // totp, otp_email, otp_sms, otp_whatsapp, auto (or empty)
		Purpose string `json:"purpose"`          // withdrawal, transfer, etc.
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	// Validate purpose
	if req.Purpose == "" {
		client.SendError("purpose is required")
		return
	}

	userID := client.UserID

	//  If no method provided or "auto", determine best available method
	if req.Method == "" || req.Method == VerificationMethodAuto {
		h.logger.Info("auto-selecting verification method",
			zap.String("user_id", userID),
			zap.String("purpose", req.Purpose))

		h.handleAutoVerificationRequest(ctx, client, userID, req.Purpose)
		return
	}

	// Validate method
	if !isValidVerificationMethod(req.Method) {
		client.SendError(fmt.Sprintf("invalid verification method: %s", req.Method))
		return
	}

	// Handle based on method
	switch req.Method {
	case VerificationMethodTOTP:
		h.handleTOTPVerificationRequest(ctx, client, userID, req.Purpose)

	case VerificationMethodOTPEmail, VerificationMethodOTPSMS, VerificationMethodOTPWhatsApp:
		h.handleOTPVerificationRequest(ctx, client, userID, req.Method, req.Purpose)

	default:
		client.SendError("unsupported verification method")
	}
}

// handleAutoVerificationRequest auto-selects best verification method
func (h *PaymentHandler) handleAutoVerificationRequest(ctx context.Context, client *Client, userID, purpose string) {
	h.logger.Info("determining best verification method",
		zap.String("user_id", userID),
		zap.String("purpose", purpose))

	// Priority 1: Check if TOTP/2FA is enabled
	statusResp, err := h.accountClient.Client.GetTwoFAStatus(ctx, &accountpb.GetTwoFAStatusRequest{
		UserId: userID,
	})

	if err == nil && statusResp.IsEnabled {
		h.logger.Info("2FA enabled, using TOTP",
			zap.String("user_id", userID))
		h.handleTOTPVerificationRequest(ctx, client, userID, purpose)
		return
	}

	// Priority 2: Get user profile to check available methods
	user, err := h.profileFetcher.FetchProfile(ctx, "user", userID)
	if err != nil {
		h.logger.Error("failed to fetch user profile for auto-verification",
			zap.String("user_id", userID),
			zap.Error(err))
		client.SendError("failed to determine verification method")
		return
	}

	// Priority 3: Try SMS (most common for financial transactions)
	if user.Phone != "" {
		h.logger.Info("auto-selected SMS verification",
			zap.String("user_id", userID),
			zap.String("phone", maskPhone(user.Phone)))
		h.handleOTPVerificationRequest(ctx, client, userID, VerificationMethodOTPSMS, purpose)
		return
	}

	// Priority 4: Fall back to email
	if user.Email != "" {
		h.logger.Info("auto-selected email verification",
			zap.String("user_id", userID),
			zap.String("email", maskEmail(user.Email)))
		h.handleOTPVerificationRequest(ctx, client, userID, VerificationMethodOTPEmail, purpose)
		return
	}

	// No verification method available
	h.logger.Warn("no verification method available",
		zap.String("user_id", userID))
	client.SendError("no verification method available.  Please add a phone number or email to your profile")
}

// handleTOTPVerificationRequest handles TOTP (2FA) verification request
func (h *PaymentHandler) handleTOTPVerificationRequest(ctx context.Context, client *Client, userID, purpose string) {
	h.logger.Info("processing TOTP verification request",
		zap.String("user_id", userID),
		zap.String("purpose", purpose))

	// Check if 2FA is enabled
	statusResp, err := h.accountClient.Client.GetTwoFAStatus(ctx, &accountpb.GetTwoFAStatusRequest{
		UserId: userID,
	})
	if err != nil {
		h.logger.Error("failed to check 2FA status",
			zap.String("user_id", userID),
			zap.Error(err))
		client.SendError("failed to check 2FA status")
		return
	}

	if !statusResp.IsEnabled {
		h.logger.Warn("2FA not enabled, suggesting OTP",
			zap.String("user_id", userID))
		client.SendError("2FA is not enabled for your account.  Please use OTP verification instead or enable 2FA")
		return
	}

	h.logger.Info("2FA enabled, awaiting TOTP code",
		zap.String("user_id", userID))

	// Send response indicating TOTP code is required
	client.SendSuccess("2FA enabled. Please provide your TOTP code", map[string]interface{}{
		"method":    VerificationMethodTOTP,
		"purpose":   purpose,
		"next_step": "verify_totp",
	})
}

// handleOTPVerificationRequest handles OTP verification request
func (h *PaymentHandler) handleOTPVerificationRequest(ctx context.Context, client *Client, userID, method, purpose string) {
	h.logger.Info("processing OTP verification request",
		zap.String("user_id", userID),
		zap.String("method", method),
		zap.String("purpose", purpose))

	// Get user details
	user, err := h.profileFetcher.FetchProfile(ctx, "user", userID)
	if err != nil {
		h.logger.Error("failed to fetch user profile",
			zap.String("user_id", userID),
			zap.Error(err))
		client.SendError("user not found")
		return
	}

	// Determine channel and recipient
	var channel, recipient string
	switch method {
	case VerificationMethodOTPEmail:
		channel = "email"
		recipient = user.Email
		if recipient == "" {
			h.logger.Warn("email not configured for user",
				zap.String("user_id", userID))
			client.SendError("email not configured.  Please add an email to your profile")
			return
		}

	case VerificationMethodOTPSMS:
		channel = "sms"
		recipient = user.Phone
		if recipient == "" {
			h.logger.Warn("phone not configured for user",
				zap.String("user_id", userID))
			client.SendError("phone number not configured. Please add a phone number to your profile")
			return
		}

	case VerificationMethodOTPWhatsApp:
		channel = "whatsapp"
		recipient = user.Phone
		if recipient == "" {
			h.logger.Warn("phone not configured for WhatsApp",
				zap.String("user_id", userID))
			client.SendError("phone number not configured for WhatsApp. Please add a phone number to your profile")
			return
		}
	}

	h.logger.Info("generating OTP",
		zap.String("user_id", userID),
		zap.String("channel", channel),
		zap.String("recipient", maskRecipient(channel, recipient)))

	// Generate OTP
	otpResp, err := h.otp.Client.GenerateOTP(ctx, &otppb.GenerateOTPRequest{
		UserId:    userID,
		Channel:   channel,
		Purpose:   purpose,
		Recipient: recipient,
	})
	if err != nil || !otpResp.Ok {
		errMsg := "failed to generate OTP"
		if otpResp != nil && otpResp.Error != "" {
			errMsg = otpResp.Error
		}
		h.logger.Error("OTP generation failed",
			zap.String("user_id", userID),
			zap.String("channel", channel),
			zap.String("error", errMsg))
		client.SendError(errMsg)
		return
	}

	h.logger.Info("OTP generated successfully",
		zap.String("user_id", userID),
		zap.String("channel", channel))

	// Store OTP session in cache
	sessionKey := fmt.Sprintf("%s%s:%s", OTPVerificationPrefix, userID, purpose)
	if err := h.cacheOTPSession(ctx, sessionKey, method, channel); err != nil {
		h.logger.Error("failed to cache OTP session",
			zap.String("user_id", userID),
			zap.Error(err))
		client.SendError("failed to create verification session")
		return
	}

	// Send success response
	masked := maskRecipient(channel, recipient)

	h.logger.Info("OTP sent successfully",
		zap.String("user_id", userID),
		zap.String("channel", channel),
		zap.String("masked_recipient", masked))

	client.SendSuccess(fmt.Sprintf("OTP sent to %s via %s", masked, channel), map[string]interface{}{
		"method":     method,
		"channel":    channel,
		"recipient":  masked,
		"purpose":    purpose,
		"next_step":  "verify_otp",
		"expires_in": int(OTPSessionTTL.Seconds()),
	})
}

// cacheOTPSession stores OTP session in Redis
func (h *PaymentHandler) cacheOTPSession(ctx context.Context, sessionKey, method, channel string) error {
	sessionData := map[string]interface{}{
		"method":     method,
		"channel":    channel,
		"created_at": time.Now().Unix(),
	}

	data, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}

	return h.rdb.Set(ctx, sessionKey, data, OTPSessionTTL).Err()
}

// isValidVerificationMethod validates verification method
func isValidVerificationMethod(method string) bool {
	switch method {
	case VerificationMethodTOTP,
		VerificationMethodOTPEmail,
		VerificationMethodOTPSMS,
		VerificationMethodOTPWhatsApp,
		VerificationMethodAuto:
		return true
	}
	return false
}

// maskRecipient masks recipient for logging
func maskRecipient(channel, recipient string) string {
	if recipient == "" {
		return "***"
	}

	switch channel {
	case "email":
		return maskEmail(recipient)
	case "sms", "whatsapp":
		return maskPhone(recipient)
	default:
		return "***"
	}
}

// maskEmail masks email address
func maskEmail(email string) string {
	if email == "" {
		return "***@***"
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}

	local := parts[0]
	domain := parts[1]

	if len(local) <= 2 {
		return "**@" + domain
	}

	return local[:2] + "***@" + domain
}

// maskPhone masks phone number
func maskPhone(phone string) string {
	if phone == "" {
		return "***"
	}

	if len(phone) <= 4 {
		return "***"
	}

	return "***" + phone[len(phone)-4:]
}

// handleVerifyTOTP verifies TOTP code and generates verification token
func (h *PaymentHandler) handleVerifyTOTP(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		Code    string `json:"code"`
		Purpose string `json:"purpose"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	if req.Code == "" {
		client.SendError("TOTP code is required")
		return
	}

	if req.Purpose == "" {
		client.SendError("purpose is required")
		return
	}

	userID := client.UserID

	// Verify TOTP
	verifyResp, err := h.accountClient.Client.VerifyTwoFA(ctx, &accountpb.VerifyTwoFARequest{
		UserId: userID,
		Code:   req.Code,
		Method: "totp",
	})
	if err != nil {
		client.SendError("verification failed: " + err.Error())
		return
	}

	if !verifyResp.Success {
		client.SendError("invalid TOTP code")
		return
	}

	// Generate verification token
	token, err := h.generateVerificationToken(ctx, userID, req.Purpose, VerificationMethodTOTP)
	if err != nil {
		client.SendError("failed to generate verification token")
		return
	}

	// Send success response with token
	client.SendSuccess("verification successful", map[string]interface{}{
		"verification_token": token,
		"purpose":            req.Purpose,
		"method":             VerificationMethodTOTP,
		"expires_in":         int(VerificationTokenTTL.Seconds()),
		"message":            "Use this token for your next withdrawal request",
	})
}

// handleVerifyOTP verifies OTP code and generates verification token
func (h *PaymentHandler) handleVerifyOTP(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		Code    string `json:"code"`
		Purpose string `json:"purpose"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	if req.Code == "" {
		client.SendError("OTP code is required")
		return
	}

	if req.Purpose == "" {
		client.SendError("purpose is required")
		return
	}

	userID := client.UserID

	// Check OTP session exists
	sessionKey := fmt.Sprintf("%s%s:%s", OTPVerificationPrefix, userID, req.Purpose)
	exists, err := h.checkOTPSession(ctx, sessionKey)
	if err != nil || !exists {
		client.SendError("no active OTP session found.  Please request a new OTP.")
		return
	}

	// Verify OTP
	userIDInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		client.SendError("invalid user ID format")
		return
	}
	verifyResp, err := h.otp.Client.VerifyOTP(ctx, &otppb.VerifyOTPRequest{
		UserId:  userIDInt,
		Code:    req.Code,
		Purpose: req.Purpose,
	})
	if err != nil || !verifyResp.Valid {
		errMsg := "invalid OTP code"
		if verifyResp != nil && verifyResp.Error != "" {
			errMsg = verifyResp.Error
		}
		client.SendError(errMsg)
		return
	}

	// Get session details
	method, _ := h.getOTPSessionMethod(ctx, sessionKey)

	// Delete OTP session
	h.deleteOTPSession(ctx, sessionKey)

	// Generate verification token
	token, err := h.generateVerificationToken(ctx, userID, req.Purpose, method)
	if err != nil {
		client.SendError("failed to generate verification token")
		return
	}

	// Send success response with token
	client.SendSuccess("verification successful", map[string]interface{}{
		"verification_token": token,
		"purpose":            req.Purpose,
		"method":             method,
		"expires_in":         int(VerificationTokenTTL.Seconds()),
		"message":            "Use this token for your next withdrawal request",
	})
}

// generateVerificationToken creates a random token and caches it
func (h *PaymentHandler) generateVerificationToken(ctx context.Context, userID, purpose, method string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	// Store in Redis with metadata
	tokenKey := fmt.Sprintf("%s%s", VerificationTokenPrefix, token)
	tokenData := map[string]interface{}{
		"user_id":    userID,
		"purpose":    purpose,
		"method":     method,
		"created_at": time.Now().Unix(),
	}

	tokenJSON, _ := json.Marshal(tokenData)

	// Use Redis client from payment handler (assuming it has rdb field)
	// If not, you'll need to pass it through constructor
	err := h.rdb.Set(ctx, tokenKey, tokenJSON, VerificationTokenTTL).Err()
	if err != nil {
		return "", err
	}

	return token, nil
}

// validateVerificationToken validates token and returns user ID and purpose
func (h *PaymentHandler) validateVerificationToken(ctx context.Context, token, expectedPurpose string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("verification token is required")
	}

	tokenKey := fmt.Sprintf("%s%s", VerificationTokenPrefix, token)

	tokenJSON, err := h.rdb.Get(ctx, tokenKey).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("invalid or expired verification token")
	}
	if err != nil {
		return "", fmt.Errorf("failed to validate token: %w", err)
	}

	var tokenData map[string]interface{}
	if err := json.Unmarshal([]byte(tokenJSON), &tokenData); err != nil {
		return "", fmt.Errorf("invalid token data")
	}

	userID, ok := tokenData["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid token format")
	}

	purpose, ok := tokenData["purpose"].(string)
	if !ok || purpose != expectedPurpose {
		return "", fmt.Errorf("token purpose mismatch")
	}

	// Delete token after successful validation (one-time use)
	h.rdb.Del(ctx, tokenKey)

	return userID, nil
}

// Helper methods for OTP session management
// func (h *PaymentHandler) cacheOTPSession(ctx context. Context, key, method, channel string) error {
// 	sessionData := map[string]string{
// 		"method":  method,
// 		"channel": channel,
// 	}
// 	sessionJSON, _ := json.Marshal(sessionData)
// 	return h.rdb.Set(ctx, key, sessionJSON, OTPSessionTTL).Err()
// }

func (h *PaymentHandler) checkOTPSession(ctx context.Context, key string) (bool, error) {
	exists, err := h.rdb.Exists(ctx, key).Result()
	return exists > 0, err
}

func (h *PaymentHandler) getOTPSessionMethod(ctx context.Context, key string) (string, error) {
	sessionJSON, err := h.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	var sessionData map[string]string
	if err := json.Unmarshal([]byte(sessionJSON), &sessionData); err != nil {
		return "", err
	}

	return sessionData["method"], nil
}

func (h *PaymentHandler) deleteOTPSession(ctx context.Context, key string) {
	h.rdb.Del(ctx, key)
}

// func maskRecipient(channel, recipient string) string {
// 	if len(recipient) == 0 {
// 		return ""
// 	}

// 	switch channel {
// 	case "email":
// 		parts := strings.Split(recipient, "@")
// 		if len(parts) != 2 {
// 			return recipient
// 		}
// 		username := parts[0]
// 		domain := parts[1]
// 		if len(username) <= 2 {
// 			return "***@" + domain
// 		}
// 		return username[:2] + "***@" + domain

// 	case "sms", "whatsapp":
// 		if len(recipient) <= 4 {
// 			return "***" + recipient[len(recipient)-2:]
// 		}
// 		return "***" + recipient[len(recipient)-4:]

// 	default:
// 		return recipient
// 	}
// }

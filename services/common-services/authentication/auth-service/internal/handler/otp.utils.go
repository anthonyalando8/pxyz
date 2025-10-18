package handler
import (
	"auth-service/internal/domain"
	"fmt"
	"strings"
	"x/shared/utils/errors"
)

type RequestOTP struct {
	Purpose string `json:"purpose"`
	Channel string `json:"channel"`
	Target  string `json:"target"`
}
// Allowed purposes
func isAllowedPurpose(p string) bool {
	allowed := map[string]bool{
		"login":          true,
		"password_reset": true,
		"verify_email":   true,
		"verify_phone":   true,
		"email_change":   true,
		"register":       true,
		"sys_test":       true,
	}
	return allowed[p]
}

// Allowed channels and recipient resolution
func resolveChannelAndRecipient(req RequestOTP, user *domain.UserProfile) (string, string, error) {
	allowedChannels := map[string]bool{
		"sms":      true,
		"email":    true,
		"whatsapp": true,
	}

	// Explicit target provided
	if req.Target != "" {
		channel := req.Channel
		if channel == "" {
			if strings.Contains(req.Target, "@") {
				channel = "email"
			} else {
				channel = "sms"
			}
		}
		if !allowedChannels[channel] {
			return "", "", xerrors.ErrInvalidChannel
		}
		return channel, req.Target, nil
	}

	// No explicit target → fallback to user
	switch req.Channel {
	case "sms":
		if user.Phone == nil || *user.Phone == "" {
			return "", "", xerrors.ErrUserNoPhone
		}
		return "sms", *user.Phone, nil
	case "whatsapp":
		if user.Phone == nil || *user.Phone == "" {
			return "", "", xerrors.ErrUserNoPhone
		}
		return "whatsapp", *user.Phone, nil
	case "email":
		if user.Email == nil || *user.Email == "" {
			return "", "", xerrors.ErrUserNoEmail
		}
		return "email", *user.Email, nil
	default:
		// auto fallback
		if user.Email != nil && *user.Email != "" {
			return "email", *user.Email, nil
		}
		if user.Phone != nil && *user.Phone != "" {
			return "sms", *user.Phone, nil
		}
		return "", "", xerrors.ErrUserNoComms
	}
}

// Masking logic
func maskRecipient(channel, recipient string) string {
	if channel == "email" {
		parts := strings.Split(recipient, "@")
		if len(parts) != 2 {
			return recipient
		}
		local, domain := parts[0], parts[1]
		if len(local) <= 2 {
			return "***@" + domain
		}
		return fmt.Sprintf("%c*****%c@%s", local[0], local[len(local)-1], domain)
	}

	// SMS / WhatsApp → show only last 4 digits
	if len(recipient) > 4 {
		return "****" + recipient[len(recipient)-4:]
	}
	return "****"
}

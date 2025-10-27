package handler

import (
	"net/http"
	"strings"
	"log"
	//"x/shared/response"
	"auth-service/internal/domain"
	"x/shared/auth/middleware"
)

func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// maskEmail masks email addresses like a***g@gmail.com
func maskEmail(email string) string {
	atIdx := strings.Index(email, "@")
	if atIdx <= 1 {
		return "***" // not a valid email, return masked
	}
	return email[:1] + "***" + email[atIdx-1:]
}

// maskPhone masks phone numbers like +2547****89
func maskPhone(phone string) string {
	if len(phone) < 6 {
		return "****"
	}
	return phone[:5] + "****" + phone[len(phone)-2:]
}

// --- Helper: Extract user ID and user object from context ---
func (h *AuthHandler) getUserFromContext(r *http.Request) (string, *domain.UserProfile, bool) {
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		return "", nil, false
	}

	user, err := h.GetFullUserProfile(r.Context(), userID)
	if err != nil {
		// Log but still return userID — context was valid
		log.Printf("failed to get user by ID %s: %v", userID, err)
		return userID, nil, true
	}

	return userID, user, true
}


func (h *AuthHandler) getDeviceIDFromContext(r *http.Request) string {
	deviceID, ok := r.Context().Value(middleware.ContextDeviceID).(string)
	if !ok {
		return ""
	}
	return deviceID
}

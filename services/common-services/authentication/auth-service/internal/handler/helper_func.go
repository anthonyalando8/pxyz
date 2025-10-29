package handler

import (
	"net/http"
	"strings"
	"log"
	"context"
	//"x/shared/response"
	"auth-service/internal/domain"
	"x/shared/auth/middleware"
		"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"x/shared/genproto/shared/notificationpb"
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

func (h *AuthHandler) sendWelcomeNotification(userID string) {
	ctx := context.Background()

	_, err := h.notificationClient.Client.CreateNotification(ctx, &notificationpb.CreateNotificationsRequest{
		Notifications: []*notificationpb.Notification{
			{
				RequestId:   uuid.New().String(),
				OwnerType:   "user",
				OwnerId:     userID,
				EventType:   "WELCOME",
				ChannelHint: []string{"email", "ws"},
				Title:       "Welcome to Pxyz!",
				Body:        "Your account has been created successfully. Let's get started 🚀",
				Priority:    "high",
				Status:      "pending",
				VisibleInApp: true,
				Payload: func() *structpb.Struct {
					s, _ := structpb.NewStruct(map[string]interface{}{
						"LoginURL": "https://tradex-frontend-jkxr.vercel.app",
					})
					return s
				}(),
			},	
		},
	})

	if err != nil {
		log.Printf("⚠️ Failed to create welcome notification for user=%s: %v", userID, err)
	} else {
		log.Printf("✅ Welcome notification created for user=%s", userID)
	}
}
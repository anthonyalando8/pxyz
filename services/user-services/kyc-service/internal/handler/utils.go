package handler

import (
	"context"
	"log"
	"fmt"
	"time"
	"x/shared/genproto/emailpb"
	"x/shared/auth/middleware"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	notificationpb "x/shared/genproto/shared/notificationpb"
)

func (h *KYCHandler) handleRoleUpgrade(ctx context.Context, userID,newRole string)error{
	return h.urbacservice.AssignRoleByName(ctx, userID, newRole, 0)
}

func (h *KYCHandler) postKYCSubmission(ctx context.Context, userID string) error {
	h.sendKYCSubmissionNotification(userID)

	currentRoleVal := ctx.Value(middleware.ContextRole)
	currentRole, ok := currentRoleVal.(string)
	if !ok || currentRole == "" || currentRole == "temp" {
		currentRole = "any"
	}

	if currentRole == "any" {
		// Run in background
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := h.handleRoleUpgrade(bgCtx, userID, "kyc_unverified"); err != nil {
				log.Printf("[WARN] async role upgrade failed for user %s: %v", userID, err)
			} else {
				log.Printf("[INFO] role upgraded to kyc_unverified for user %s", userID)
			}
		}()
	}

	return nil
}


func (h *KYCHandler)sendKYCSubmissionNotification(userID string) {
	if h.notificationClient == nil{
		return
	}

	go func(uid string) {
		ctx := context.Background() // background context for async processing

		_, err := h.notificationClient.Client.CreateNotification(ctx, &notificationpb.CreateNotificationsRequest{
			Notifications: []*notificationpb.Notification{
				{
					RequestId:      uuid.New().String(),
					OwnerType:      "user",
					OwnerId:        uid,
					EventType:      "KYC_SUBMITTED",
					Title: "KYC Documents Submitted",
					Body: "Your KYC documents have been submitted awaiting review.",
					ChannelHint:    []string{"email"},
					Payload: func() *structpb.Struct {
						s, _ := structpb.NewStruct(map[string]interface{}{})
						return s
					}(),
					VisibleInApp:   false,
					//RecipientEmail: email,
					Priority:       "high",
					Status:         "pending",
				},
			},
		})
		if err != nil {
			log.Printf("[WARN] failed to send KYC submission uid to %s: %v", uid, err)
		} else {
			log.Printf("Successfully queued KYC submission notification | Recipient=%s", uid)
		}
	}(userID)
}


func (h *KYCHandler) sendKYCReviewResult(userID, recipientEmail, status string) {
	subject := "Your KYC review is complete"
	body := fmt.Sprintf(`
		<!DOCTYPE html>
		<html><head><meta charset="UTF-8"><title>KYC Review</title></head>
		<body>
			<p>Hello,</p>
			<p>Your KYC review is now complete. Status: <strong>%s</strong></p>
			<p>Thank you,<br>Pxyz Team</p>
		</body>
		</html>`, status)

	_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
		UserId:         userID,
		RecipientEmail: recipientEmail,
		Subject:        subject,
		Body:           body,
		Type:           "kyc_review_result",
	})
	if err != nil {
		log.Printf("[WARN] failed to send KYC review result email to %s: %v", recipientEmail, err)
	}
}
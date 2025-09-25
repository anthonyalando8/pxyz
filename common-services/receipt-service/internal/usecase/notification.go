package usecase

// import (
// 	"log"
// 	"strings"
// 	"fmt"
// 	"context"
// 	"receipt-service/internal/domain"
// 	notificationpb "x/shared/genproto/shared/notificationpb"

// 	"github.com/google/uuid"
// 	"google.golang.org/protobuf/types/known/structpb"
// )


// func (uc *ReceiptUsecase) sendReceiptNotification(_ context.Context, rec *domain.Receipt) {
// 	if uc.notificationClient == nil {
// 		return
// 	}

// 	send := func(ownerType, ownerID, name, email, phone, codedType string, channels []string, party domain.PartyInfo) {
// 		if len(channels) == 0 {
// 			return
// 		}

// 		ownerType = strings.ToLower(ownerType)

// 		payload := map[string]interface{}{
// 			"ReferenceNumber": rec.Code,
// 			"Amount":          rec.Amount,
// 			"Currency":        rec.Currency,
// 			"Date":            rec.CreatedAt.Format("2006-01-02"),
// 			"Time":            rec.CreatedAt.Format("15:04:05"),
// 			"ExternalRef":     rec.ExternalRef,
// 		}

// 		go func() {
// 			_, err := uc.notificationClient.Client.CreateNotification(context.Background(), &notificationpb.CreateNotificationRequest{
// 				Notification: &notificationpb.Notification{
// 					RequestId:      uuid.New().String(),
// 					OwnerType:      ownerType,
// 					OwnerId:        ownerID,
// 					EventType:      codedType,
// 					Title:          fmt.Sprintf("New Receipt %s", rec.Code),
// 					Body:           fmt.Sprintf("You have a new receipt %s of %.2f %s", rec.Code, rec.Amount, rec.Currency),
// 					ChannelHint:    append(channels, "ws"),
// 					Payload: func() *structpb.Struct {
// 						s, _ := structpb.NewStruct(payload)
// 						return s
// 					}(),
// 					VisibleInApp:   true,
// 					RecipientEmail: email,
// 					RecipientPhone: phone,
// 					Priority:       "high",
// 					Status:         "pending",
// 					RecipientName:  name,
// 				},
// 			})
// 			if err != nil {
// 				log.Printf("[WARN] failed to send receipt notification to %s (%s): %v", name, ownerID, err)
// 			}
// 		}()
// 	}

// 	// Notify the user (either creditor or debitor)
// 	user := rec.Creditor
// 	if rec.Creditor.Type == "system" {
// 		user = rec.Debitor
// 	}
// 	channels := []string{}
// 	if user.Email != "" {
// 		channels = append(channels, "email")
// 	}
// 	if user.Phone != "" {
// 		channels = append(channels, "sms")
// 	}
// 	// Determine event type for user
// 	userCodedType := "ACCOUNT_DEBITED"
// 	if user.ID == rec.Creditor.ID {
// 		userCodedType = "ACCOUNT_CREDITED"
// 	}
// 	send(user.Type, user.ID, user.Name, user.Email, user.Phone, userCodedType, channels, user)

// 	// Notify partner only if he is creditor and debitor is system
// 	if rec.Creditor.Type == "partner" && rec.Debitor.Type == "system" {
// 		channels := []string{}
// 		if rec.Creditor.Email != "" {
// 			channels = append(channels, "email")
// 		}
// 		if rec.Creditor.Phone != "" {
// 			channels = append(channels, "sms")
// 		}
// 		// Partner is creditor → codedType = ACCOUNT_CREDITED
// 		send("partner", rec.Creditor.ID, rec.Creditor.Name, rec.Creditor.Email, rec.Creditor.Phone, "ACCOUNT_CREDITED", channels, rec.Creditor)
// 	}

// }

package usecase

import (
	"context"
	"fmt"
	"log"
	"strings"

	//"time"

	"receipt-service/internal/domain"
	"receipt-service/internal/repository"
	"receipt-service/pkg/generator"
	notificationclient "x/shared/notification"

	notificationpb "x/shared/genproto/shared/notificationpb"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/jackc/pgx/v5"
)

// ReceiptUsecase handles business logic for receipts
type ReceiptUsecase struct {
	repo repository.ReceiptRepository
	gen  *generator.Generator
	notificationClient *notificationclient.NotificationService

}

// NewReceiptUsecase creates a new ReceiptUsecase
func NewReceiptUsecase(r repository.ReceiptRepository, gen *generator.Generator, notificationClient *notificationclient.NotificationService) *ReceiptUsecase {
	return &ReceiptUsecase{
		repo: r,
		gen:  gen,
		notificationClient: notificationClient,
	}
}

// CreateReceipt generates a unique receipt code and inserts a new receipt.
// It retries generation if collisions occur.
func (uc *ReceiptUsecase) CreateReceipt(ctx context.Context, rec *domain.Receipt, tx pgx.Tx) (*domain.Receipt, error) {
	// checkFunc for GenerateUnique: returns true if code exists
	checkFunc := func(code string) bool {
		exists, _ := uc.repo.ExistsByCode(ctx, code)
		return exists
	}

	// generate unique code
	var err error
	rec.Code, err = uc.gen.GenerateUnique(checkFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique receipt code: %w", err)
	}

	// insert into DB
	if err := uc.repo.Create(ctx, rec, tx); err != nil {
		return nil, fmt.Errorf("failed to create receipt: %w", err)
	}

	uc.sendReceiptNotification(ctx, rec)

	return rec, nil
}

func (uc *ReceiptUsecase) sendReceiptNotification(_ context.Context, rec *domain.Receipt) {
	if uc.notificationClient == nil {
		return
	}

	send := func(ownerType, ownerID, name, email, phone, codedType string, channels []string, party domain.PartyInfo) {
		if len(channels) == 0 {
			return
		}

		ownerType = strings.ToLower(ownerType)

		payload := map[string]interface{}{
			"ReferenceNumber": rec.Code,
			"Amount":          rec.Amount,
			"Currency":        rec.Currency,
			"Date":            rec.CreatedAt.Format("2006-01-02"),
			"Time":            rec.CreatedAt.Format("15:04:05"),
			"ExternalRef":     rec.ExternalRef,
			"Party": map[string]interface{}{
				"ID":            party.ID,
				"Type":          party.Type,
				"Name":          party.Name,
				"Phone":         party.Phone,
				"Email":         party.Email,
				"AccountNumber": party.AccountNumber,
				"IsCreditor":    party.IsCreditor,
			},
			"Creditor": rec.Creditor,
			"Debitor":  rec.Debitor,
		}

		go func() {
			_, err := uc.notificationClient.Client.CreateNotification(context.Background(), &notificationpb.CreateNotificationRequest{
				Notification: &notificationpb.Notification{
					RequestId:      uuid.New().String(),
					OwnerType:      ownerType,
					OwnerId:        ownerID,
					EventType:      codedType,
					Title:          fmt.Sprintf("New Receipt %s", rec.Code),
					Body:           fmt.Sprintf("You have a new receipt %s of %.2f %s", rec.Code, rec.Amount, rec.Currency),
					ChannelHint:    append(channels, "ws"),
					Payload: func() *structpb.Struct {
						s, _ := structpb.NewStruct(payload)
						return s
					}(),
					VisibleInApp:   true,
					RecipientEmail: email,
					RecipientPhone: phone,
					Priority:       "high",
					Status:         "pending",
					RecipientName:  name,
				},
			})
			if err != nil {
				log.Printf("[WARN] failed to send receipt notification to %s (%s): %v", name, ownerID, err)
			}
		}()
	}

	// Notify the user (either creditor or debitor)
	user := rec.Creditor
	if rec.Creditor.Type == "system" {
		user = rec.Debitor
	}
	channels := []string{}
	if user.Email != "" {
		channels = append(channels, "email")
	}
	if user.Phone != "" {
		channels = append(channels, "sms")
	}
	// Determine event type for user
	userCodedType := "ACCOUNT_DEBITED"
	if user.ID == rec.Creditor.ID {
		userCodedType = "ACCOUNT_CREDITED"
	}
	send(user.Type, user.ID, user.Name, user.Email, user.Phone, userCodedType, channels, user)

	// Notify partner only if he is creditor and debitor is system
	if rec.Creditor.Type == "partner" && rec.Debitor.Type == "system" {
		channels := []string{}
		if rec.Creditor.Email != "" {
			channels = append(channels, "email")
		}
		if rec.Creditor.Phone != "" {
			channels = append(channels, "sms")
		}
		// Partner is creditor → codedType = ACCOUNT_CREDITED
		send("partner", rec.Creditor.ID, rec.Creditor.Name, rec.Creditor.Email, rec.Creditor.Phone, "ACCOUNT_CREDITED", channels, rec.Creditor)
	}

}

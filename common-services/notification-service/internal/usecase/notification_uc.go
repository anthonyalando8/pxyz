package usecase

import (
	"context"
	"log"
	"time"
	"fmt"

	"notification-service/internal/domain"
	"notification-service/internal/repository"
	"notification-service/pkg/notifier"
	"x/shared/utils/errors"

	authclient "x/shared/auth"
	authpb "x/shared/genproto/authpb"

)

type NotificationUsecase struct {
	repo     repository.Repository
	notifier *notifier.Notifier
	authClient *authclient.AuthService

}

// NewNotificationUsecase creates a new NotificationUsecase with repo + notifier
func NewNotificationUsecase(r repository.Repository, n *notifier.Notifier, authClient *authclient.AuthService) *NotificationUsecase {
	return &NotificationUsecase{
		repo:     r,
		notifier: n,
		authClient: authClient,
	}
}


// -----------------------------
// Notifications
// -----------------------------
func (uc *NotificationUsecase) CreateNotification(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	// 1. Save to DB
	created, err := uc.repo.CreateNotification(ctx, n)
	if err != nil {
		return nil, err
	}

	// 2. Lookup user profile for recipient + personalization
	var recipient string
	var templateData map[string]any

	if created.OwnerType == "user" && uc.authClient != nil {
		profileResp, err := uc.authClient.UserClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{
			UserId: created.OwnerID,
		})
		if err != nil {
			log.Printf("⚠️ Failed to fetch user profile for notification (userID=%s): %v", created.OwnerID, err)
		} else if profileResp != nil && profileResp.Ok {
			user := profileResp.User

			// pick recipient (prefer email > phone)
			if user.Email != "" {
				recipient = user.Email
			} else if user.Phone != "" {
				recipient = user.Phone
			}

			// build template data for rendering
			templateData = map[string]any{
				"UserName": fmt.Sprintf("%s %s", user.FirstName, user.LastName),
				"LoginURL": "https://app.pxyz.com/login", // 🔹 replace with actual frontend URL
				"Year":     time.Now().Year(),
			}
		}
	}

	// 3. Build notifier.Message
	msg := &notifier.Message{
		OwnerID:   created.OwnerID,
		OwnerType: created.OwnerType,
		Recipient: recipient,
		Title:     created.Title,
		Body:      created.Body,
		Metadata:  created.Metadata,
		Channels:  created.ChannelHint,
		Type:      notifier.NotificationType(created.EventType),
		Data:      templateData, // 🔹 passed into template
		Ctx:       ctx,
	}

	// 4. Dispatch asynchronously
	go func(m *notifier.Message) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ Panic recovered in notifier.Notify: %v", r)
			}
		}()
		uc.notifier.Notify(m)
	}(msg)

	return created, nil
}



func (uc *NotificationUsecase) GetNotificationByID(ctx context.Context, id int64) (*domain.Notification, error) {
	return uc.repo.GetNotificationByID(ctx, id)
}

func (uc *NotificationUsecase) GetNotificationByRequestID(ctx context.Context, requestID string) (*domain.Notification, error) {
	return uc.repo.GetNotificationByRequestID(ctx, requestID)
}

func (uc *NotificationUsecase) ListNotificationsByOwner(ctx context.Context, ownerType, ownerID string, limit, offset int) ([]*domain.Notification, error) {
	return uc.repo.ListNotificationsByOwner(ctx, ownerType, ownerID, limit, offset)
}

func (uc *NotificationUsecase) UpdateNotificationStatus(ctx context.Context, id int64, status string, deliveredAt *time.Time) error {
	return uc.repo.UpdateNotificationStatus(ctx, id, status, deliveredAt)
}

func (uc *NotificationUsecase) DeleteNotificationsByOwner(ctx context.Context, ownerType, ownerID string) error {
	return uc.repo.DeleteNotificationsByOwner(ctx, ownerType, ownerID)
}

func (uc *NotificationUsecase) MarkAsRead(ctx context.Context, id int64, ownerType, ownerID string) error {
	if id <= 0 {
		return xerrors.ErrInvalidInput
	}
	return uc.repo.MarkNotificationAsRead(ctx, id, ownerType, ownerID)
}

func (uc *NotificationUsecase) ListUnread(ctx context.Context, ownerType, ownerID string, limit, offset int) ([]*domain.Notification, error) {
	if limit <= 0 {
		limit = 20
	}
	return uc.repo.ListUnreadNotifications(ctx, ownerType, ownerID, limit, offset)
}

func (uc *NotificationUsecase) CountUnread(ctx context.Context, ownerType, ownerID string) (int, error) {
	return uc.repo.CountUnreadNotifications(ctx, ownerType, ownerID)
}

func (uc *NotificationUsecase) HideFromApp(ctx context.Context, id int64, ownerType, ownerID string) error {
	if id <= 0 {
		return xerrors.ErrInvalidInput
	}
	return uc.repo.HideNotification(ctx, id, ownerType, ownerID)
}

// -----------------------------
// Deliveries
// -----------------------------

func (uc *NotificationUsecase) CreateDelivery(ctx context.Context, d *domain.NotificationDelivery) (*domain.NotificationDelivery, error) {
	return uc.repo.CreateDelivery(ctx, d)
}

func (uc *NotificationUsecase) ListPendingDeliveries(ctx context.Context, limit int) ([]*domain.NotificationDelivery, error) {
	return uc.repo.ListPendingDeliveries(ctx, limit)
}

func (uc *NotificationUsecase) MarkDeliverySent(ctx context.Context, id int64) error {
	return uc.repo.MarkDeliverySent(ctx, id)
}

func (uc *NotificationUsecase) MarkDeliveryFailed(ctx context.Context, id int64, errMsg string) error {
	return uc.repo.MarkDeliveryFailed(ctx, id, errMsg)
}

func (uc *NotificationUsecase) IncrementDeliveryAttempt(ctx context.Context, id int64, lastError string) error {
	return uc.repo.IncrementDeliveryAttempt(ctx, id, lastError)
}

func (uc *NotificationUsecase) GetDeliveriesByNotificationID(ctx context.Context, notificationID int64) ([]*domain.NotificationDelivery, error) {
	return uc.repo.GetDeliveriesByNotificationID(ctx, notificationID)
}

// -----------------------------
// Preferences
// -----------------------------

func (uc *NotificationUsecase) UpsertPreference(ctx context.Context, p *domain.NotificationPreference) (*domain.NotificationPreference, error) {
	return uc.repo.UpsertPreference(ctx, p)
}

func (uc *NotificationUsecase) GetPreferenceByOwner(ctx context.Context, ownerType, ownerID string) (*domain.NotificationPreference, error) {
	return uc.repo.GetPreferenceByOwner(ctx, ownerType, ownerID)
}

func (uc *NotificationUsecase) DeletePreferenceByOwner(ctx context.Context, ownerType, ownerID string) error {
	return uc.repo.DeletePreferenceByOwner(ctx, ownerType, ownerID)
}

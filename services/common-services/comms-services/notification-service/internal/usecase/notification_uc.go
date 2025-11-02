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
	adminauthpb "x/shared/genproto/admin/authpb"
	patnerauthpb "x/shared/genproto/partner/authpb"
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
		//return nil, err
		log.Printf("⚠️ Error creating notification in DB: %v", err)
		created = n // Proceed with original notification data
	}

	templateData := make(map[string]any)
	for k, v := range created.Payload {
		templateData[k] = v
	}

	msgRecipients := map[string]string{}

	// ✅ Start with provided recipient info
	if n.RecipientEmail != "" {
		msgRecipients["email"] = n.RecipientEmail
	}
	if n.RecipientPhone != "" {
		msgRecipients["phone"] = n.RecipientPhone
	}
	if n.RecipientName != "" {
		templateData["UserName"] = n.RecipientName
	}

	// Fallback to profile service if needed
	needEmail := contains(created.ChannelHint, "email") && n.RecipientEmail == ""
	needPhone := (contains(created.ChannelHint, "sms") || contains(created.ChannelHint, "whatsapp")) && n.RecipientPhone == ""

	if needEmail || needPhone {
		email, phone, firstName, lastName, resolvedOwnerType, err := uc.fetchProfile(ctx, created.OwnerType, created.OwnerID)
		if err != nil {
			log.Printf("⚠️ Could not fetch profile for notification: %v", err)
		} else {
			// Update recipients
			if needEmail && email != "" {
				msgRecipients["email"] = email
			}
			if needPhone && phone != "" {
				msgRecipients["phone"] = phone
			}
			// Update template data
			if templateData["UserName"] == nil || templateData["UserName"] == "" {
				templateData["UserName"] = fmt.Sprintf("%s %s", firstName, lastName)
			}
			templateData["UserEmail"] = email
			templateData["UserPhone"] = phone

			// Update OwnerType in case it was empty before
			if created.OwnerType == "" && resolvedOwnerType != "" {
				created.OwnerType = resolvedOwnerType
			}
		}
	}

	templateData["Year"] = time.Now().Year()

	// Pick a primary recipient (optional, for backward compatibility)
	var recipient string
	if e, ok := msgRecipients["email"]; ok {
		recipient = e
	} else if p, ok := msgRecipients["phone"]; ok {
		recipient = p
	}

	// 3. Build notifier.Message
	msg := &domain.Message{
		OwnerID:    created.OwnerID,
		OwnerType:  created.OwnerType,
		Recipient:  recipient,
		Recipients: msgRecipients,
		Title:      created.Title,
		Body:       created.Body,
		Metadata:   created.Metadata,
		Channels:   created.ChannelHint,
		Type:       domain.NotificationType(created.EventType),
		Data:       templateData,
		Ctx:        ctx,
	}

	// 4. Dispatch asynchronously
	go func(m *domain.Message) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ Panic recovered in notifier.Notify: %v", r)
			}
		}()
		uc.notifier.Notify(m)
	}(msg)

	return created, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}



// fetchProfile attempts to retrieve a profile from the appropriate auth service based on ownerType
func (uc *NotificationUsecase) fetchProfile(ctx context.Context, ownerType, ownerID string) (email, phone, firstName, lastName string, resolvedOwnerType string, err error) {
	if uc.authClient == nil {
		return "", "", "", "", "", fmt.Errorf("auth client not initialized")
	}

	type result struct {
		email, phone, firstName, lastName, ownerType string
		ok                                           bool
	}

	tryFetch := func(clientType string) result {
		switch clientType {
		case "user":
			if uc.authClient.UserClient == nil {
				return result{}
			}
			resp, e := uc.authClient.UserClient.GetUserProfile(ctx, &authpb.GetUserProfileRequest{UserId: ownerID})
			if e != nil || resp == nil || !resp.Ok || resp.User == nil {
				return result{}
			}
			return result{resp.User.Email, resp.User.Phone, resp.User.FirstName, resp.User.LastName, "user", true}

		case "partner":
			if uc.authClient.PartnerClient == nil {
				return result{}
			}
			resp, e := uc.authClient.PartnerClient.GetUserProfile(ctx, &patnerauthpb.GetUserProfileRequest{UserId: ownerID})
			if e != nil || resp == nil || !resp.Ok || resp.User == nil {
				return result{}
			}
			return result{resp.User.Email, resp.User.Phone, resp.User.FirstName, resp.User.LastName, "partner", true}

		case "admin":
			if uc.authClient.AdminClient == nil {
				return result{}
			}
			resp, e := uc.authClient.AdminClient.GetUserProfile(ctx, &adminauthpb.GetUserProfileRequest{UserId: ownerID})
			if e != nil || resp == nil || !resp.Ok || resp.User == nil {
				return result{}
			}
			return result{resp.User.Email, resp.User.Phone, resp.User.FirstName, resp.User.LastName, "admin", true}
		}
		return result{}
	}

	// If ownerType is known, fetch directly
	if ownerType != "" {
		res := tryFetch(ownerType)
		if !res.ok {
			return "", "", "", "", "", fmt.Errorf("failed to fetch profile for type: %s", ownerType)
		}
		return res.email, res.phone, res.firstName, res.lastName, res.ownerType, nil
	}

	// ownerType unknown → concurrent fetch
	types := []string{"user", "partner", "admin"}
	ch := make(chan result, len(types))

	for _, t := range types {
		go func(t string) {
			ch <- tryFetch(t)
		}(t)
	}

	for i := 0; i < len(types); i++ {
		res := <-ch
		if res.ok {
			return res.email, res.phone, res.firstName, res.lastName, res.ownerType, nil
		}
	}

	return "", "", "", "", "", fmt.Errorf("profile not found for ownerID: %s", ownerID)
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

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
	// authpb "x/shared/genproto/authpb"
	// adminauthpb "x/shared/genproto/admin/authpb"
	// patnerauthpb "x/shared/genproto/partner/authpb"
	"notification-service/internal/helpers"
)

type NotificationUsecase struct {
	repo               repository.Repository
	notifier           *notifier.Notifier
	authClient         *authclient.AuthService
	profileFetcher     *helpers.ProfileFetcher
	notificationBuilder *helpers.NotificationBuilder
}

// NewNotificationUsecase creates a new NotificationUsecase with dependencies
func NewNotificationUsecase(
	r repository.Repository,
	n *notifier.Notifier,
	authClient *authclient.AuthService,
) *NotificationUsecase {
	// Initialize helpers
	profileFetcher := helpers.NewProfileFetcher(authClient)
	notificationBuilder := helpers.NewNotificationBuilder(profileFetcher)

	return &NotificationUsecase{
		repo:                r,
		notifier:            n,
		authClient:          authClient,
		profileFetcher:      profileFetcher,
		notificationBuilder: notificationBuilder,
	}
}

// CreateNotification creates and dispatches a notification
func (uc *NotificationUsecase) CreateNotification(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	if n == nil {
		return nil, fmt.Errorf("notification is nil")
	}

	notificationID := n.ID
	if notificationID == 0 {
		notificationID = -1 // indicate new notification
	}

	log.Printf("[NOTIFICATION CREATE] Starting creation for ID: %v, OwnerID: %s, Type: %s, Channels: %v",
		notificationID, n.OwnerID, n.EventType, n.ChannelHint)

	startTime := time.Now()

	// 1. Persist to database (non-blocking on failure)
	created := uc.persistNotification(ctx, n)

	// 2. Build message with enriched data
	msg, err := uc.notificationBuilder.BuildMessage(ctx, created)
	if err != nil {
		log.Printf("[NOTIFICATION CREATE ERROR] Failed to build message for ID %v: %v", notificationID, err)
		return created, fmt.Errorf("failed to build notification message: %w", err)
	}

	// 3. Dispatch notification asynchronously
	uc.dispatchNotification(msg, notificationID)

	duration := time.Since(startTime)
	log.Printf("[NOTIFICATION CREATE SUCCESS] Notification ID %v created and dispatched (took %v)",
		notificationID, duration)

	return created, nil
}

// persistNotification saves notification to database with error resilience
func (uc *NotificationUsecase) persistNotification(ctx context.Context, n *domain.Notification) *domain.Notification {
	notificationID := n.ID
	if notificationID == 0 {
		notificationID = -1 // indicate new notification
	}

	log.Printf("[DB SAVE] Attempting to persist notification ID: %v", notificationID)

	// Add timeout for DB operation
	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	created, err := uc.repo.CreateNotification(dbCtx, n)
	if err != nil {
		log.Printf("[DB SAVE ERROR] Failed to persist notification ID %v: %v. Proceeding with original data.",
			notificationID, err)
		
		// Return original notification on DB failure
		// The notification will still be dispatched
		return n
	}

	log.Printf("[DB SAVE SUCCESS] Notification ID %d persisted successfully", created.ID)
	return created
}

// dispatchNotification sends the notification asynchronously
func (uc *NotificationUsecase) dispatchNotification(msg *domain.Message, notificationID interface{}) {
	log.Printf("[DISPATCH] Queuing notification ID %s for async dispatch", notificationID)

	go func(m *domain.Message, id interface{}) {
		// Panic recovery
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[DISPATCH PANIC] Recovered from panic while dispatching notification ID %s: %v",
					id, r)
			}
		}()

		dispatchStart := time.Now()
		log.Printf("[DISPATCH START] Dispatching notification ID %s via notifier", id)

		// Create independent context for background operation
		dispatchCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Update message context
		m.Ctx = dispatchCtx

		// Dispatch
		uc.notifier.Notify(m)

		duration := time.Since(dispatchStart)
		log.Printf("[DISPATCH COMPLETE] Notification ID %v dispatched (took %v)", id, duration)
	}(msg, notificationID)
}

// Backward compatibility helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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

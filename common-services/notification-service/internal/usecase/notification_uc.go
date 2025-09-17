package usecase

import (
	"context"
	"time"

	"notification-service/internal/domain"
	"notification-service/internal/repository"
)

type NotificationUsecase struct {
	repo repository.Repository
}

func NewNotificationUsecase(r repository.Repository) *NotificationUsecase {
	return &NotificationUsecase{repo: r}
}

// -----------------------------
// Notifications
// -----------------------------

func (uc *NotificationUsecase) CreateNotification(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	created, err := uc.repo.CreateNotification(ctx, n)
	if err != nil {
		return nil, err
	}
	// TODO: push event to broker for async delivery worker
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

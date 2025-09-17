package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"notification-service/internal/domain"
)

// Repository aggregates all notification DB operations
type Repository interface {
	// Notifications
	CreateNotification(ctx context.Context, n *domain.Notification) (*domain.Notification, error)
	GetNotificationByID(ctx context.Context, id int64) (*domain.Notification, error)
	GetNotificationByRequestID(ctx context.Context, requestID string) (*domain.Notification, error)
	ListNotificationsByOwner(ctx context.Context, ownerType, ownerID string, limit, offset int) ([]*domain.Notification, error)
	UpdateNotificationStatus(ctx context.Context, id int64, status string, deliveredAt *time.Time) error
	DeleteNotificationsByOwner(ctx context.Context, ownerType, ownerID string) error

	// Deliveries
	CreateDelivery(ctx context.Context, d *domain.NotificationDelivery) (*domain.NotificationDelivery, error)
	ListPendingDeliveries(ctx context.Context, limit int) ([]*domain.NotificationDelivery, error)
	MarkDeliverySent(ctx context.Context, id int64) error
	MarkDeliveryFailed(ctx context.Context, id int64, errMsg string) error
	IncrementDeliveryAttempt(ctx context.Context, id int64, lastError string) error
	GetDeliveriesByNotificationID(ctx context.Context, notificationID int64) ([]*domain.NotificationDelivery, error)

	// Preferences
	UpsertPreference(ctx context.Context, p *domain.NotificationPreference) (*domain.NotificationPreference, error)
	GetPreferenceByOwner(ctx context.Context, ownerType, ownerID string) (*domain.NotificationPreference, error)
	DeletePreferenceByOwner(ctx context.Context, ownerType, ownerID string) error
}

type pgRepo struct {
	db *pgxpool.Pool
}

// CreateDelivery implements Repository.
func (p *pgRepo) CreateDelivery(ctx context.Context, d *domain.NotificationDelivery) (*domain.NotificationDelivery, error) {
	panic("unimplemented")
}

// CreateNotification implements Repository.
func (p *pgRepo) CreateNotification(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	panic("unimplemented")
}

// DeleteNotificationsByOwner implements Repository.
func (p *pgRepo) DeleteNotificationsByOwner(ctx context.Context, ownerType string, ownerID string) error {
	panic("unimplemented")
}

// DeletePreferenceByOwner implements Repository.
func (p *pgRepo) DeletePreferenceByOwner(ctx context.Context, ownerType string, ownerID string) error {
	panic("unimplemented")
}

// GetDeliveriesByNotificationID implements Repository.
func (p *pgRepo) GetDeliveriesByNotificationID(ctx context.Context, notificationID int64) ([]*domain.NotificationDelivery, error) {
	panic("unimplemented")
}

// GetNotificationByID implements Repository.
func (p *pgRepo) GetNotificationByID(ctx context.Context, id int64) (*domain.Notification, error) {
	panic("unimplemented")
}

// GetNotificationByRequestID implements Repository.
func (p *pgRepo) GetNotificationByRequestID(ctx context.Context, requestID string) (*domain.Notification, error) {
	panic("unimplemented")
}

// GetPreferenceByOwner implements Repository.
func (p *pgRepo) GetPreferenceByOwner(ctx context.Context, ownerType string, ownerID string) (*domain.NotificationPreference, error) {
	panic("unimplemented")
}

// IncrementDeliveryAttempt implements Repository.
func (p *pgRepo) IncrementDeliveryAttempt(ctx context.Context, id int64, lastError string) error {
	panic("unimplemented")
}

// ListNotificationsByOwner implements Repository.
func (p *pgRepo) ListNotificationsByOwner(ctx context.Context, ownerType string, ownerID string, limit int, offset int) ([]*domain.Notification, error) {
	panic("unimplemented")
}

// ListPendingDeliveries implements Repository.
func (p *pgRepo) ListPendingDeliveries(ctx context.Context, limit int) ([]*domain.NotificationDelivery, error) {
	panic("unimplemented")
}

// MarkDeliveryFailed implements Repository.
func (p *pgRepo) MarkDeliveryFailed(ctx context.Context, id int64, errMsg string) error {
	panic("unimplemented")
}

// MarkDeliverySent implements Repository.
func (p *pgRepo) MarkDeliverySent(ctx context.Context, id int64) error {
	panic("unimplemented")
}

// UpdateNotificationStatus implements Repository.
func (p *pgRepo) UpdateNotificationStatus(ctx context.Context, id int64, status string, deliveredAt *time.Time) error {
	panic("unimplemented")
}

// UpsertPreference implements Repository.
func (*pgRepo) UpsertPreference(ctx context.Context, p *domain.NotificationPreference) (*domain.NotificationPreference, error) {
	panic("unimplemented")
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &pgRepo{db: db}
}

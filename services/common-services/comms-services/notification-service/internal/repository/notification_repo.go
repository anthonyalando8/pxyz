package repository

import (
	"context"
	"log"
	"time"

	"notification-service/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"x/shared/utils/errors"
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
	DeleteNotificationByID(ctx context.Context, id int64) error

	// Notifications - extended
	MarkNotificationAsRead(ctx context.Context, id int64, ownerType, ownerID string) error
	ListUnreadNotifications(ctx context.Context, ownerType, ownerID string, limit, offset int) ([]*domain.Notification, error)
	CountUnreadNotifications(ctx context.Context, ownerType, ownerID string) (int, error)
	HideNotification(ctx context.Context, id int64, ownerType, ownerID string) error

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

// CountUnreadNotifications implements Repository.
func (p *pgRepo) CountUnreadNotifications(ctx context.Context, ownerType, ownerID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM notifications
		WHERE owner_type = $1
		  AND owner_id = $2
		  AND visible_in_app = true
		  AND read_at IS NULL
	`

	var count int
	err := p.db.QueryRow(ctx, query, ownerType, ownerID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}


// HideNotification implements Repository.
func (p *pgRepo) HideNotification(ctx context.Context, id int64, ownerType, ownerID string) error {
	query := `
		UPDATE notifications
		SET visible_in_app = false
		WHERE id = $1
		  AND owner_type = $2
		  AND owner_id = $3
		  AND visible_in_app = true
	`

	ct, err := p.db.Exec(ctx, query, id, ownerType, ownerID)
	if err != nil {
		return err
	}

	if ct.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}


// ListUnreadNotifications implements Repository.
func (p *pgRepo) ListUnreadNotifications(ctx context.Context, ownerType, ownerID string, limit, offset int) ([]*domain.Notification, error) {
	query := `
		SELECT 
			id, request_id, owner_type, owner_id, event_type,
			channel_hint, title, body, payload, priority, status,
			visible_in_app, read_at, created_at, delivered_at, metadata
		FROM notifications
		WHERE owner_type = $1 
		  AND owner_id = $2
		  AND visible_in_app = true
		  AND read_at IS NULL
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := p.db.Query(ctx, query, ownerType, ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		var n domain.Notification
		err := rows.Scan(
			&n.ID,
			&n.RequestID,
			&n.OwnerType,
			&n.OwnerID,
			&n.EventType,
			&n.ChannelHint,
			&n.Title,
			&n.Body,
			&n.Payload,
			&n.Priority,
			&n.Status,
			&n.VisibleInApp,
			&n.ReadAt,
			&n.CreatedAt,
			&n.DeliveredAt,
			&n.Metadata,
		)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return notifications, nil
}


// MarkNotificationAsRead implements Repository.
func (p *pgRepo) MarkNotificationAsRead(ctx context.Context, id int64, ownerType, ownerID string) error {
	query := `
		UPDATE notifications
		SET read_at = NOW()
		WHERE id = $1
		  AND owner_type = $2
		  AND owner_id = $3
		  AND read_at IS NULL
	`

	ct, err := p.db.Exec(ctx, query, id, ownerType, ownerID)
	if err != nil {
		return err
	}

	if ct.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}


// CreateDelivery implements Repository.
func (p *pgRepo) CreateDelivery(ctx context.Context, d *domain.NotificationDelivery) (*domain.NotificationDelivery, error) {
	panic("unimplemented")
}

// CreateNotification implements Repository.
// internal/repository/notification_repo.go

func (p *pgRepo) CreateNotification(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	// Ensure RequestID is set
	if n.RequestID == "" {
		n.RequestID = uuid.New().String()
	}

	query := `
		INSERT INTO notifications (
			request_id, owner_type, owner_id, event_type,
			channel_hint, title, body, payload, priority,
			status, visible_in_app, read_at, delivered_at, metadata
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14
		)
		RETURNING 
			id, request_id, owner_type, owner_id, event_type,
			channel_hint, title, body, payload, priority, status,
			visible_in_app, read_at, created_at, delivered_at, metadata
	`

	row := p.db.QueryRow(ctx, query,
		n.RequestID,
		n. OwnerType,
		n.OwnerID,
		n.EventType,
		n.ChannelHint,
		n.Title,
		n.Body,
		n. Payload,
		n.Priority,
		n.Status,
		n.VisibleInApp,
		n.ReadAt,
		n.DeliveredAt,
		n. Metadata,
	)

	var created domain.Notification
	err := row.Scan(
		&created.ID,
		&created.RequestID,
		&created.OwnerType,
		&created.OwnerID,
		&created.EventType,
		&created. ChannelHint,
		&created.Title,
		&created.Body,
		&created. Payload,
		&created. Priority,
		&created.Status,
		&created.VisibleInApp,
		&created.ReadAt,
		&created.CreatedAt,
		&created.DeliveredAt,
		&created.Metadata,
	)
	if err != nil {
		return nil, err
	}

	// ✅ Preserve recipient data from input (not stored in DB yet)
	created.RecipientEmail = n.RecipientEmail
	created.RecipientPhone = n.RecipientPhone
	created.RecipientName = n.RecipientName

	// ✅ Log for debugging
	if created.RecipientPhone != "" {
		log.Printf("[DB CREATE] Notification ID %d created with phone: %s", 
			created.ID, maskPhone(created.RecipientPhone))
	}

	return &created, nil
}

// Helper for logging (add this if not present)
func maskPhone(phone string) string {
	if phone == "" {
		return "[empty]"
	}
	if len(phone) < 4 {
		return "***"
	}
	return "***" + phone[len(phone)-4:]
}


// DeleteNotificationsByOwner implements Repository.
func (p *pgRepo) DeleteNotificationsByOwner(ctx context.Context, ownerType string, ownerID string) error {
	query := `
		DELETE FROM notifications
		WHERE owner_type = $1 AND owner_id = $2
	`

	ct, err := p.db.Exec(ctx, query, ownerType, ownerID)
	if err != nil {
		return err
	}

	if ct.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}


func (p *pgRepo) DeleteNotificationByID(ctx context.Context, id int64) error {
	query := `DELETE FROM notifications WHERE id = $1`

	ct, err := p.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if ct.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
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
	query := `
		SELECT 
			id, request_id, owner_type, owner_id, event_type,
			channel_hint, title, body, payload, priority, status,
			visible_in_app, read_at, created_at, delivered_at, metadata
		FROM notifications
		WHERE id = $1
	`

	row := p.db.QueryRow(ctx, query, id)

	var n domain.Notification
	err := row.Scan(
		&n.ID,
		&n.RequestID,
		&n.OwnerType,
		&n.OwnerID,
		&n.EventType,
		&n.ChannelHint,
		&n.Title,
		&n.Body,
		&n.Payload,
		&n.Priority,
		&n.Status,
		&n.VisibleInApp,
		&n.ReadAt,
		&n.CreatedAt,
		&n.DeliveredAt,
		&n.Metadata,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound // not found
		}
		return nil, err
	}

	return &n, nil
}


// GetNotificationByRequestID implements Repository.
func (p *pgRepo) GetNotificationByRequestID(ctx context.Context, requestID string) (*domain.Notification, error) {
	query := `
		SELECT 
			id, request_id, owner_type, owner_id, event_type,
			channel_hint, title, body, payload, priority, status,
			visible_in_app, read_at, created_at, delivered_at, metadata
		FROM notifications
		WHERE request_id = $1
	`

	row := p.db.QueryRow(ctx, query, requestID)

	var n domain.Notification
	err := row.Scan(
		&n.ID,
		&n.RequestID,
		&n.OwnerType,
		&n.OwnerID,
		&n.EventType,
		&n.ChannelHint,
		&n.Title,
		&n.Body,
		&n.Payload,
		&n.Priority,
		&n.Status,
		&n.VisibleInApp,
		&n.ReadAt,
		&n.CreatedAt,
		&n.DeliveredAt,
		&n.Metadata,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	return &n, nil
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
	query := `
		SELECT 
			id, request_id, owner_type, owner_id, event_type,
			channel_hint, title, body, payload, priority, status,
			visible_in_app, read_at, created_at, delivered_at, metadata
		FROM notifications
		WHERE owner_type = $1 AND owner_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := p.db.Query(ctx, query, ownerType, ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		var n domain.Notification
		err := rows.Scan(
			&n.ID,
			&n.RequestID,
			&n.OwnerType,
			&n.OwnerID,
			&n.EventType,
			&n.ChannelHint,
			&n.Title,
			&n.Body,
			&n.Payload,
			&n.Priority,
			&n.Status,
			&n.VisibleInApp,
			&n.ReadAt,
			&n.CreatedAt,
			&n.DeliveredAt,
			&n.Metadata,
		)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return notifications, nil
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
	query := `
		UPDATE notifications
		SET status = $1,
		    delivered_at = COALESCE($2, delivered_at)
		WHERE id = $3
	`

	ct, err := p.db.Exec(ctx, query, status, deliveredAt, id)
	if err != nil {
		return err
	}

	if ct.RowsAffected() == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}


// UpsertPreference implements Repository.
func (*pgRepo) UpsertPreference(ctx context.Context, p *domain.NotificationPreference) (*domain.NotificationPreference, error) {
	panic("unimplemented")
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &pgRepo{db: db}
}

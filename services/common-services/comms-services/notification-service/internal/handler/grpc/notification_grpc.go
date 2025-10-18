package grpchandler

import (
	"context"
	"log"
	"time"

	"notification-service/internal/domain"
	"notification-service/internal/usecase"
	"x/shared/genproto/shared/notificationpb"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NotificationHandler struct {
	notificationpb.UnimplementedNotificationServiceServer
	uc *usecase.NotificationUsecase
}

func NewNotificationGRPCHandler(uc *usecase.NotificationUsecase) *NotificationHandler {
	return &NotificationHandler{uc: uc}
}

// ===== Critical Methods =====

// CreateNotifications handles multiple notifications at once
func (h *NotificationHandler) CreateNotification(
	ctx context.Context,
	req *notificationpb.CreateNotificationsRequest,
) (*notificationpb.NotificationsResponse, error) {
	var createdNotifications []*notificationpb.Notification

	for _, nPB := range req.GetNotifications() {
		if nPB.Payload != nil {
			log.Printf(
				"[NotificationHandler] Received Payload | EventType=%s | OwnerID=%s | Payload=%+v",
				nPB.EventType,
				nPB.OwnerId,
				nPB.Payload.AsMap(),
			)
		} else {
			log.Printf(
				"[NotificationHandler] Received Notification with empty payload | EventType=%s | OwnerID=%s",
				nPB.EventType,
				nPB.OwnerId,
			)
		}

		nDomain := pbToDomain(nPB)
		created, err := h.uc.CreateNotification(ctx, nDomain)
		if err != nil {
			return nil, err
		}
		createdNotifications = append(createdNotifications, domainToPB(created))
	}

	return &notificationpb.NotificationsResponse{Notifications: createdNotifications}, nil
}

// DeleteNotificationsByOwner clears all notifications for a user/owner
func (h *NotificationHandler) DeleteNotificationsByOwner(ctx context.Context, req *notificationpb.DeleteNotificationsByOwnerRequest) (*notificationpb.DeleteNotificationsByOwnerResponse, error) {
	err := h.uc.DeleteNotificationsByOwner(ctx, req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		return nil, err
	}

	return &notificationpb.DeleteNotificationsByOwnerResponse{Success: true}, nil
}

// GetNotification by ID
func (h *NotificationHandler) GetNotification(ctx context.Context, req *notificationpb.GetNotificationRequest) (*notificationpb.NotificationsResponse, error) {
	n, err := h.uc.GetNotificationByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationsResponse{Notifications: []*notificationpb.Notification{domainToPB(n)}}, nil
}

// GetNotification by RequestID
func (h *NotificationHandler) GetNotificationByRequestID(ctx context.Context, req *notificationpb.GetNotificationByRequestIDRequest) (*notificationpb.NotificationsResponse, error) {
	n, err := h.uc.GetNotificationByRequestID(ctx, req.GetRequestId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationsResponse{Notifications: []*notificationpb.Notification{domainToPB(n)}}, nil
}

// ListNotifications
func (h *NotificationHandler) ListNotifications(ctx context.Context, req *notificationpb.ListNotificationsRequest) (*notificationpb.ListNotificationsResponse, error) {
	list, err := h.uc.ListNotificationsByOwner(ctx, req.GetOwnerType(), req.GetOwnerId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, err
	}

	var res []*notificationpb.Notification
	for _, n := range list {
		res = append(res, domainToPB(n))
	}

	return &notificationpb.ListNotificationsResponse{Notifications: res}, nil
}

// MarkAsRead
func (h *NotificationHandler) MarkAsRead(ctx context.Context, req *notificationpb.MarkAsReadRequest) (*notificationpb.NotificationsResponse, error) {
	err := h.uc.MarkAsRead(ctx, req.GetId(), req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		return nil, err
	}
	n, err := h.uc.GetNotificationByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationsResponse{Notifications: []*notificationpb.Notification{domainToPB(n)}}, nil
}

// HideFromApp
func (h *NotificationHandler) HideFromApp(ctx context.Context, req *notificationpb.HideFromAppRequest) (*notificationpb.NotificationsResponse, error) {
	err := h.uc.HideFromApp(ctx, req.GetId(), req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		return nil, err
	}
	n, err := h.uc.GetNotificationByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationsResponse{Notifications: []*notificationpb.Notification{domainToPB(n)}}, nil
}

// CountUnread
func (h *NotificationHandler) CountUnread(ctx context.Context, req *notificationpb.CountUnreadRequest) (*notificationpb.CountUnreadResponse, error) {
	count, err := h.uc.CountUnread(ctx, req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.CountUnreadResponse{Count: int32(count)}, nil
}

// ===== Helpers: PB ↔ Domain =====

func pbToDomain(pb *notificationpb.Notification) *domain.Notification {
	if pb == nil {
		return nil
	}

	var readAt *time.Time
	if pb.ReadAt != nil {
		t := pb.ReadAt.AsTime()
		readAt = &t
	}

	var deliveredAt *time.Time
	if pb.DeliveredAt != nil {
		t := pb.DeliveredAt.AsTime()
		deliveredAt = &t
	}

	createdAt := time.Now()
	if pb.CreatedAt != nil {
		createdAt = pb.CreatedAt.AsTime()
	}

	return &domain.Notification{
		ID:             pb.Id,
		RequestID:      pb.RequestId,
		OwnerType:      pb.OwnerType,
		OwnerID:        pb.OwnerId,
		EventType:      pb.EventType,
		ChannelHint:    pb.ChannelHint,
		Title:          pb.Title,
		Body:           pb.Body,
		Payload:        pb.Payload.AsMap(),
		Priority:       pb.Priority,
		Status:         pb.Status,
		VisibleInApp:   pb.VisibleInApp,
		ReadAt:         readAt,
		CreatedAt:      createdAt,
		DeliveredAt:    deliveredAt,
		Metadata:       pb.Metadata.AsMap(),
		RecipientEmail: pb.RecipientEmail,
		RecipientPhone: pb.RecipientPhone,
		RecipientName:  pb.RecipientName,
	}
}

func domainToPB(n *domain.Notification) *notificationpb.Notification {
	if n == nil {
		return nil
	}

	pb := &notificationpb.Notification{
		Id:           n.ID,
		RequestId:    n.RequestID,
		OwnerType:    n.OwnerType,
		OwnerId:      n.OwnerID,
		EventType:    n.EventType,
		ChannelHint:  n.ChannelHint,
		Title:        n.Title,
		Body:         n.Body,
		Priority:     n.Priority,
		Status:       n.Status,
		VisibleInApp: n.VisibleInApp,
		CreatedAt:    timestamppb.New(n.CreatedAt),
		Payload:      mapToStructPB(n.Payload),
		Metadata:     mapToStructPB(n.Metadata),
	}

	if n.ReadAt != nil {
		pb.ReadAt = timestamppb.New(*n.ReadAt)
	}
	if n.DeliveredAt != nil {
		pb.DeliveredAt = timestamppb.New(*n.DeliveredAt)
	}

	pb.RecipientEmail = n.RecipientEmail
	pb.RecipientPhone = n.RecipientPhone
	pb.RecipientName = n.RecipientName

	return pb
}

func mapToStructPB(m map[string]interface{}) *structpb.Struct {
	if m == nil {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}

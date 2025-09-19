package grpchandler

import (
	"context"
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

func NewNotificationGRPCHandler(uc *usecase.NotificationUsecase,) *NotificationHandler {
	return &NotificationHandler{uc: uc,}
}

// ===== Critical Methods =====

// CreateNotification is used by other services to post notifications
func (h *NotificationHandler) CreateNotification(ctx context.Context, req *notificationpb.CreateNotificationRequest) (*notificationpb.NotificationResponse, error) {
	n := pbToDomain(req.GetNotification())

	created, err := h.uc.CreateNotification(ctx, n)
	if err != nil {
		return nil, err
	}

	return &notificationpb.NotificationResponse{
		Notification: domainToPB(created),
	}, nil
}


// DeleteNotificationsByOwner clears all notifications for a user/owner
func (h *NotificationHandler) DeleteNotificationsByOwner(ctx context.Context, req *notificationpb.DeleteNotificationsByOwnerRequest) (*notificationpb.DeleteNotificationsByOwnerResponse, error) {
	err := h.uc.DeleteNotificationsByOwner(ctx, req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		return nil, err
	}

	return &notificationpb.DeleteNotificationsByOwnerResponse{Success: true}, nil
}

// ===== Optional Methods (REST might cover, but available for gRPC too) =====

func (h *NotificationHandler) GetNotification(ctx context.Context, req *notificationpb.GetNotificationRequest) (*notificationpb.NotificationResponse, error) {
	n, err := h.uc.GetNotificationByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationResponse{Notification: domainToPB(n)}, nil
}

func (h *NotificationHandler) GetNotificationByRequestID(ctx context.Context, req *notificationpb.GetNotificationByRequestIDRequest) (*notificationpb.NotificationResponse, error) {
	n, err := h.uc.GetNotificationByRequestID(ctx, req.GetRequestId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationResponse{Notification: domainToPB(n)}, nil
}

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

func (h *NotificationHandler) MarkAsRead(ctx context.Context, req *notificationpb.MarkAsReadRequest) (*notificationpb.NotificationResponse, error) {
	err := h.uc.MarkAsRead(ctx, req.GetId(), req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		return nil, err
	}
	// fetch updated notification
	n, err := h.uc.GetNotificationByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationResponse{Notification: domainToPB(n)}, nil
}

func (h *NotificationHandler) HideFromApp(ctx context.Context, req *notificationpb.HideFromAppRequest) (*notificationpb.NotificationResponse, error) {
	err := h.uc.HideFromApp(ctx, req.GetId(), req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		return nil, err
	}
	// fetch updated notification
	n, err := h.uc.GetNotificationByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &notificationpb.NotificationResponse{Notification: domainToPB(n)}, nil
}

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

	return &domain.Notification{
		ID:           pb.Id,
		RequestID:    pb.RequestId,
		OwnerType:    pb.OwnerType,
		OwnerID:      pb.OwnerId,
		EventType:    pb.EventType,
		ChannelHint:  pb.ChannelHint,
		Title:        pb.Title,
		Body:         pb.Body,
		Payload:      pb.Payload.AsMap(),
		Priority:     pb.Priority,
		Status:       pb.Status,
		VisibleInApp: pb.VisibleInApp,
		ReadAt:       readAt,
		CreatedAt:    pb.CreatedAt.AsTime(),
		DeliveredAt:  deliveredAt,
		Metadata:     pb.Metadata.AsMap(),
	}
}

func domainToPB(n *domain.Notification) *notificationpb.Notification {
	if n == nil {
		return nil
	}

	var readAt *time.Time
	if n.ReadAt != nil {
		readAt = n.ReadAt
	}

	var deliveredAt *time.Time
	if n.DeliveredAt != nil {
		deliveredAt = n.DeliveredAt
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

	if readAt != nil {
		pb.ReadAt = timestamppb.New(*readAt)
	}
	if deliveredAt != nil {
		pb.DeliveredAt = timestamppb.New(*deliveredAt)
	}

	return pb
}

// mapToStructPB safely converts a map[string]interface{} to *structpb.Struct, returning nil if the map is nil or on error.
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

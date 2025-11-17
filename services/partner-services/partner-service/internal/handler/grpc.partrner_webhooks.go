// handler/grpc_partner_webhooks.go
package handler

import (
	"context"
	"log"
	partnersvcpb "x/shared/genproto/partner/svcpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UpdateWebhookConfig updates webhook settings
func (h *GRPCPartnerHandler) UpdateWebhookConfig(
	ctx context.Context,
	req *partnersvcpb.UpdateWebhookConfigRequest,
) (*partnersvcpb.WebhookConfigResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	err := h.uc.UpdateWebhookConfig(ctx, req.PartnerId, req.WebhookUrl, req.WebhookSecret, req.CallbackUrl)
	if err != nil {
		log.Printf("[ERROR] UpdateWebhookConfig failed: %v", err)
		return &partnersvcpb.WebhookConfigResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &partnersvcpb.WebhookConfigResponse{
		Success: true,
		Message: "Webhook configuration updated successfully",
	}, nil
}

// TestWebhook sends a test webhook
func (h *GRPCPartnerHandler) TestWebhook(
	ctx context.Context,
	req *partnersvcpb.TestWebhookRequest,
) (*partnersvcpb.TestWebhookResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	statusCode, err := h.uc.TestWebhook(ctx, req.PartnerId)
	if err != nil {
		log.Printf("[ERROR] TestWebhook failed: %v", err)
		return &partnersvcpb.TestWebhookResponse{
			Success:    false,
			StatusCode: 0,
			Message:    err.Error(),
		}, nil
	}

	return &partnersvcpb.TestWebhookResponse{
		Success:    statusCode >= 200 && statusCode < 300,
		StatusCode: int32(statusCode),
		Message:    "Webhook test completed",
	}, nil
}

// ListWebhookLogs returns webhook delivery logs
func (h *GRPCPartnerHandler) ListWebhookLogs(
	ctx context.Context,
	req *partnersvcpb.ListWebhookLogsRequest,
) (*partnersvcpb.ListWebhookLogsResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	logs, total, err := h.uc.ListWebhookLogs(ctx, req.PartnerId, int(req.Limit), int(req.Offset))
	if err != nil {
		log.Printf("[ERROR] ListWebhookLogs failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list webhook logs")
	}

	protoLogs := make([]*partnersvcpb.WebhookLog, 0, len(logs))
	for _, log := range logs {
		protoLog := &partnersvcpb.WebhookLog{
			Id:             log.ID,
			PartnerId:      log.PartnerID,
			EventType:      log.EventType,
			Status:         log.Status,
			Attempts:       int32(log.Attempts),
			ErrorMessage:   getStringValue(log.ErrorMessage),
			CreatedAt:      timestamppb.New(log.CreatedAt),
		}
		if log.ResponseStatus != nil {
			protoLog.ResponseStatus = int32(*log.ResponseStatus)
		}
		if log.LastAttemptAt != nil {
			protoLog.LastAttemptAt = timestamppb.New(*log.LastAttemptAt)
		}
		protoLogs = append(protoLogs, protoLog)
	}

	return &partnersvcpb.ListWebhookLogsResponse{
		Logs:       protoLogs,
		TotalCount: total,
	}, nil
}

func getStringValue(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}
// internal/handler/notification.go
package handler

import (
	"context"
	"encoding/json"
)
func (h *P2PWebSocketHandler) handleListNotifications(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "notification.list",
		Success: false,
		Message: "Notification system not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleMarkNotificationRead(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "notification.mark_read",
		Success: false,
		Message: "Notification system not yet implemented",
	})
}

package handler

import (
	"context"
	"encoding/json"
)

func (h *P2PWebSocketHandler) handleSendMessage(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "chat.send_message",
		Success: false,
		Message: "Chat not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleGetMessages(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "chat.get_messages",
		Success: false,
		Message: "Chat not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleMarkRead(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "chat.mark_read",
		Success: false,
		Message: "Chat not yet implemented",
	})
}
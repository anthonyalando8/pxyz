package handler

import (
	"context"
	"encoding/json"
)
func (h *P2PWebSocketHandler) handleRaiseDispute(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "dispute.raise",
		Success: false,
		Message: "Dispute system not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleGetDispute(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "dispute.get",
		Success: false,
		Message: "Dispute system not yet implemented",
	})
}

package handler

import (
	"context"
	"encoding/json"

)
func (h *P2PWebSocketHandler) handleCreateReview(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "review.create",
		Success: false,
		Message: "Review system not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleListReviews(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "review.list",
		Success: false,
		Message: "Review system not yet implemented",
	})
}

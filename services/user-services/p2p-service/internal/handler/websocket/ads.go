package handler

import (
	"context"
	"encoding/json"
)

func (h *P2PWebSocketHandler) handleCreateAd(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "ad.create",
		Success: false,
		Message: "Ad creation not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleListAds(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "ad.list",
		Success: false,
		Message: "Ad listing not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleUpdateAd(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "ad.update",
		Success: false,
		Message: "Ad update not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleDeleteAd(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "ad.delete",
		Success: false,
		Message: "Ad deletion not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleGetMyAds(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "ad.my_ads",
		Success: false,
		Message: "Get my ads not yet implemented",
	})
}
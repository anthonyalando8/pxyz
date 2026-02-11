package handler

import (
	"context"
	"encoding/json"
)
func (h *P2PWebSocketHandler) handleCreateOrder(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "order.create",
		Success: false,
		Message: "Order creation not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleListOrders(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "order.list",
		Success: false,
		Message: "Order listing not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleGetOrder(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "order.get",
		Success: false,
		Message: "Get order not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleCancelOrder(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "order.cancel",
		Success: false,
		Message: "Order cancellation not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleConfirmPayment(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "order.confirm_payment",
		Success: false,
		Message: "Payment confirmation not yet implemented",
	})
}

func (h *P2PWebSocketHandler) handleReleaseCrypto(ctx context.Context, client *Client, data json.RawMessage) {
	client.SendJSON(&WSResponse{
		Type:    "order.release_crypto",
		Success: false,
		Message: "Crypto release not yet implemented",
	})
}

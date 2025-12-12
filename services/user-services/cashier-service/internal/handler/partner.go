package handler

import (
    "context"
    "encoding/json"
)

// ============================================================================
// PARTNER OPERATIONS
// ============================================================================

// Get partners by service
func (h *PaymentHandler) handleGetPartners(ctx context.Context, client *Client, data json.RawMessage) {
	var req struct {
		Service string `json:"service"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		client.SendError("invalid request format")
		return
	}

	partners, err := h.GetPartnersByService(ctx, req.Service)
	if err != nil {
		client.SendError("failed to fetch partners: " + err.Error())
		return
	}

	client.SendSuccess("partners retrieved", map[string]interface{}{
		"partners": partners,
		"count":    len(partners),
	})
}
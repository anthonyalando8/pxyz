// internal/handler/admin_crypto_deposit_handler.go
package handler

import (
	"encoding/json"
	"net/http"

	cryptopb "x/shared/genproto/shared/accounting/cryptopb"
	"x/shared/response"

	"go.uber.org/zap"
)

// ============================================================================
// DEPOSIT QUERIES
// ============================================================================

// GetDeposit retrieves a specific deposit by ID
// GET /admin/svc/crypto/deposits
func (h *AdminHandler) GetDeposit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	depositID := r.URL.Query().Get("deposit_id")
	userID := r.URL.Query().Get("user_id")

	if depositID == "" {
		h.respondError(w, http.StatusBadRequest, "deposit_id is required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.DepositClient.GetDeposit(ctx, &cryptopb.GetDepositRequest{
		DepositId: depositID,
		UserId:    userID,
	})
	if err != nil {
		h.logger.Error("Failed to get deposit",
			zap.String("deposit_id", depositID),
			zap.Error(err))
		h.respondError(w, http.StatusNotFound, "Deposit not found", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"deposit": resp.Deposit,
		},
	})
}

// GetUserDeposits retrieves all deposits for a specific user
// GET /admin/svc/crypto/deposits/user
func (h *AdminHandler) GetUserDeposits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.respondError(w, http.StatusBadRequest, "user_id is required", nil)
		return
	}

	// Optional filters
	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	status := r.URL.Query().Get("status")

	// Pagination
	page := parseIntQuery(r, "page", 1)
	pageSize := parseIntQuery(r, "page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}

	// Call gRPC service
	resp, err := h.cryptoClient.DepositClient.GetUserDeposits(ctx, &cryptopb.GetUserDepositsRequest{
		UserId: userID,
		Chain:  stringToChainEnum(chain),
		Asset:  asset,
		Status: stringToDepositStatusEnum(status),
		Pagination: &cryptopb.PaginationRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	})
	if err != nil {
		h.logger.Error("Failed to get user deposits",
			zap.String("user_id", userID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get deposits", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"deposits":   resp.Deposits,
			"pagination": resp.Pagination,
		},
	})
}

// GetPendingDeposits retrieves all pending deposits (waiting confirmations)
// GET /admin/svc/crypto/deposits/pending
func (h *AdminHandler) GetPendingDeposits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Optional user filter
	userID := r.URL.Query().Get("user_id")

	// Call gRPC service
	resp, err := h.cryptoClient.DepositClient.GetPendingDeposits(ctx, &cryptopb.GetPendingDepositsRequest{
		UserId: userID,
	})
	if err != nil {
		h.logger.Error("Failed to get pending deposits",
			zap.String("user_id", userID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get pending deposits", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"deposits": resp.Deposits,
			"total":    resp.Total,
		},
	})
}

// GetAllDeposits retrieves all deposits across all users (admin overview)
// GET /admin/svc/crypto/deposits/all
func (h *AdminHandler) GetAllDeposits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Optional filters
	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	status := r.URL.Query().Get("status")

	// Pagination
	page := parseIntQuery(r, "page", 1)
	pageSize := parseIntQuery(r, "page_size", 50)
	if pageSize > 100 {
		pageSize = 100
	}

	// Call gRPC service with empty user_id to get all
	resp, err := h.cryptoClient.DepositClient.GetUserDeposits(ctx, &cryptopb.GetUserDepositsRequest{
		UserId: "", // Empty = all users
		Chain:  stringToChainEnum(chain),
		Asset:  asset,
		Status: stringToDepositStatusEnum(status),
		Pagination: &cryptopb.PaginationRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	})
	if err != nil {
		h.logger.Error("Failed to get all deposits", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get deposits", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"deposits":   resp.Deposits,
			"pagination": resp.Pagination,
		},
	})
}

// ============================================================================
// DEPOSIT MONITORING & ADMIN INTERVENTIONS
// ============================================================================

// GetDepositStats retrieves deposit statistics
// GET /admin/svc/crypto/deposits/stats
func (h *AdminHandler) GetDepositStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get deposits for statistics
	resp, err := h.cryptoClient.DepositClient.GetUserDeposits(ctx, &cryptopb.GetUserDepositsRequest{
		UserId: "", // All users
		Pagination: &cryptopb.PaginationRequest{
			Page:     1,
			PageSize: 1000, // Get many for stats
		},
	})
	if err != nil {
		h.logger.Error("Failed to get deposits for stats", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get deposit stats", err)
		return
	}

	// Calculate statistics
	stats := calculateDepositStats(resp.Deposits)

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

// GetRecentDeposits retrieves most recent deposits across all users
// GET /admin/svc/crypto/deposits/recent
func (h *AdminHandler) GetRecentDeposits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := parseIntQuery(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}

	// Call gRPC service
	resp, err := h.cryptoClient.DepositClient.GetUserDeposits(ctx, &cryptopb.GetUserDepositsRequest{
		UserId: "", // All users
		Pagination: &cryptopb.PaginationRequest{
			Page:     1,
			PageSize: int32(limit),
		},
	})
	if err != nil {
		h.logger.Error("Failed to get recent deposits", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get recent deposits", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"deposits": resp.Deposits,
			"total":    len(resp.Deposits),
		},
	})
}

// GetDepositsByTxHash retrieves deposit by blockchain transaction hash
// GET /admin/svc/crypto/deposits/tx/{tx_hash}
func (h *AdminHandler) GetDepositByTxHash(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	txHash := r.URL.Query().Get("tx_hash")
	if txHash == "" {
		h.respondError(w, http.StatusBadRequest, "tx_hash is required", nil)
		return
	}

	// Get all deposits and filter by tx_hash
	// Note: In production, you should add a dedicated gRPC method for this
	resp, err := h.cryptoClient.DepositClient.GetUserDeposits(ctx, &cryptopb.GetUserDepositsRequest{
		UserId: "",
		Pagination: &cryptopb.PaginationRequest{
			Page:     1,
			PageSize: 100,
		},
	})
	if err != nil {
		h.logger.Error("Failed to search deposits by tx_hash",
			zap.String("tx_hash", txHash),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to search deposits", err)
		return
	}

	// Filter by tx_hash
	var matchingDeposits []*cryptopb.Deposit
	for _, deposit := range resp.Deposits {
		if deposit.TxHash == txHash {
			matchingDeposits = append(matchingDeposits, deposit)
		}
	}

	if len(matchingDeposits) == 0 {
		h.respondError(w, http.StatusNotFound, "No deposits found for this tx_hash", nil)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"deposits": matchingDeposits,
			"total":    len(matchingDeposits),
		},
	})
}

// ============================================================================
// FUTURE: ADMIN INTERVENTIONS (TODO)
// ============================================================================

// RetryDepositProcessing manually retries processing a stuck deposit
// POST /admin/svc/crypto/deposits/{deposit_id}/retry
func (h *AdminHandler) RetryDepositProcessing(w http.ResponseWriter, r *http.Request) {
	//ctx := r.Context()
	userID, _, ok := h.getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		DepositID string `json:"deposit_id"`
		AdminID   string `json:"-"`
		Reason    string `json:"reason,omitempty"`
	}
	req.AdminID = userID
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.DepositID == "" || req.AdminID == "" {
		h.respondError(w, http.StatusBadRequest, "deposit_id and admin_id are required", nil)
		return
	}

	h.logger.Info("Admin retrying deposit processing",
		zap.String("deposit_id", req.DepositID),
		zap.String("admin_id", req.AdminID))

	// TODO: Implement gRPC method for this
	// resp, err := h.cryptoClient.DepositClient.RetryDeposit(ctx, &cryptopb.RetryDepositRequest{
	//     DepositId: req.DepositID,
	//     AdminId:   req.AdminID,
	//     Reason:    req.Reason,
	// })

	h.respondJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"success": false,
		"message": "Retry deposit processing not yet implemented",
	})
}

// MarkDepositAsFailed manually marks a deposit as failed
// POST /admin/svc/crypto/deposits/{deposit_id}/fail
func (h *AdminHandler) MarkDepositAsFailed(w http.ResponseWriter, r *http.Request) {
	//ctx := r.Context()
	userID, _, ok := h.getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		DepositID string `json:"deposit_id"`
		AdminID   string `json:"-"`
		Reason    string `json:"reason"`
	}
	req.AdminID = userID

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.DepositID == "" || req.AdminID == "" || req.Reason == "" {
		h.respondError(w, http.StatusBadRequest, "deposit_id, admin_id, and reason are required", nil)
		return
	}

	h.logger.Info("Admin marking deposit as failed",
		zap.String("deposit_id", req.DepositID),
		zap.String("admin_id", req.AdminID))

	// TODO: Implement gRPC method for this
	// resp, err := h.cryptoClient.DepositClient.MarkDepositFailed(ctx, &cryptopb.MarkDepositFailedRequest{
	//     DepositId: req.DepositID,
	//     AdminId:   req.AdminID,
	//     Reason:    req.Reason,
	// })

	h.respondJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"success": false,
		"message": "Mark deposit as failed not yet implemented",
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func stringToDepositStatusEnum(status string) cryptopb.DepositStatus {
	if status == "" {
		return cryptopb.DepositStatus_DEPOSIT_STATUS_UNSPECIFIED
	}

	switch status {
	case "DETECTED":
		return cryptopb.DepositStatus_DEPOSIT_STATUS_DETECTED
	case "PENDING":
		return cryptopb.DepositStatus_DEPOSIT_STATUS_PENDING
	case "CONFIRMED":
		return cryptopb.DepositStatus_DEPOSIT_STATUS_CONFIRMED
	case "CREDITED":
		return cryptopb.DepositStatus_DEPOSIT_STATUS_CREDITED
	case "FAILED":
		return cryptopb.DepositStatus_DEPOSIT_STATUS_FAILED
	default:
		return cryptopb.DepositStatus_DEPOSIT_STATUS_UNSPECIFIED
	}
}

type DepositStats struct {
	Total           int                        `json:"total"`
	ByStatus        map[string]int             `json:"by_status"`
	ByChain         map[string]int             `json:"by_chain"`
	ByAsset         map[string]int             `json:"by_asset"`
	PendingCount    int                        `json:"pending_count"`
	ConfirmedCount  int                        `json:"confirmed_count"`
	CreditedCount   int                        `json:"credited_count"`
	FailedCount     int                        `json:"failed_count"`
	TotalAmount     map[string]string          `json:"total_amount"` // By asset
}

func calculateDepositStats(deposits []*cryptopb.Deposit) *DepositStats {
	stats := &DepositStats{
		Total:       len(deposits),
		ByStatus:    make(map[string]int),
		ByChain:     make(map[string]int),
		ByAsset:     make(map[string]int),
		TotalAmount: make(map[string]string),
	}

	for _, deposit := range deposits {
		// By status
		statusStr := deposit.Status.String()
		stats.ByStatus[statusStr]++

		// Count by specific statuses
		switch deposit.Status {
		case cryptopb.DepositStatus_DEPOSIT_STATUS_PENDING:
			stats.PendingCount++
		case cryptopb.DepositStatus_DEPOSIT_STATUS_CONFIRMED:
			stats.ConfirmedCount++
		case cryptopb.DepositStatus_DEPOSIT_STATUS_CREDITED:
			stats.CreditedCount++
		case cryptopb.DepositStatus_DEPOSIT_STATUS_FAILED:
			stats.FailedCount++
		}

		// By chain
		chainStr := deposit.Chain.String()
		stats.ByChain[chainStr]++

		// By asset
		stats.ByAsset[deposit.Asset]++

		// Total amount by asset
		if deposit.Amount != nil {
			stats.TotalAmount[deposit.Asset] = deposit.Amount.Amount
		}
	}

	return stats
}
// internal/handler/admin_crypto_transaction_handler.go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	cryptopb "x/shared/genproto/shared/accounting/cryptopb"
	"x/shared/response"

	"go.uber.org/zap"
)

// ============================================================================
// TRANSACTION QUERIES
// ============================================================================

// GetTransaction retrieves a specific transaction by ID
// GET /admin/svc/crypto/transactions/{transaction_id}
func (h *AdminHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	transactionID := r.URL.Query().Get("transaction_id")
	if transactionID == "" {
		h.respondError(w, http.StatusBadRequest, "transaction_id is required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.GetTransaction(ctx, &cryptopb.GetTransactionRequest{
		TransactionId: transactionID,
	})
	if err != nil {
		h.logger.Error("Failed to get transaction",
			zap.String("transaction_id", transactionID),
			zap.Error(err))
		h.respondError(w, http.StatusNotFound, "Transaction not found", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"transaction": resp.Transaction,
		},
	})
}

// GetUserTransactions retrieves transactions for a specific user
// GET /admin/svc/crypto/transactions/user
func (h *AdminHandler) GetUserTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.respondError(w, http.StatusBadRequest, "user_id is required", nil)
		return
	}

	// Optional filters
	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	txType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	// Pagination
	page := parseIntQuery(r, "page", 1)
	pageSize := parseIntQuery(r, "page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.GetUserTransactions(ctx, &cryptopb.GetUserTransactionsRequest{
		UserId: userID,
		Chain:  stringToChainEnum(chain),
		Asset:  asset,
		Type:   stringToTransactionTypeEnum(txType),
		Status: stringToTransactionStatusEnum(status),
		Pagination: &cryptopb.PaginationRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	})
	if err != nil {
		h.logger.Error("Failed to get user transactions",
			zap.String("user_id", userID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get transactions", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"transactions": resp.Transactions,
			"pagination":   resp.Pagination,
		},
	})
}

// GetTransactionStatus gets the current status of a transaction
// GET /admin/svc/crypto/transactions/{transaction_id}/status
func (h *AdminHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	transactionID := r.URL.Query().Get("transaction_id")
	if transactionID == "" {
		h.respondError(w, http.StatusBadRequest, "transaction_id is required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.GetTransactionStatus(ctx, &cryptopb.GetTransactionStatusRequest{
		TransactionId: transactionID,
	})
	if err != nil {
		h.logger.Error("Failed to get transaction status",
			zap.String("transaction_id", transactionID),
			zap.Error(err))
		h.respondError(w, http.StatusNotFound, "Transaction not found", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"status":                  resp.Status.String(),
			"confirmations":           resp.Confirmations,
			"required_confirmations":  resp.RequiredConfirmations,
			"tx_hash":                 resp.TxHash,
			"status_message":          resp.StatusMessage,
			"updated_at":              resp.UpdatedAt,
		},
	})
}

// ============================================================================
// WITHDRAWAL APPROVALS (Admin Interventions)
// ============================================================================

// GetPendingWithdrawals retrieves all pending withdrawal approvals
// GET /admin/svc/crypto/approvals/pending
func (h *AdminHandler) GetPendingWithdrawals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Pagination
	limit := parseIntQuery(r, "limit", 20)
	offset := parseIntQuery(r, "offset", 0)
	if limit > 100 {
		limit = 100
	}

	// Optional filters
	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	minRiskScore := parseIntQuery(r, "min_risk_score", 0)

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.GetPendingWithdrawals(ctx, &cryptopb.GetPendingWithdrawalsRequest{
		Limit:        int32(limit),
		Offset:       int32(offset),
		Chain:        chain,
		Asset:        asset,
		MinRiskScore: int32(minRiskScore),
	})
	if err != nil {
		h.logger.Error("Failed to get pending withdrawals", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get pending withdrawals", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"approvals": resp.Approvals,
			"total":     resp.Total,
		},
	})
}

// GetWithdrawalApproval gets details of a specific approval
// GET /admin/svc/crypto/approvals/{approval_id}
func (h *AdminHandler) GetWithdrawalApproval(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	approvalIDStr := r.URL.Query().Get("approval_id")
	if approvalIDStr == "" {
		h.respondError(w, http.StatusBadRequest, "approval_id is required", nil)
		return
	}

	approvalID, err := strconv.ParseInt(approvalIDStr, 10, 64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid approval_id", err)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.GetWithdrawalApproval(ctx, &cryptopb.GetWithdrawalApprovalRequest{
		ApprovalId: approvalID,
	})
	if err != nil {
		h.logger.Error("Failed to get approval",
			zap.Int64("approval_id", approvalID),
			zap.Error(err))
		h.respondError(w, http.StatusNotFound, "Approval not found", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"approval": resp.Approval,
		},
	})
}

// ApproveWithdrawal approves a pending withdrawal
// POST /admin/svc/crypto/approvals/{approval_id}/approve
func (h *AdminHandler) ApproveWithdrawal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, role, ok := h. getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ApprovalID int64  `json:"approval_id"`
		ApprovedBy string `json:"-"`
		Notes      string `json:"notes,omitempty"`
	}

	req.ApprovedBy = userID + " (" + role + ")"
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate
	if req.ApprovalID <= 0 {
		h.respondError(w, http.StatusBadRequest, "approval_id is required", nil)
		return
	}
	if req.ApprovedBy == "" {
		h.respondError(w, http.StatusBadRequest, "approved_by is required", nil)
		return
	}

	h.logger.Info("Admin approving withdrawal",
		zap.Int64("approval_id", req.ApprovalID),
		zap.String("approved_by", req.ApprovedBy))

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.ApproveWithdrawal(ctx, &cryptopb.ApproveWithdrawalRequest{
		ApprovalId: req.ApprovalID,
		ApprovedBy: req.ApprovedBy,
		Notes:      req.Notes,
	})
	if err != nil {
		h.logger.Error("Failed to approve withdrawal",
			zap.Int64("approval_id", req.ApprovalID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to approve withdrawal", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"message":     resp.Message,
			"transaction": resp.Transaction,
		},
	})
}

// RejectWithdrawal rejects a pending withdrawal
// POST /admin/svc/crypto/approvals/{approval_id}/reject
func (h *AdminHandler) RejectWithdrawal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, role, ok := h. getAdminContext(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ApprovalID      int64  `json:"approval_id"`
		RejectedBy      string `json:"-"`
		RejectionReason string `json:"rejection_reason"`
	}
	req.RejectedBy = userID + " (" + role + ")"

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate
	if req.ApprovalID <= 0 {
		h.respondError(w, http.StatusBadRequest, "approval_id is required", nil)
		return
	}
	if req.RejectedBy == "" {
		h.respondError(w, http.StatusBadRequest, "rejected_by is required", nil)
		return
	}
	if req.RejectionReason == "" {
		h.respondError(w, http.StatusBadRequest, "rejection_reason is required", nil)
		return
	}

	h.logger.Info("Admin rejecting withdrawal",
		zap.Int64("approval_id", req.ApprovalID),
		zap.String("rejected_by", req.RejectedBy))

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.RejectWithdrawal(ctx, &cryptopb.RejectWithdrawalRequest{
		ApprovalId:      req.ApprovalID,
		RejectedBy:      req.RejectedBy,
		RejectionReason: req.RejectionReason,
	})
	if err != nil {
		h.logger.Error("Failed to reject withdrawal",
			zap.Int64("approval_id", req.ApprovalID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to reject withdrawal", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"message": resp.Message,
		},
	})
}

// ============================================================================
// SWEEP OPERATIONS (System Maintenance)
// ============================================================================

// SweepUserWallet sweeps a specific user's wallet to system wallet
// POST /admin/svc/crypto/sweep/user
func (h *AdminHandler) SweepUserWallet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		UserID string `json:"user_id"`
		Chain  string `json:"chain"`
		Asset  string `json:"asset"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate
	if req.UserID == "" || req.Chain == "" || req.Asset == "" {
		h.respondError(w, http.StatusBadRequest, "user_id, chain, and asset are required", nil)
		return
	}

	h.logger.Info("Sweeping user wallet",
		zap.String("user_id", req.UserID),
		zap.String("chain", req.Chain),
		zap.String("asset", req.Asset))

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.SweepUserWallet(ctx, &cryptopb.SweepUserWalletRequest{
		UserId: req.UserID,
		Chain:  stringToChainEnum(req.Chain),
		Asset:  req.Asset,
	})
	if err != nil {
		h.logger.Error("Failed to sweep wallet",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to sweep wallet", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"message":     resp.Message,
			"transaction": resp.Transaction,
		},
	})
}

// SweepAllUsers sweeps all user wallets for a specific chain/asset
// POST /admin/svc/crypto/sweep/all
func (h *AdminHandler) SweepAllUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Chain string `json:"chain"`
		Asset string `json:"asset"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate
	if req.Chain == "" || req.Asset == "" {
		h.respondError(w, http.StatusBadRequest, "chain and asset are required", nil)
		return
	}

	h.logger.Info("Sweeping all user wallets",
		zap.String("chain", req.Chain),
		zap.String("asset", req.Asset))

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.SweepAllUsers(ctx, &cryptopb.SweepAllUsersRequest{
		Chain: stringToChainEnum(req.Chain),
		Asset: req.Asset,
	})
	if err != nil {
		h.logger.Error("Failed to sweep all wallets",
			zap.String("chain", req.Chain),
			zap.String("asset", req.Asset),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to sweep wallets", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"message":       resp.Message,
			"transactions":  resp.Transactions,
			"success_count": resp.SuccessCount,
			"failed_count":  resp.FailedCount,
		},
	})
}

// ============================================================================
// FEE ESTIMATION (For Admin Reference)
// ============================================================================

// EstimateNetworkFee estimates network fee for a withdrawal
// GET /admin/svc/crypto/fees/estimate
func (h *AdminHandler) EstimateNetworkFee(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	amount := r.URL.Query().Get("amount")
	toAddress := r.URL.Query().Get("to_address")

	if chain == "" || asset == "" || amount == "" {
		h.respondError(w, http.StatusBadRequest, "chain, asset, and amount are required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.TransactionClient.EstimateNetworkFee(ctx, &cryptopb.EstimateNetworkFeeRequest{
		Chain:     stringToChainEnum(chain),
		Asset:     asset,
		Amount:    amount,
		ToAddress: toAddress,
	})
	if err != nil {
		h.logger.Error("Failed to estimate fee",
			zap.String("chain", chain),
			zap.String("asset", asset),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to estimate fee", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"fee_amount":    resp.FeeAmount,
			"fee_currency":  resp.FeeCurrency,
			"fee_formatted": resp.FeeFormatted,
			"estimated_at":  resp.EstimatedAt,
			"valid_for":     resp.ValidFor,
			"explanation":   resp.Explanation,
		},
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

func stringToTransactionTypeEnum(txType string) cryptopb.TransactionType {
	if txType == "" {
		return cryptopb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}

	switch txType {
	case "DEPOSIT":
		return cryptopb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case "WITHDRAWAL":
		return cryptopb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case "SWEEP":
		return cryptopb.TransactionType_TRANSACTION_TYPE_SWEEP
	case "INTERNAL_TRANSFER":
		return cryptopb.TransactionType_TRANSACTION_TYPE_INTERNAL_TRANSFER
	default:
		return cryptopb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func stringToTransactionStatusEnum(status string) cryptopb.TransactionStatus {
	if status == "" {
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}

	switch status {
	case "PENDING":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_PENDING
	case "BROADCASTING":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_BROADCASTING
	case "BROADCASTED":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_BROADCASTED
	case "CONFIRMING":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_CONFIRMING
	case "CONFIRMED":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
	case "COMPLETED":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case "FAILED":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_FAILED
	case "CANCELLED":
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_CANCELLED
	default:
		return cryptopb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}
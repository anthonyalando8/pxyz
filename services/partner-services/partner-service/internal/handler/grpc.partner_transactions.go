// handler/grpc_partner_transactions. go
package handler

import (
	"context"
	"log"
	"partner-service/internal/domain"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// DEPOSIT OPERATIONS
// ============================================================================

// InitiateDeposit creates a new deposit transaction
func (h *GRPCPartnerHandler) InitiateDeposit(
	ctx context.Context,
	req *partnersvcpb.InitiateDepositRequest,
) (*partnersvcpb.InitiateDepositResponse, error) {
	if req.PartnerId == "" || req.TransactionRef == "" || req.UserId == "" {
		return nil, status.Errorf(codes. InvalidArgument, "partner_id, transaction_ref, and user_id are required")
	}

	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "amount must be greater than 0")
	}

	if req.Currency == "" {
		return nil, status.Errorf(codes.InvalidArgument, "currency is required")
	}

	txn := &domain. PartnerTransaction{
		PartnerID:      req.PartnerId,
		TransactionRef:  req.TransactionRef,
		UserID:         req.UserId,
		Amount:         req. Amount,
		Currency:       req.Currency,
		PaymentMethod:  &req.PaymentMethod,
		ExternalRef:    &req.ExternalRef,
		Metadata:       convertMetadata(req.Metadata),
	}

	if err := h. uc.InitiateDeposit(ctx, txn); err != nil {
		log.Printf("[ERROR] InitiateDeposit failed: %v", err)
		return &partnersvcpb.InitiateDepositResponse{
			Success:        false,
			TransactionId: 0,
			TransactionRef: req.TransactionRef,
			Status:        "failed",
			Message:       err.Error(),
		}, nil
	}

	return &partnersvcpb.InitiateDepositResponse{
		Success:        true,
		TransactionId:   txn.ID,
		TransactionRef: txn.TransactionRef,
		Status:         txn.Status,
		Message:        "Deposit initiated successfully",
		CreatedAt:      timestamppb.New(txn.CreatedAt),
	}, nil
}

// ============================================================================
// WITHDRAWAL OPERATIONS
// ============================================================================

// InitiateWithdrawal creates a new withdrawal transaction
func (h *GRPCPartnerHandler) InitiateWithdrawal(
	ctx context.Context,
	req *partnersvcpb.InitiateWithdrawalRequest,
) (*partnersvcpb.InitiateWithdrawalResponse, error) {
	// Validation
	if req.PartnerId == "" || req.TransactionRef == "" || req.UserId == "" {
		return nil, status. Errorf(codes.InvalidArgument, "partner_id, transaction_ref, and user_id are required")
	}

	if req.Amount <= 0 {
		return nil, status. Errorf(codes.InvalidArgument, "amount must be greater than 0")
	}

	if req.Currency == "" {
		return nil, status.Errorf(codes.InvalidArgument, "currency is required")
	}

	if req.PaymentMethod == "" {
		return nil, status. Errorf(codes.InvalidArgument, "payment_method is required for withdrawal")
	}

	txn := &domain.PartnerTransaction{
		PartnerID:      req.PartnerId,
		TransactionRef: req. TransactionRef,
		UserID:         req.UserId,
		Amount:         req.Amount,
		Currency:       req.Currency,
		PaymentMethod:  &req.PaymentMethod,
		ExternalRef:    &req.ExternalRef,
		Metadata:       convertMetadata(req.Metadata),
	}

	if err := h.uc.InitiateWithdrawal(ctx, txn); err != nil {
		log.Printf("[ERROR] InitiateWithdrawal failed: %v", err)
		return &partnersvcpb.InitiateWithdrawalResponse{
			Success:        false,
			TransactionId:   0,
			TransactionRef: req.TransactionRef,
			Status:         "failed",
			Message:        err.Error(),
		}, nil
	}

	return &partnersvcpb.InitiateWithdrawalResponse{
		Success:        true,
		TransactionId:  txn.ID,
		TransactionRef: txn.TransactionRef,
		Status:         txn.Status,
		Message:        "Withdrawal initiated successfully",
		CreatedAt:      timestamppb.New(txn. CreatedAt),
	}, nil
}

// ============================================================================
// TRANSACTION QUERIES
// ============================================================================

// GetTransactionByRef retrieves a single transaction by reference
func (h *GRPCPartnerHandler) GetTransactionByRef(
	ctx context.Context,
	req *partnersvcpb.GetTransactionByRefRequest,
) (*partnersvcpb.TransactionResponse, error) {
	if req.PartnerId == "" || req.TransactionRef == "" {
		return nil, status. Errorf(codes.InvalidArgument, "partner_id and transaction_ref are required")
	}

	txn, err := h.uc.GetTransactionByRef(ctx, req.PartnerId, req.TransactionRef)
	if err != nil {
		log.Printf("[ERROR] GetTransactionByRef failed: %v", err)
		return nil, status.Errorf(codes. NotFound, "transaction not found")
	}

	return &partnersvcpb.TransactionResponse{
		Transaction: txnToProto(txn),
	}, nil
}

// GetTransactionStatus retrieves transaction status (alias for GetTransactionByRef)
func (h *GRPCPartnerHandler) GetTransactionStatus(
	ctx context.Context,
	req *partnersvcpb.GetTransactionStatusRequest,
) (*partnersvcpb.TransactionStatusResponse, error) {
	if req.PartnerId == "" || req.TransactionRef == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id and transaction_ref are required")
	}

	txn, err := h.uc.GetTransactionStatus(ctx, req. PartnerId, req.TransactionRef)
	if err != nil {
		log.Printf("[ERROR] GetTransactionStatus failed: %v", err)
		return nil, status.Errorf(codes.NotFound, "transaction not found")
	}

	return &partnersvcpb.TransactionStatusResponse{
		Transaction: txnToProto(txn),
	}, nil
}

// ListTransactions returns paginated transactions
func (h *GRPCPartnerHandler) ListTransactions(
	ctx context.Context,
	req *partnersvcpb.ListTransactionsRequest,
) (*partnersvcpb.ListTransactionsResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	// Set defaults
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	var statusFilter *string
	if req.Status != "" {
		statusFilter = &req. Status
	}

	// Convert timestamps
	var from, to *time.Time
	if req.From != nil {
		fromTime := req.From.AsTime()
		from = &fromTime
	}
	if req.To != nil {
		toTime := req.To.AsTime()
		to = &toTime
	}

	txns, total, err := h. uc.ListTransactions(ctx, req.PartnerId, limit, offset, statusFilter, from, to)
	if err != nil {
		log.Printf("[ERROR] ListTransactions failed: %v", err)
		return nil, status.Errorf(codes. Internal, "failed to list transactions")
	}

	protoTxns := make([]*partnersvcpb.PartnerTransaction, 0, len(txns))
	for _, txn := range txns {
		protoTxns = append(protoTxns, txnToProto(&txn))
	}

	return &partnersvcpb.ListTransactionsResponse{
		Transactions: protoTxns,
		TotalCount:   total,
	}, nil
}

// ListTransactionsByType returns transactions filtered by type
func (h *GRPCPartnerHandler) ListTransactionsByType(
	ctx context.Context,
	req *partnersvcpb.ListTransactionsByTypeRequest,
) (*partnersvcpb.ListTransactionsResponse, error) {
	if req.PartnerId == "" {
		return nil, status. Errorf(codes.InvalidArgument, "partner_id is required")
	}

	if req.TransactionType == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction_type is required")
	}

	// Validate transaction type
	if req.TransactionType != "deposit" && req.TransactionType != "withdrawal" {
		return nil, status.Errorf(codes. InvalidArgument, "transaction_type must be 'deposit' or 'withdrawal'")
	}

	// Set defaults
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	offset := int(req. Offset)
	if offset < 0 {
		offset = 0
	}

	txns, total, err := h. uc.ListTransactionsByType(ctx, req.PartnerId, req.TransactionType, limit, offset)
	if err != nil {
		log.Printf("[ERROR] ListTransactionsByType failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list transactions")
	}

	protoTxns := make([]*partnersvcpb. PartnerTransaction, 0, len(txns))
	for _, txn := range txns {
		protoTxns = append(protoTxns, txnToProto(&txn))
	}

	return &partnersvcpb.ListTransactionsResponse{
		Transactions:  protoTxns,
		TotalCount:   total,
	}, nil
}

// ============================================================================
// TRANSACTION MANAGEMENT
// ============================================================================

// CancelTransaction cancels a pending transaction
func (h *GRPCPartnerHandler) CancelTransaction(
	ctx context.Context,
	req *partnersvcpb.CancelTransactionRequest,
) (*partnersvcpb.CancelTransactionResponse, error) {
	if req.PartnerId == "" || req.TransactionRef == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id and transaction_ref are required")
	}

	// Get transaction before cancelling
	txn, err := h.uc.GetTransactionByRef(ctx, req.PartnerId, req.TransactionRef)
	if err != nil {
		log.Printf("[ERROR] CancelTransaction - transaction not found: %v", err)
		return nil, status. Errorf(codes.NotFound, "transaction not found")
	}

	// Cancel the transaction
	if err := h.uc.CancelTransaction(ctx, req.PartnerId, req.TransactionRef); err != nil {
		log.Printf("[ERROR] CancelTransaction failed: %v", err)
		return &partnersvcpb.CancelTransactionResponse{
			Success:  false,
			Message: err. Error(),
		}, nil
	}

	// Get updated transaction
	updatedTxn, _ := h.uc.GetTransactionByRef(ctx, req. PartnerId, req.TransactionRef)
	if updatedTxn != nil {
		txn = updatedTxn
	}

	return &partnersvcpb.CancelTransactionResponse{
		Success:      true,
		Message:     "Transaction cancelled successfully",
		Transaction: txnToProto(txn),
	}, nil
}

// GetTransactionStats returns transaction statistics
func (h *GRPCPartnerHandler) GetTransactionStats(
	ctx context.Context,
	req *partnersvcpb.GetTransactionStatsRequest,
) (*partnersvcpb.GetTransactionStatsResponse, error) {
	if req.PartnerId == "" {
		return nil, status. Errorf(codes.InvalidArgument, "partner_id is required")
	}

	// Set defaults
	from := time.Now().AddDate(0, -1, 0) // Default:  last month
	to := time.Now()

	if req.From != nil {
		from = req.From.AsTime()
	}
	if req. To != nil {
		to = req.To.AsTime()
	}

	stats, err := h.uc.GetTransactionStats(ctx, req.PartnerId, from, to)
	if err != nil {
		log.Printf("[ERROR] GetTransactionStats failed: %v", err)
		return nil, status.Errorf(codes. Internal, "failed to get transaction stats")
	}

	return &partnersvcpb.GetTransactionStatsResponse{
		TotalCount:        stats["total_count"].(int64),
		CompletedCount:   stats["completed_count"].(int64),
		FailedCount:      stats["failed_count"].(int64),
		PendingCount:     stats["pending_count"].(int64),
		DepositCount:     stats["deposit_count"].(int64),
		WithdrawalCount:  stats["withdrawal_count"].(int64),
		TotalAmount:      stats["total_amount"].(float64),
		TotalDeposits:    stats["total_deposits"].(float64),
		TotalWithdrawals: stats["total_withdrawals"].(float64),
		AvgAmount:        stats["avg_amount"].(float64),
		MinAmount:        stats["min_amount"].(float64),
		MaxAmount:        stats["max_amount"].(float64),
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// txnToProto converts domain transaction to protobuf
func txnToProto(txn *domain.PartnerTransaction) *partnersvcpb.PartnerTransaction {
	proto := &partnersvcpb. PartnerTransaction{
		Id:              txn.ID,
		PartnerId:       txn. PartnerID,
		TransactionRef:  txn.TransactionRef,
		UserId:          txn.UserID,
		Amount:          txn.Amount,
		Currency:        txn.Currency,
		Status:          txn.Status,
		TransactionType: txn.TransactionType,
		ErrorMessage:    StringValue(txn.ErrorMessage),
		Metadata:        convertMetadataToProto(txn. Metadata),
		CreatedAt:       timestamppb.New(txn.CreatedAt),
		UpdatedAt:       timestamppb.New(txn.UpdatedAt),
	}

	if txn.PaymentMethod != nil {
		proto.PaymentMethod = *txn.PaymentMethod
	}
	if txn.ExternalRef != nil {
		proto.ExternalRef = *txn.ExternalRef
	}
	if txn.ProcessedAt != nil {
		proto.ProcessedAt = timestamppb.New(*txn.ProcessedAt)
	}

	return proto
}

// convertMetadata converts protobuf metadata to domain metadata
func convertMetadata(meta map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range meta {
		result[k] = v
	}
	return result
}

// convertMetadataToProto converts domain metadata to protobuf metadata
func convertMetadataToProto(meta map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range meta {
		if str, ok := v.(string); ok {
			result[k] = str
		} else {
			result[k] = "" // Or convert to string if needed
		}
	}
	return result
}
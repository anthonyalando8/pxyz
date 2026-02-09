// internal/handler/transaction_handler.go
package handler

import (
	"context"
	"crypto-service/internal/domain"
	"crypto-service/internal/usecase"

	//"fmt"

	pb "x/shared/genproto/shared/accounting/cryptopb"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TransactionHandler struct {
	pb.UnimplementedTransactionServiceServer
	transactionUsecase *usecase.TransactionUsecase
	logger             *zap.Logger
}

func NewTransactionHandler(
	transactionUsecase *usecase.TransactionUsecase,
	logger *zap.Logger,
) *TransactionHandler {
	return &TransactionHandler{
		transactionUsecase: transactionUsecase,
		logger:             logger,
	}
}

// ============================================================================
// NETWORK FEE ESTIMATION
// ============================================================================

// EstimateNetworkFee estimates blockchain network fee
func (h *TransactionHandler) EstimateNetworkFee(
	ctx context.Context,
	req *pb.EstimateNetworkFeeRequest,
) (*pb.EstimateNetworkFeeResponse, error) {


	// Validate
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED || req.Asset == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "chain, asset, and amount required")
	}

	chainName := chainEnumToString(req.Chain)

	// Estimate fee
	estimate, err := h.transactionUsecase.EstimateNetworkFee(
		ctx,
		chainName,
		req.Asset,
		req.Amount,
		req.ToAddress,
	)
	if err != nil {
		h.logger.Error("Failed to estimate network fee", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to estimate fee: %v", err)
	}

	return &pb.EstimateNetworkFeeResponse{
		Chain:        req.Chain,
		Asset:        req.Asset,
		FeeAmount:    estimate.FeeAmount.String(),
		FeeCurrency:  estimate.FeeCurrency,
		FeeFormatted: estimate.FeeFormatted,
		EstimatedAt:  timestamppb.New(estimate.EstimatedAt),
		ValidFor:     int32(estimate.ValidFor.Seconds()),
		Explanation:  "Estimated blockchain network fee",
	}, nil
}

// GetWithdrawalQuote gets complete withdrawal quote
func (h *TransactionHandler) GetWithdrawalQuote(
	ctx context.Context,
	req *pb.GetWithdrawalQuoteRequest,
) (*pb.GetWithdrawalQuoteResponse, error) {



	// Validate
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED || req.Asset == "" || req.Amount == "" || req.ToAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}

	chainName := chainEnumToString(req.Chain)

	// Get quote
	quote, err := h.transactionUsecase.GetWithdrawalQuote(
		ctx,
		chainName,
		req.Asset,
		req.Amount,
		req.ToAddress,
	)
	if err != nil {
		h.logger.Error("Failed to get quote", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get quote: %v", err)
	}

	return &pb.GetWithdrawalQuoteResponse{
		QuoteId: quote.QuoteID,
		Chain:   req.Chain,
		Asset:   req.Asset,
		Amount: &pb.Money{
			Amount:   quote.Amount.String(),
			Currency: req.Asset,
		},
		NetworkFee: &pb.Money{
			Amount:   quote.NetworkFee.String(),
			Currency: quote.NetworkFeeCurrency,
		},
		NetworkFeeCurrency: quote.NetworkFeeCurrency,
		ValidUntil:         timestamppb.New(quote.ValidUntil),
		Explanation:        quote.Explanation,
	}, nil
}

// ============================================================================
// WITHDRAWAL
// ============================================================================

// Withdraw executes withdrawal from system hot wallet to external address
func (h *TransactionHandler) Withdraw(
	ctx context.Context,
	req *pb.WithdrawRequest,
) (*pb.WithdrawResponse, error) {

	h.logger.Info("Withdraw request",
		zap.String("accounting_tx_id", req.AccountingTxId),
		zap.String("user_id", req.UserId),
		zap.String("chain", req.Chain.String()),
		zap.String("asset", req.Asset),
		zap.String("amount", req.Amount),
		zap.String("to", req.ToAddress))

	// Validate request
	if req.AccountingTxId == "" {
		return nil, status.Error(codes.InvalidArgument, "accounting_tx_id required for idempotency")
	}
	if req.UserId == "" || req.Asset == "" || req.Amount == "" || req.ToAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "chain required")
	}

	chainName := chainEnumToString(req.Chain)

	// Execute withdrawal
	tx, err := h.transactionUsecase.Withdraw(
		ctx,
		req.AccountingTxId,
		chainName,
		req.Asset,
		req.Amount,
		req.ToAddress,
		req.Memo,
		req.UserId,
	)
	if err != nil {
		h.logger.Error("Withdrawal failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "withdrawal failed: %v", err)
	}

	return &pb.WithdrawResponse{
		Transaction: h.transactionToProto(tx),
		Message:     "Withdrawal initiated successfully",
	}, nil
}

// ============================================================================
// SWEEP OPERATIONS
// ============================================================================

// SweepUserWallet sweeps funds from user's deposit address to system wallet
func (h *TransactionHandler) SweepUserWallet(
	ctx context.Context,
	req *pb.SweepUserWalletRequest,
) (*pb.SweepUserWalletResponse, error) {


	// Validate
	if req.UserId == "" || req.Chain == pb.Chain_CHAIN_UNSPECIFIED || req.Asset == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}

	chainName := chainEnumToString(req.Chain)

	// Execute sweep
	tx, err := h.transactionUsecase.SweepUserWallet(ctx, req.UserId, chainName, req.Asset)
	if err != nil {
		h.logger.Error("Sweep failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "sweep failed: %v", err)
	}

	return &pb.SweepUserWalletResponse{
		Transaction: h.transactionToProto(tx),
		Message:     "Wallet swept successfully",
	}, nil
}

// SweepAllUsers sweeps all user wallets for a chain/asset
func (h *TransactionHandler) SweepAllUsers(
	ctx context.Context,
	req *pb.SweepAllUsersRequest,
) (*pb.SweepAllUsersResponse, error) {

	h.logger.Info("SweepAllUsers request",
		zap.String("chain", req.Chain.String()),
		zap.String("asset", req.Asset))

	// Validate
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED || req.Asset == "" {
		return nil, status.Error(codes.InvalidArgument, "chain and asset required")
	}

	chainName := chainEnumToString(req.Chain)

	// Execute batch sweep
	transactions, err := h.transactionUsecase.SweepAllUsers(ctx, chainName, req.Asset)
	if err != nil {
		h.logger.Error("Batch sweep failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "batch sweep failed: %v", err)
	}

	// Convert to proto
	pbTransactions := make([]*pb.Transaction, len(transactions))
	for i, tx := range transactions {
		pbTransactions[i] = h.transactionToProto(tx)
	}

	return &pb.SweepAllUsersResponse{
		Transactions: pbTransactions,
		SuccessCount: int32(len(transactions)),
		FailedCount:  0,
		Message:      "Batch sweep completed",
	}, nil
}

// ============================================================================
// TRANSACTION QUERIES
// ============================================================================

// GetTransaction retrieves transaction by ID
func (h *TransactionHandler) GetTransaction(
	ctx context.Context,
	req *pb.GetTransactionRequest,
) (*pb.GetTransactionResponse, error) {

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id required")
	}

	tx, err := h.transactionUsecase.GetTransaction(ctx, req.TransactionId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "transaction not found: %v", err)
	}

	return &pb.GetTransactionResponse{
		Transaction: h.transactionToProto(tx),
	}, nil
}

// GetUserTransactions retrieves user's transaction history
func (h *TransactionHandler) GetUserTransactions(
	ctx context.Context,
	req *pb.GetUserTransactionsRequest,
) (*pb.GetUserTransactionsResponse, error) {

	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Set defaults
	limit := int(req.Pagination.PageSize)
	if limit == 0 || limit > 100 {
		limit = 20
	}

	page := int(req.Pagination.Page)
	if page == 0 {
		page = 1
	}

	offset := (page - 1) * limit

	// Get transactions
	transactions, err := h.transactionUsecase.GetUserTransactions(ctx, req.UserId, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get transactions", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get transactions: %v", err)
	}

	// Convert to summaries
	summaries := make([]*pb.TransactionSummary, len(transactions))
	for i, tx := range transactions {
		summaries[i] = h.transactionToSummaryProto(tx)
	}

	return &pb.GetUserTransactionsResponse{
		Transactions: summaries,
		Pagination: &pb.PaginationResponse{
			Page:       int32(page),
			PageSize:   int32(limit),
			Total:      int64(len(summaries)),
			TotalPages: int32((len(summaries) + limit - 1) / limit),
		},
	}, nil
}

// GetTransactionStatus gets transaction status
func (h *TransactionHandler) GetTransactionStatus(
	ctx context.Context,
	req *pb.GetTransactionStatusRequest,
) (*pb.GetTransactionStatusResponse, error) {

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id required")
	}

	tx, err := h.transactionUsecase.GetTransaction(ctx, req.TransactionId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "transaction not found: %v", err)
	}

	return &pb.GetTransactionStatusResponse{
		Status:                transactionStatusToProto(tx.Status),
		Confirmations:         int32(tx.Confirmations),
		RequiredConfirmations: int32(tx.RequiredConfirmations),
		TxHash:                getStringValue(tx.TxHash),
		StatusMessage:         getStringValue(tx.StatusMessage),
		UpdatedAt:             timestamppb.New(tx.UpdatedAt),
	}, nil
}

// ============================================================================
// APPROVAL METHODS
// ============================================================================

// GetPendingWithdrawals retrieves pending withdrawal approvals
func (h *TransactionHandler) GetPendingWithdrawals(
	ctx context.Context,
	req *pb.GetPendingWithdrawalsRequest,
) (*pb.GetPendingWithdrawalsResponse, error) {

	h.logger.Info("Getting pending withdrawal approvals",
		zap.Int32("limit", req.Limit),
		zap.Int32("offset", req.Offset))

	// Set defaults
	limit := int(req.Limit)
	offset := int(req.Offset)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Get pending approvals
	approvals, err := h.transactionUsecase.GetPendingApprovals(ctx, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get pending approvals", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get pending approvals: %v", err)
	}

	// Get total count
	stats, err := h.transactionUsecase.GetApprovalStats(ctx)
	totalCount := int32(len(approvals))
	if err == nil && stats != nil {
		totalCount = int32(stats.Pending)
	}

	// Convert to protobuf
	pbApprovals := make([]*pb.WithdrawalApprovalInfo, len(approvals))
	for i, approval := range approvals {
		pbApprovals[i] = h.approvalToProto(approval)
	}

	return &pb.GetPendingWithdrawalsResponse{
		Approvals: pbApprovals,
		Total:     totalCount,
	}, nil
}

// ApproveWithdrawal approves a pending withdrawal
func (h *TransactionHandler) ApproveWithdrawal(
	ctx context.Context,
	req *pb.ApproveWithdrawalRequest,
) (*pb.ApproveWithdrawalResponse, error) {

	h.logger.Info("Approving withdrawal",
		zap.Int64("approval_id", req.ApprovalId),
		zap.String("approved_by", req.ApprovedBy))

	// Validate request
	if req.ApprovalId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid approval_id")
	}
	if req.ApprovedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "approved_by is required")
	}

	// Approve withdrawal
	tx, err := h.transactionUsecase.ApproveWithdrawal(
		ctx,
		req.ApprovalId,
		req.ApprovedBy,
		req.Notes,
	)
	if err != nil {
		h.logger.Error("Failed to approve withdrawal",
			zap.Int64("approval_id", req.ApprovalId),
			zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to approve withdrawal: %v", err)
	}

	h.logger.Info("Withdrawal approved successfully",
		zap.Int64("approval_id", req.ApprovalId),
		zap.String("tx_id", tx.TransactionID))

	return &pb.ApproveWithdrawalResponse{
		Success:     true,
		Message:     "Withdrawal approved and executing on blockchain",
		Transaction: h.transactionToProto(tx),
	}, nil
}

// RejectWithdrawal rejects a pending withdrawal
func (h *TransactionHandler) RejectWithdrawal(
	ctx context.Context,
	req *pb.RejectWithdrawalRequest,
) (*pb.RejectWithdrawalResponse, error) {

	h.logger.Info("Rejecting withdrawal",
		zap.Int64("approval_id", req.ApprovalId),
		zap.String("rejected_by", req.RejectedBy))

	// Validate request
	if req.ApprovalId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid approval_id")
	}
	if req.RejectedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "rejected_by is required")
	}
	if req.RejectionReason == "" {
		return nil, status.Error(codes.InvalidArgument, "rejection_reason is required")
	}

	// Reject withdrawal
	err := h.transactionUsecase.RejectWithdrawal(
		ctx,
		req.ApprovalId,
		req.RejectedBy,
		req.RejectionReason,
	)
	if err != nil {
		h.logger.Error("Failed to reject withdrawal",
			zap.Int64("approval_id", req.ApprovalId),
			zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to reject withdrawal: %v", err)
	}

	h.logger.Info("Withdrawal rejected successfully",
		zap.Int64("approval_id", req.ApprovalId),
		zap.String("reason", req.RejectionReason))

	return &pb.RejectWithdrawalResponse{
		Success: true,
		Message: "Withdrawal rejected successfully",
	}, nil
}

// GetWithdrawalApproval gets a specific approval by ID
func (h *TransactionHandler) GetWithdrawalApproval(
	ctx context.Context,
	req *pb.GetWithdrawalApprovalRequest,
) (*pb.GetWithdrawalApprovalResponse, error) {

	h.logger.Debug("Getting withdrawal approval",
		zap.Int64("approval_id", req.ApprovalId))

	if req.ApprovalId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid approval_id")
	}

	// Get approval
	approval, err := h.transactionUsecase.GetApprovalByID(ctx, req.ApprovalId)
	if err != nil {
		h.logger.Error("Failed to get approval",
			zap.Int64("approval_id", req.ApprovalId),
			zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "approval not found: %v", err)
	}

	return &pb.GetWithdrawalApprovalResponse{
		Approval: h.approvalToProto(approval),
	}, nil
}

// ============================================================================
// CONVERSION HELPERS
// ============================================================================

// transactionToProto converts domain.CryptoTransaction to protobuf
func (h *TransactionHandler) transactionToProto(tx *domain.CryptoTransaction) *pb.Transaction {
	if tx == nil {
		return nil
	}

	pbTx := &pb.Transaction{
		Id:                    tx.ID,
		TransactionId:         tx.TransactionID,
		UserId:                tx.UserID,
		Type:                  transactionTypeToProto(tx.Type),
		Chain:                 stringToChainEnum(tx.Chain),
		Asset:                 tx.Asset,
		FromAddress:           tx.FromAddress,
		ToAddress:             tx.ToAddress,
		IsInternal:            tx.IsInternal,
		Amount:                &pb.Money{Amount: tx.Amount.String(), Currency: tx.Asset},
		Confirmations:         int32(tx.Confirmations),
		RequiredConfirmations: int32(tx.RequiredConfirmations),
		Status:                transactionStatusToProto(tx.Status),
		InitiatedAt:           timestamppb.New(tx.InitiatedAt),
		CreatedAt:             timestamppb.New(tx.CreatedAt),
	}

	// Network fee
	if tx.NetworkFee != nil {
		currency := getStringValue(tx.NetworkFeeCurrency)
		if currency == "" {
			currency = tx.Asset
		}
		pbTx.NetworkFee = &pb.Money{
			Amount:   tx.NetworkFee.String(),
			Currency: currency,
		}
	}

	// Optional fields
	if tx.AccountingTxID != nil {
		pbTx.AccountingTxId = *tx.AccountingTxID
	}
	if tx.TxHash != nil {
		pbTx.TxHash = *tx.TxHash
	}
	if tx.BlockNumber != nil {
		pbTx.BlockNumber = *tx.BlockNumber
	}
	if tx.StatusMessage != nil {
		pbTx.StatusMessage = *tx.StatusMessage
	}
	if tx.BroadcastedAt != nil {
		pbTx.BroadcastedAt = timestamppb.New(*tx.BroadcastedAt)
	}
	if tx.ConfirmedAt != nil {
		pbTx.ConfirmedAt = timestamppb.New(*tx.ConfirmedAt)
	}

	return pbTx
}

// transactionToSummaryProto converts to summary
func (h *TransactionHandler) transactionToSummaryProto(tx *domain.CryptoTransaction) *pb.TransactionSummary {
	summary := &pb.TransactionSummary{
		TransactionId: tx.TransactionID,
		Type:          transactionTypeToProto(tx.Type),
		Chain:         stringToChainEnum(tx.Chain),
		Asset:         tx.Asset,
		Amount:        tx.Amount.String(),
		Status:        transactionStatusToProto(tx.Status),
		IsInternal:    tx.IsInternal,
		CreatedAt:     timestamppb.New(tx.CreatedAt),
	}

	if tx.TxHash != nil {
		summary.TxHash = *tx.TxHash
	}
	if tx.NetworkFee != nil {
		summary.NetworkFee = tx.NetworkFee.String()
	}

	return summary
}

// approvalToProto converts domain.WithdrawalApproval to protobuf
func (h *TransactionHandler) approvalToProto(approval *domain.WithdrawalApproval) *pb.WithdrawalApprovalInfo {
	if approval == nil {
		return nil
	}

	// Convert risk factors
	riskFactors := make([]*pb.RiskFactor, len(approval.RiskFactors))
	for i, factor := range approval.RiskFactors {
		riskFactors[i] = &pb.RiskFactor{
			Factor:      factor.Factor,
			Description: factor.Description,
			Score:       int32(factor.Score),
		}
	}

	// Convert status
	status := pb.WithdrawalApprovalStatus_WITHDRAWAL_APPROVAL_STATUS_UNSPECIFIED
	switch approval.Status {
	case domain.ApprovalStatusPendingReview:
		status = pb.WithdrawalApprovalStatus_WITHDRAWAL_APPROVAL_STATUS_PENDING_REVIEW
	case domain.ApprovalStatusApproved:
		status = pb.WithdrawalApprovalStatus_WITHDRAWAL_APPROVAL_STATUS_APPROVED
	case domain.ApprovalStatusRejected:
		status = pb.WithdrawalApprovalStatus_WITHDRAWAL_APPROVAL_STATUS_REJECTED
	case domain.ApprovalStatusAutoApproved:
		status = pb.WithdrawalApprovalStatus_WITHDRAWAL_APPROVAL_STATUS_AUTO_APPROVED
	}

	return &pb.WithdrawalApprovalInfo{
		Id:               approval.ID,
		TransactionId:    approval.TransactionID,
		UserId:           approval.UserID,
		Amount:           &pb.Money{Amount: approval.Amount.String(), Currency: approval.Asset},
		Asset:            approval.Asset,
		Chain:            pb.Chain_CHAIN_UNSPECIFIED, // TODO: Add chain to approval domain
		ToAddress:        approval.ToAddress,
		RiskScore:        int32(approval.RiskScore),
		RiskFactors:      riskFactors,
		RequiresApproval: approval.RequiresApproval,
		Status:           status,
		CreatedAt:        timestamppb.New(approval.CreatedAt),
	}
}

// ============================================================================
// ENUM CONVERSION HELPERS
// ============================================================================

func transactionTypeToProto(txType domain.TransactionType) pb.TransactionType {
	switch txType {
	case domain.TransactionTypeDeposit:
		return pb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case domain.TransactionTypeWithdrawal:
		return pb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case domain.TransactionTypeSweep:
		return pb.TransactionType_TRANSACTION_TYPE_SWEEP
	case domain.TransactionTypeInternalTransfer:
		return pb.TransactionType_TRANSACTION_TYPE_INTERNAL_TRANSFER
	case domain.TransactionTypeConversion:
		return pb.TransactionType_TRANSACTION_TYPE_CONVERSION
	case domain.TransactionTypeFeePayment:
		return pb.TransactionType_TRANSACTION_TYPE_FEE_PAYMENT
	default:
		return pb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func transactionStatusToProto(status domain.TransactionStatus) pb.TransactionStatus {
	switch status {
	case domain.TransactionStatusPending:
		return pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	case domain.TransactionStatusBroadcasting:
		return pb.TransactionStatus_TRANSACTION_STATUS_BROADCASTING
	case domain.TransactionStatusBroadcasted:
		return pb.TransactionStatus_TRANSACTION_STATUS_BROADCASTED
	case domain.TransactionStatusConfirming:
		return pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMING
	case domain.TransactionStatusConfirmed:
		return pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
	case domain.TransactionStatusCompleted:
		return pb.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case domain.TransactionStatusFailed:
		return pb.TransactionStatus_TRANSACTION_STATUS_FAILED
	case domain.TransactionStatusCancelled:
		return pb.TransactionStatus_TRANSACTION_STATUS_CANCELLED
	default:
		return pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

// getStringValue safely dereferences string pointer
func getStringValue(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

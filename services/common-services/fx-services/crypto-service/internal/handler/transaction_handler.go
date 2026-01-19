// internal/handler/transaction_handler.go
package handler

import (
	"context"
	"crypto-service/internal/domain"
	"crypto-service/internal/usecase"

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

// Withdraw initiates a withdrawal to external address
func (h *TransactionHandler) Withdraw(
	ctx context.Context,
	req *pb.WithdrawRequest,
) (*pb.WithdrawResponse, error) {
	
	h.logger.Info("Withdraw request",
		zap.String("user_id", req.UserId),
		zap.String("asset", req.Asset),
		zap.String("amount", req. Amount),
		zap.String("to", req.ToAddress),
	)
	
	// Validate request
	if req.UserId == "" || req.Asset == "" || req.Amount == "" || req.ToAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}
	
	chainName := chainEnumToString(req.Chain)
	
	// Execute withdrawal
	tx, err := h.transactionUsecase.Withdraw(
		ctx,
		req.UserId,
		chainName,
		req.Asset,
		req. Amount,
		req.ToAddress,
		req.Memo,
	)
	if err != nil {
		h.logger.Error("Withdrawal failed", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "withdrawal failed: %v", err)
	}
	
	return &pb.WithdrawResponse{
		Transaction: transactionToProto(tx),
		Message:     "Withdrawal initiated successfully",
	}, nil
}

// InternalTransfer transfers between users (ledger-only)
func (h *TransactionHandler) InternalTransfer(
	ctx context.Context,
	req *pb.InternalTransferRequest,
) (*pb.InternalTransferResponse, error) {
	
	h.logger. Info("InternalTransfer request",
		zap.String("from_user", req.FromUserId),
		zap.String("to_user", req.ToUserId),
		zap.String("asset", req.Asset),
		zap.String("amount", req.Amount),
	)
	
	// Validate
	if req.FromUserId == "" || req.ToUserId == "" || req.Asset == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}
	
	if req.FromUserId == req. ToUserId {
		return nil, status.Error(codes.InvalidArgument, "cannot transfer to self")
	}
	
	chainName := chainEnumToString(req.Chain)
	
	// Execute transfer
	tx, err := h.transactionUsecase.InternalTransfer(
		ctx,
		req.FromUserId,
		req.ToUserId,
		chainName,
		req.Asset,
		req.Amount,
		req.Memo,
	)
	if err != nil {
		h.logger.Error("Internal transfer failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "transfer failed: %v", err)
	}
	
	return &pb.InternalTransferResponse{
		Transaction: transactionToProto(tx),
		Message:     "Internal transfer completed successfully",
	}, nil
}

// GetWithdrawalQuote gets fee estimate for withdrawal
func (h *TransactionHandler) GetWithdrawalQuote(
	ctx context. Context,
	req *pb. GetWithdrawalQuoteRequest,
) (*pb.GetWithdrawalQuoteResponse, error) {
	
	h.logger.Info("GetWithdrawalQuote request",
		zap.String("user_id", req.UserId),
		zap.String("asset", req. Asset),
		zap.String("amount", req.Amount),
	)
	
	// Validate
	if req.UserId == "" || req.Asset == "" || req.Amount == "" || req.ToAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}
	
	chainName := chainEnumToString(req.Chain)
	
	// Get quote
	quote, err := h. transactionUsecase.GetWithdrawalQuote(
		ctx,
		req.UserId,
		chainName,
		req.Asset,
		req. Amount,
		req.ToAddress,
	)
	if err != nil {
		h.logger.Error("Failed to get quote", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "failed to get quote: %v", err)
	}
	
	return &pb.GetWithdrawalQuoteResponse{
		QuoteId:  quote.QuoteID,
		Amount:  &pb.Money{
			Amount:   quote.Amount.String(),
			Currency: req.Asset,
		},
		Fees:  &pb.FeeBreakdown{
			NetworkFee:  &pb.Money{
				Amount:   quote.NetworkFee.String(),
				Currency: quote.NetworkFeeCurrency,
			},
			NetworkFeeCurrency: quote.NetworkFeeCurrency,
			PlatformFee:  &pb.Money{
				Amount:   quote.PlatformFee.String(),
				Currency: req.Asset,
			},
			TotalFee: &pb.Money{
				Amount:   quote. TotalFee.String(),
				Currency: req.Asset,
			},
			Explanation: quote.Explanation,
		},
		TotalCost: &pb.Money{
			Amount:   quote.TotalCost.String(),
			Currency: req.Asset,
		},
		RequiredBalance: &pb.Money{
			Amount:   quote.TotalCost.String(),
			Currency: req.Asset,
		},
		UserHasBalance: quote.UserHasBalance,
		ValidUntil:     timestamppb.New(quote. ValidUntil),
		Explanation:    quote.Explanation,
	}, nil
}

// GetTransaction retrieves transaction by ID
func (h *TransactionHandler) GetTransaction(
	ctx context.Context,
	req *pb.GetTransactionRequest,
) (*pb.GetTransactionResponse, error) {
	
	if req.TransactionId == "" || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id and user_id required")
	}
	
	tx, err := h.transactionUsecase.GetTransaction(ctx, req.TransactionId, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "transaction not found: %v", err)
	}
	
	return &pb.GetTransactionResponse{
		Transaction: transactionToProto(tx),
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
	if limit == 0 {
		limit = 20
	}
	
	page := int(req.Pagination. Page)
	if page == 0 {
		page = 1
	}
	
	offset := (page - 1) * limit
	
	// Get transactions
	transactions, err := h.transactionUsecase. GetUserTransactions(ctx, req.UserId, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get transactions", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "failed to get transactions: %v", err)
	}
	
	// Convert to summaries
	summaries := make([]*pb.TransactionSummary, len(transactions))
	for i, tx := range transactions {
		summaries[i] = transactionToSummaryProto(tx)
	}
	
	return &pb. GetUserTransactionsResponse{
		Transactions: summaries,
		Pagination: &pb.PaginationResponse{
			Page:      int32(page),
			PageSize: int32(limit),
			Total:    int64(len(summaries)), // Would get actual total from DB
		},
	}, nil
}

// GetTransactionStatus gets transaction status
func (h *TransactionHandler) GetTransactionStatus(
	ctx context.Context,
	req *pb.GetTransactionStatusRequest,
) (*pb.GetTransactionStatusResponse, error) {
	
	if req.TransactionId == "" || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id and user_id required")
	}
	
	tx, err := h.transactionUsecase.GetTransaction(ctx, req.TransactionId, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "transaction not found: %v", err)
	}
	
	var txHash string
	if tx.TxHash != nil {
		txHash = *tx.TxHash
	}
	
	var statusMsg string
	if tx.StatusMessage != nil {
		statusMsg = *tx.StatusMessage
	}
	
	return &pb.GetTransactionStatusResponse{
		Status:                 transactionStatusToProto(tx. Status),
		Confirmations:         int32(tx. Confirmations),
		RequiredConfirmations: int32(tx. RequiredConfirmations),
		TxHash:                txHash,
		StatusMessage:         statusMsg,
		UpdatedAt:             timestamppb.New(tx.UpdatedAt),
	}, nil
}

// CancelTransaction cancels a pending transaction
func (h *TransactionHandler) CancelTransaction(
	ctx context.Context,
	req *pb.CancelTransactionRequest,
) (*pb.CancelTransactionResponse, error) {
	
	if req.TransactionId == "" || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id and user_id required")
	}
	
	err := h.transactionUsecase.CancelTransaction(ctx, req.TransactionId, req.UserId, req.Reason)
	if err != nil {
		h.logger.Error("Failed to cancel transaction", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "failed to cancel:  %v", err)
	}
	
	return &pb. CancelTransactionResponse{
		Success: true,
		Message: "Transaction cancelled successfully",
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func transactionToProto(tx *domain.CryptoTransaction) *pb.Transaction {
	pbTx := &pb.Transaction{
		Id:            tx.ID,
		TransactionId: tx.TransactionID,
		UserId:        tx.UserID,
		Type:          transactionTypeToProto(tx. Type),
		Chain:         stringToChainEnum(tx.Chain),
		Asset:         tx. Asset,
		FromAddress:   tx.FromAddress,
		ToAddress:     tx.ToAddress,
		IsInternal:    tx.IsInternal,
		Amount:  &pb.Money{
			Amount:   tx.Amount.String(),
			Currency: tx.Asset,
		},
		Fees: &pb.FeeBreakdown{
			NetworkFee:  &pb.Money{
				Amount:  tx.NetworkFee.String(),
			},
			PlatformFee: &pb. Money{
				Amount: tx. PlatformFee.String(),
			},
			TotalFee: &pb.Money{
				Amount: tx.TotalFee.String(),
			},
		},
		Status:                 transactionStatusToProto(tx. Status),
		Confirmations:         int32(tx.Confirmations),
		RequiredConfirmations:  int32(tx.RequiredConfirmations),
		InitiatedAt:           timestamppb.New(tx.InitiatedAt),
		CreatedAt:             timestamppb.New(tx.CreatedAt),
	}
	
	if tx. TxHash != nil {
		pbTx.TxHash = *tx.TxHash
	}
	
	if tx.BlockNumber != nil {
		pbTx.BlockNumber = *tx.BlockNumber
	}
	
	if tx. ConfirmedAt != nil {
		pbTx.ConfirmedAt = timestamppb.New(*tx. ConfirmedAt)
	}
	
	if tx.StatusMessage != nil {
		pbTx.StatusMessage = *tx. StatusMessage
	}
	
	return pbTx
}

func transactionToSummaryProto(tx *domain.CryptoTransaction) *pb.TransactionSummary {
	summary := &pb.TransactionSummary{
		TransactionId: tx.TransactionID,
		Type:          transactionTypeToProto(tx.Type),
		Chain:         stringToChainEnum(tx.Chain),
		Asset:         tx.Asset,
		Amount:         tx.Amount.String(),
		Fee:           tx.TotalFee.String(),
		Status:        transactionStatusToProto(tx.Status),
		IsInternal:    tx.IsInternal,
		CreatedAt:      timestamppb.New(tx.CreatedAt),
	}
	
	if tx. TxHash != nil {
		summary.TxHash = *tx.TxHash
	}
	
	return summary
}

func transactionTypeToProto(txType domain.TransactionType) pb.TransactionType {
	switch txType {
	case domain. TransactionTypeDeposit:
		return pb. TransactionType_TRANSACTION_TYPE_DEPOSIT
	case domain.TransactionTypeWithdrawal:
		return pb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
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
		return pb. TransactionStatus_TRANSACTION_STATUS_PENDING
	case domain. TransactionStatusBroadcasting:
		return pb.TransactionStatus_TRANSACTION_STATUS_BROADCASTING
	case domain.TransactionStatusBroadcasted:
		return pb.TransactionStatus_TRANSACTION_STATUS_BROADCASTED
	case domain. TransactionStatusConfirming:
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
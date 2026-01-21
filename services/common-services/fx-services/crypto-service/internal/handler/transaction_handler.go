// internal/handler/transaction_handler.go
package handler

import (
	"context"
	"crypto-service/internal/domain"
	"crypto-service/internal/usecase"
	//"math/big"

	pb "x/shared/genproto/shared/accounting/cryptopb"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TransactionHandler struct {
	pb.UnimplementedTransactionServiceServer
	transactionUsecase *usecase.TransactionUsecase
	logger             *zap. Logger
}

func NewTransactionHandler(
	transactionUsecase *usecase. TransactionUsecase,
	logger *zap.Logger,
) *TransactionHandler {
	return &TransactionHandler{
		transactionUsecase: transactionUsecase,
		logger:              logger,
	}
}

// ============================================================================
// NETWORK FEE ESTIMATION (for accounting module)
// ============================================================================

// EstimateNetworkFee estimates blockchain network fee
// This is called by accounting module to calculate total withdrawal cost
func (h *TransactionHandler) EstimateNetworkFee(
	ctx context.Context,
	req *pb.EstimateNetworkFeeRequest,
) (*pb.EstimateNetworkFeeResponse, error) {
	
	h.logger.Info("EstimateNetworkFee request",
		zap.String("chain", req.Chain. String()),
		zap.String("asset", req.Asset),
		zap.String("amount", req.Amount))
	
	// Validate
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED || req.Asset == "" || req. Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "chain, asset, and amount required")
	}
	
	chainName := chainEnumToString(req.Chain)
	
	// Estimate fee
	estimate, err := h.transactionUsecase.EstimateNetworkFee(
		ctx,
		chainName,
		req.Asset,
		req.Amount,
		req.ToAddress, // Optional
	)
	if err != nil {
		h.logger.Error("Failed to estimate network fee", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to estimate fee: %v", err)
	}
	
	return &pb.EstimateNetworkFeeResponse{
		Chain:        req.Chain,
		Asset:        req.Asset,
		FeeAmount:    estimate.FeeAmount. String(),
		FeeCurrency:  estimate.FeeCurrency,
		FeeFormatted: estimate.FeeFormatted,
		EstimatedAt:  timestamppb.New(estimate.EstimatedAt),
		ValidFor:     int32(estimate.ValidFor. Seconds()),
		Explanation:  "Estimated blockchain network fee",
	}, nil
}

// GetWithdrawalQuote gets complete withdrawal quote (for display to user)
func (h *TransactionHandler) GetWithdrawalQuote(
	ctx context. Context,
	req *pb. GetWithdrawalQuoteRequest,
) (*pb.GetWithdrawalQuoteResponse, error) {
	
	h.logger.Info("GetWithdrawalQuote request",
		zap.String("chain", req.Chain.String()),
		zap.String("asset", req.Asset),
		zap.String("amount", req.Amount))
	
	// Validate
	if req.Chain == pb. Chain_CHAIN_UNSPECIFIED || req.Asset == "" || req.Amount == "" || req.ToAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}
	
	chainName := chainEnumToString(req.Chain)
	
	// Get quote (only network fee - platform fee from accounting)
	quote, err := h. transactionUsecase.GetWithdrawalQuote(
		ctx,
		chainName,
		req.Asset,
		req.Amount,
		req.ToAddress,
	)
	if err != nil {
		h. logger.Error("Failed to get quote", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "failed to get quote: %v", err)
	}
	
	return &pb.GetWithdrawalQuoteResponse{
		QuoteId:  quote.QuoteID,
		Chain:   req.Chain,
		Asset:   req.Asset,
		Amount:  &pb.Money{
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
// WITHDRAWAL (called by accounting after balance deduction)
// ============================================================================

// Withdraw executes withdrawal from system hot wallet to external address
// NOTE: This should be called by accounting module AFTER: 
//   1. User virtual balance has been debited
//   2. Platform fee has been collected
//   3. Network fee has been reserved
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
	
	// Execute withdrawal from system wallet
	tx, err := h.transactionUsecase.Withdraw(
		ctx,
		req.AccountingTxId, // For idempotency
		chainName,
		req.Asset,
		req.Amount,
		req.ToAddress,
		req. Memo,
		req.UserId, // For tracking
	)
	if err != nil {
		h.logger. Error("Withdrawal failed", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "withdrawal failed: %v", err)
	}
	
	return &pb.WithdrawResponse{
		Transaction: transactionToProto(tx),
		Message:     "Withdrawal initiated successfully",
	}, nil
}

// ============================================================================
// SWEEP OPERATIONS (internal use)
// ============================================================================

// SweepUserWallet sweeps funds from user's deposit address to system wallet
func (h *TransactionHandler) SweepUserWallet(
	ctx context.Context,
	req *pb.SweepUserWalletRequest,
) (*pb.SweepUserWalletResponse, error) {
	
	h.logger.Info("SweepUserWallet request",
		zap.String("user_id", req.UserId),
		zap.String("chain", req.Chain.String()),
		zap.String("asset", req.Asset))
	
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
		Transaction: transactionToProto(tx),
		Message:     "Wallet swept successfully",
	}, nil
}

// SweepAllUsers sweeps all user wallets for a chain/asset (batch operation)
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
	transactions, err := h.transactionUsecase.SweepAllUsers(ctx, chainName, req. Asset)
	if err != nil {
		h.logger.Error("Batch sweep failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "batch sweep failed: %v", err)
	}
	
	// Convert to proto
	pbTransactions := make([]*pb.Transaction, len(transactions))
	for i, tx := range transactions {
		pbTransactions[i] = transactionToProto(tx)
	}
	
	return &pb.SweepAllUsersResponse{
		Transactions:  pbTransactions,
		SuccessCount: int32(len(transactions)),
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
	
	tx, err := h.transactionUsecase. GetTransaction(ctx, req.TransactionId)
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
	
	page := int(req.Pagination.Page)
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
			Page:        int32(page),
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
	
	if req. TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id required")
	}
	
	tx, err := h. transactionUsecase.GetTransaction(ctx, req.TransactionId)
	if err != nil {
		return nil, status. Errorf(codes.NotFound, "transaction not found: %v", err)
	}
	
	var txHash string
	if tx. TxHash != nil {
		txHash = *tx.TxHash
	}
	
	var statusMsg string
	if tx.StatusMessage != nil {
		statusMsg = *tx.StatusMessage
	}
	
	return &pb.GetTransactionStatusResponse{
		Status:                transactionStatusToProto(tx. Status),
		Confirmations:         int32(tx.Confirmations),
		RequiredConfirmations: int32(tx.RequiredConfirmations),
		TxHash:                txHash,
		StatusMessage:          statusMsg,
		UpdatedAt:             timestamppb.New(tx.UpdatedAt),
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
		Asset:         tx.Asset,
		FromAddress:   tx.FromAddress,
		ToAddress:     tx. ToAddress,
		IsInternal:    tx.IsInternal,
		Amount:  &pb.Money{
			Amount:   tx.Amount.String(),
			Currency: tx.Asset,
		},
		Status:                 transactionStatusToProto(tx. Status),
		Confirmations:         int32(tx.Confirmations),
		RequiredConfirmations: int32(tx.RequiredConfirmations),
		InitiatedAt:           timestamppb.New(tx.InitiatedAt),
		CreatedAt:             timestamppb.New(tx.CreatedAt),
	}
	
	// Network fee
	if tx.NetworkFee != nil {
		currency := tx.Asset
		if tx.NetworkFeeCurrency != nil {
			currency = *tx.NetworkFeeCurrency
		}
		pbTx.NetworkFee = &pb.Money{
			Amount:   tx.NetworkFee.String(),
			Currency: currency,
		}
	}
	
	// Optional fields
	if tx.TxHash != nil {
		pbTx.TxHash = *tx.TxHash
	}
	
	if tx.BlockNumber != nil {
		pbTx.BlockNumber = *tx.BlockNumber
	}
	
	if tx.BroadcastedAt != nil {
		pbTx.BroadcastedAt = timestamppb.New(*tx. BroadcastedAt)
	}
	
	if tx. ConfirmedAt != nil {
		pbTx.ConfirmedAt = timestamppb. New(*tx.ConfirmedAt)
	}
	
	if tx.StatusMessage != nil {
		pbTx.StatusMessage = *tx.StatusMessage
	}
	
	if tx.AccountingTxID  != nil {
		pbTx.AccountingTxId = *tx.AccountingTxID
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
		Status:        transactionStatusToProto(tx.Status),
		IsInternal:    tx.IsInternal,
		CreatedAt:     timestamppb.New(tx.CreatedAt),
	}
	
	if tx. TxHash != nil {
		summary.TxHash = *tx. TxHash
	}
	
	if tx.NetworkFee != nil {
		summary.NetworkFee = tx.NetworkFee.String()
	}
	
	return summary
}

func transactionTypeToProto(txType domain.TransactionType) pb.TransactionType {
	switch txType {
	case domain.TransactionTypeDeposit:
		return pb. TransactionType_TRANSACTION_TYPE_DEPOSIT
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
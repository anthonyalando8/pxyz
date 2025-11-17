// handler/grpc_partner_transactions.go
package handler

import (
	"context"
	"fmt"
	"log"
	"partner-service/internal/domain"
	partnersvcpb "x/shared/genproto/partner/svcpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// InitiateDeposit creates a new deposit transaction
func (h *GRPCPartnerHandler) InitiateDeposit(
	ctx context.Context,
	req *partnersvcpb.InitiateDepositRequest,
) (*partnersvcpb.InitiateDepositResponse, error) {
	if req.PartnerId == "" || req.TransactionRef == "" || req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id, transaction_ref, and user_id are required")
	}

	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "amount must be greater than 0")
	}

	txn := &domain.PartnerTransaction{
		PartnerID:      req.PartnerId,
		TransactionRef: req.TransactionRef,
		UserID:         req.UserId,
		Amount:         req.Amount,
		Currency:       req.Currency,
		PaymentMethod:  &req.PaymentMethod,
		ExternalRef:    &req.ExternalRef,
		Metadata:       convertMetadata(req.Metadata),
	}

	if err := h.uc.InitiateDeposit(ctx, txn); err != nil {
		log.Printf("[ERROR] InitiateDeposit failed: %v", err)
		return &partnersvcpb.InitiateDepositResponse{
			Success: false,
			Status:  "failed",
			Message: err.Error(),
		}, nil
	}

	return &partnersvcpb.InitiateDepositResponse{
		Success:       true,
		TransactionId: fmt.Sprintf("%d", txn.ID),
		Status:        txn.Status,
		Message:       "Deposit initiated successfully",
	}, nil
}

// GetTransactionStatus retrieves transaction status
func (h *GRPCPartnerHandler) GetTransactionStatus(
	ctx context.Context,
	req *partnersvcpb.GetTransactionStatusRequest,
) (*partnersvcpb.TransactionStatusResponse, error) {
	if req.PartnerId == "" || req.TransactionRef == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id and transaction_ref are required")
	}

	txn, err := h.uc.GetTransactionStatus(ctx, req.PartnerId, req.TransactionRef)
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

	var statusFilter *string
	if req.Status != "" {
		statusFilter = &req.Status
	}

	txns, total, err := h.uc.ListTransactions(ctx, req.PartnerId, int(req.Limit), int(req.Offset), statusFilter, nil, nil)
	if err != nil {
		log.Printf("[ERROR] ListTransactions failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list transactions")
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

func txnToProto(txn *domain.PartnerTransaction) *partnersvcpb.PartnerTransaction {
	proto := &partnersvcpb.PartnerTransaction{
		Id:             txn.ID,
		PartnerId:      txn.PartnerID,
		TransactionRef: txn.TransactionRef,
		UserId:         txn.UserID,
		Amount:         txn.Amount,
		Currency:       txn.Currency,
		Status:         txn.Status,
		CreatedAt:      timestamppb.New(txn.CreatedAt),
		UpdatedAt:      timestamppb.New(txn.UpdatedAt),
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

func convertMetadata(meta map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range meta {
		result[k] = v
	}
	return result
}
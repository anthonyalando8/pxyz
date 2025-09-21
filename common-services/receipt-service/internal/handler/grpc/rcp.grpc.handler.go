package hgrpc

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"
	receiptpb "x/shared/genproto/shared/accounting/receiptpb"
	"google.golang.org/protobuf/types/known/structpb"
	"receipt-service/internal/domain"
	"receipt-service/internal/usecase"
)

// ReceiptGRPCHandler implements the ReceiptServiceServer gRPC interface
type ReceiptGRPCHandler struct {
	receiptpb.UnimplementedReceiptServiceServer
	receiptUC *usecase.ReceiptUsecase
}

// NewReceiptGRPCHandler creates a new ReceiptGRPCHandler
func NewReceiptGRPCHandler(receiptUC *usecase.ReceiptUsecase) *ReceiptGRPCHandler {
	return &ReceiptGRPCHandler{
		receiptUC: receiptUC,
	}
}

// CreateReceipt handles creating a new receipt
func (h *ReceiptGRPCHandler) CreateReceipt(ctx context.Context, req *receiptpb.CreateReceiptRequest) (*receiptpb.CreateReceiptResponse, error) {
	rec := &domain.Receipt{
		JournalID:   req.JournalId,
		Type:        req.Type,
		Amount:      req.Amount,
		Currency:    req.Currency,
		CodedType:   req.CodedType,
		ExternalRef: req.ExternalRef,
		Creditor: domain.PartyInfo{
			ID:            req.Creditor.Id,
			Type:          req.Creditor.Type,
			Name:          req.Creditor.Name,
			Email:         req.Creditor.Email,
			Phone:         req.Creditor.Phone,
			AccountNumber: req.Creditor.AccountNumber,
			IsCreditor:    req.Creditor.IsCreditor,
		},
		Debitor: domain.PartyInfo{
			ID:            req.Debitor.Id,
			Type:          req.Debitor.Type,
			Name:          req.Debitor.Name,
			Email:         req.Debitor.Email,
			Phone:         req.Debitor.Phone,
			AccountNumber: req.Debitor.AccountNumber,
			IsCreditor:    req.Debitor.IsCreditor,
		},
	}

	// Call usecase to create receipt (no transaction here, pass nil)
	createdRec, err := h.receiptUC.CreateReceipt(ctx, rec, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create receipt: %w", err)
	}

	// Trigger notifications asynchronously
	//go h.receiptUC.SendReceiptNotification(ctx, createdRec)

	// Build proto response
	resp := &receiptpb.CreateReceiptResponse{
		Receipt: convertDomainToProto(createdRec),
	}

	return resp, nil
}

func convertDomainToProto(rec *domain.Receipt) *receiptpb.Receipt {
	var payloadStruct *structpb.Struct
	if rec.Payload != nil {
		payloadStruct, _ = structpb.NewStruct(rec.Payload)
	}
	return &receiptpb.Receipt{
		Id:          rec.ID,
		Code:        rec.Code,
		JournalId:   rec.JournalID,
		Type:        rec.Type,
		Amount:      rec.Amount,
		Currency:    rec.Currency,
		Status:      rec.Status,
		CreatedAt:   timestamppb.New(rec.CreatedAt),
		Creditor: &receiptpb.PartyInfo{
			Id:            rec.Creditor.ID,
			Type:          rec.Creditor.Type,
			Name:          rec.Creditor.Name,
			Phone:         rec.Creditor.Phone,
			Email:         rec.Creditor.Email,
			AccountNumber: rec.Creditor.AccountNumber,
			IsCreditor:    rec.Creditor.IsCreditor,
		},
		Debitor: &receiptpb.PartyInfo{
			Id:            rec.Debitor.ID,
			Type:          rec.Debitor.Type,
			Name:          rec.Debitor.Name,
			Phone:         rec.Debitor.Phone,
			Email:         rec.Debitor.Email,
			AccountNumber: rec.Debitor.AccountNumber,
			IsCreditor:    rec.Debitor.IsCreditor,
		},
		CodedType:   rec.CodedType,
		ExternalRef: rec.ExternalRef,
		Payload:     payloadStruct, // converted to *structpb.Struct
	}
}

package hgrpc

import (
	"context"

    receiptpb "x/shared/genproto/shared/accounting/receipt/v2"

	"receipt-service/internal/domain"
	"receipt-service/internal/usecase"
)

// ReceiptGRPCHandler implements the ReceiptServiceServer gRPC interface
type ReceiptGRPCHandler struct {
    receiptpb.UnimplementedReceiptServiceV2Server
    receiptUC *usecase.ReceiptUsecase
}

func NewReceiptGRPCHandler(receiptUC *usecase.ReceiptUsecase) *ReceiptGRPCHandler {
    return &ReceiptGRPCHandler{receiptUC: receiptUC}
}

func (h *ReceiptGRPCHandler) CreateReceipt(ctx context.Context, req *receiptpb.CreateReceiptRequest) (*receiptpb.CreateReceiptResponse, error) {
    rec := &domain.Receipt{
        Type:        req.Type,
        CodedType:   req.CodedType,
        Amount:      req.Amount,
        Currency:    req.Currency,
        ExternalRef: req.ExternalRef,
        Creditor:    domain.PartyInfoFromProto(req.Creditor),
        Debitor:     domain.PartyInfoFromProto(req.Debitor),
        CreatedBy:   req.CreatedBy,
        Metadata:    req.Metadata.AsMap(),
    }

    created, err := h.receiptUC.CreateReceipt(ctx, rec)
    if err != nil {
        return nil, err
    }

    return &receiptpb.CreateReceiptResponse{
        Receipt: created.ToProto(),
    }, nil
}

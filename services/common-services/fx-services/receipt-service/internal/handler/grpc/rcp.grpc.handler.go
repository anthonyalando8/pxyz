package hgrpc

import (
	"context"
	//"time"

	// "google.golang.org/protobuf/types/known/structpb"
	// "google.golang.org/protobuf/types/known/timestamppb"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v2"

	"receipt-service/internal/domain"
	"receipt-service/internal/usecase"
)

// ReceiptGRPCHandler implements the ReceiptServiceServer gRPC interface
type ReceiptGRPCHandler struct {
	receiptpb.UnimplementedReceiptServiceV2Server
	receiptUC *usecase.ReceiptUsecase
}

// // ptrTime returns a pointer to the given time.Time value.
// func ptrTime(t time.Time) *time.Time {
// 	return &t
// }

func NewReceiptGRPCHandler(receiptUC *usecase.ReceiptUsecase) *ReceiptGRPCHandler {
	return &ReceiptGRPCHandler{receiptUC: receiptUC}
}

// --- Create batch ---
func (h *ReceiptGRPCHandler) CreateReceipts(ctx context.Context, req *receiptpb.CreateReceiptsRequest) (*receiptpb.CreateReceiptsResponse, error) {
	var receipts []*domain.Receipt
	for _, r := range req.Receipts {
		receipts = append(receipts, &domain.Receipt{
			Type:        r.Type,
			CodedType:   r.CodedType,
			Amount:      r.Amount,
			Currency:    r.Currency,
			ExternalRef: r.ExternalRef,
			TransactionCost: r.TransactionCost,
			Creditor:    domain.PartyInfoFromProto(r.Creditor),
			Debitor:     domain.PartyInfoFromProto(r.Debitor),
			CreatedBy:   r.CreatedBy,
			Metadata:    r.Metadata.AsMap(),
		})
	}

	created, err := h.receiptUC.CreateReceipts(ctx, receipts)
	if err != nil {
		return nil, err
	}

	resp := &receiptpb.CreateReceiptsResponse{Receipts: make([]*receiptpb.Receipt, len(created))}
	for i, rc := range created {
		resp.Receipts[i] = rc.ToProto()
	}
	return resp, nil
}

// --- Get by code ---
func (h *ReceiptGRPCHandler) GetReceiptByCode(ctx context.Context, req *receiptpb.GetReceiptByCodeRequest) (*receiptpb.Receipt, error) {
	rec, err := h.receiptUC.GetReceiptByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	return rec.ToProto(), nil
}

// --- Update batch ---
func (h *ReceiptGRPCHandler) UpdateReceipts(ctx context.Context, req *receiptpb.UpdateReceiptsRequest) (*receiptpb.UpdateReceiptsResponse, error) {
	if len(req.Updates) == 0 {
		return &receiptpb.UpdateReceiptsResponse{}, nil
	}

	// Convert proto requests to domain updates
	patches := make([]*domain.ReceiptUpdate, len(req.Updates))
	for i, u := range req.Updates {
		patches[i] = domain.ReceiptUpdateFromProto(u)
	}

	if err := h.receiptUC.UpdateReceipts(ctx, patches); err != nil {
		return nil, err
	}

	// Response is empty, since receipts are already published to Kafka
	return &receiptpb.UpdateReceiptsResponse{}, nil
}


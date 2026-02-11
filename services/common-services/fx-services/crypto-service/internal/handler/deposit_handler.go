// internal/handler/deposit_handler.go
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

type DepositHandler struct {
	pb.UnimplementedDepositServiceServer
	depositUsecase *usecase.DepositUsecase
	logger         *zap.Logger
}

func NewDepositHandler(
	depositUsecase *usecase.DepositUsecase,
	logger *zap.Logger,
) *DepositHandler {
	return &DepositHandler{
		depositUsecase: depositUsecase,
		logger:         logger,
	}
}

// GetUserDeposits retrieves user's deposit history
func (h *DepositHandler) GetUserDeposits(
	ctx context.Context,
	req *pb.GetUserDepositsRequest,
) (*pb.GetUserDepositsResponse, error) {

	// Validate
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Set defaults for pagination
	limit := int(req.Pagination.PageSize)
	if limit == 0 {
		limit = 20
	}

	page := int(req.Pagination.Page)
	if page == 0 {
		page = 1
	}

	offset := (page - 1) * limit

	// Get deposits from usecase
	deposits, err := h.depositUsecase.GetUserDeposits(ctx, req.UserId, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get user deposits", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get deposits: %v", err)
	}

	// Apply filters if provided
	var filtered []*domain.CryptoDeposit
	for _, deposit := range deposits {
		// Filter by chain
		if req.Chain != pb.Chain_CHAIN_UNSPECIFIED {
			chainName := chainEnumToString(req.Chain)
			if deposit.Chain != chainName {
				continue
			}
		}

		// Filter by asset
		if req.Asset != "" && deposit.Asset != req.Asset {
			continue
		}

		// Filter by status
		if req.Status != pb.DepositStatus_DEPOSIT_STATUS_UNSPECIFIED {
			if depositStatusToProto(deposit.Status) != req.Status {
				continue
			}
		}

		filtered = append(filtered, deposit)
	}

	// Convert to protobuf
	pbDeposits := make([]*pb.Deposit, len(filtered))
	for i, deposit := range filtered {
		pbDeposits[i] = depositToProto(deposit)
	}

	return &pb.GetUserDepositsResponse{
		Deposits: pbDeposits,
		Pagination: &pb.PaginationResponse{
			Page:       int32(page),
			PageSize:   int32(limit),
			Total:      int64(len(pbDeposits)),
			TotalPages: int32((len(pbDeposits) + limit - 1) / limit),
		},
	}, nil
}

// GetDeposit retrieves a specific deposit by ID
func (h *DepositHandler) GetDeposit(
	ctx context.Context,
	req *pb.GetDepositRequest,
) (*pb.GetDepositResponse, error) {

	// Validate
	if req.DepositId == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Get deposit
	deposit, err := h.depositUsecase.GetDeposit(ctx, req.DepositId, req.UserId)
	if err != nil {
		h.logger.Error("Failed to get deposit", zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "deposit not found: %v", err)
	}

	return &pb.GetDepositResponse{
		Deposit: depositToProto(deposit),
	}, nil
}

// GetPendingDeposits retrieves user's pending deposits (waiting for confirmations)
func (h *DepositHandler) GetPendingDeposits(
	ctx context.Context,
	req *pb.GetPendingDepositsRequest,
) (*pb.GetPendingDepositsResponse, error) {

	// Validate
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Get pending deposits
	deposits, err := h.depositUsecase.GetPendingDeposits(ctx, req.UserId)
	if err != nil {
		h.logger.Error("Failed to get pending deposits", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get pending deposits: %v", err)
	}

	// Convert to protobuf
	pbDeposits := make([]*pb.Deposit, len(deposits))
	for i, deposit := range deposits {
		pbDeposits[i] = depositToProto(deposit)
	}

	return &pb.GetPendingDepositsResponse{
		Deposits: pbDeposits,
		Total:    int32(len(pbDeposits)),
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// depositToProto converts domain. CryptoDeposit to protobuf Deposit
func depositToProto(deposit *domain.CryptoDeposit) *pb.Deposit {
	pbDeposit := &pb.Deposit{
		Id:          deposit.ID,
		DepositId:   deposit.DepositID,
		UserId:      deposit.UserID,
		Chain:       stringToChainEnum(deposit.Chain),
		Asset:       deposit.Asset,
		FromAddress: deposit.FromAddress,
		ToAddress:   deposit.ToAddress,
		Amount: &pb.Money{
			Amount:   deposit.Amount.String(),
			Currency: deposit.Asset,
			Decimals: int32(getAssetDecimals(deposit.Asset)),
		},
		TxHash:                deposit.TxHash,
		BlockNumber:           deposit.BlockNumber,
		Confirmations:         int32(deposit.Confirmations),
		RequiredConfirmations: int32(deposit.RequiredConfirmations),
		Status:                depositStatusToProto(deposit.Status),
		UserNotified:          deposit.UserNotified,
		DetectedAt:            timestamppb.New(deposit.DetectedAt),
	}

	// Optional timestamp fields
	if deposit.BlockTimestamp != nil {
		pbDeposit.BlockTimestamp = timestamppb.New(*deposit.BlockTimestamp)
	}

	if deposit.ConfirmedAt != nil {
		pbDeposit.ConfirmedAt = timestamppb.New(*deposit.ConfirmedAt)
	}

	if deposit.CreditedAt != nil {
		pbDeposit.CreditedAt = timestamppb.New(*deposit.CreditedAt)
	}

	return pbDeposit
}

// depositStatusToProto converts domain. DepositStatus to protobuf DepositStatus
func depositStatusToProto(status domain.DepositStatus) pb.DepositStatus {
	switch status {
	case domain.DepositStatusDetected:
		return pb.DepositStatus_DEPOSIT_STATUS_DETECTED
	case domain.DepositStatusPending:
		return pb.DepositStatus_DEPOSIT_STATUS_PENDING
	case domain.DepositStatusConfirmed:
		return pb.DepositStatus_DEPOSIT_STATUS_CONFIRMED
	case domain.DepositStatusCredited:
		return pb.DepositStatus_DEPOSIT_STATUS_CREDITED
	case domain.DepositStatusFailed:
		return pb.DepositStatus_DEPOSIT_STATUS_FAILED
	default:
		return pb.DepositStatus_DEPOSIT_STATUS_UNSPECIFIED
	}
}

// getAssetDecimals returns decimals for asset code
func getAssetDecimals(asset string) int {
	decimals := map[string]int{
		"TRX":  6,
		"USDT": 6,
		"BTC":  8,
		"ETH":  18,
		"USDC": 6,
	}

	if d, ok := decimals[asset]; ok {
		return d
	}
	return 6 // default
}

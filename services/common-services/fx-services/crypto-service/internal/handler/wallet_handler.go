// internal/handler/wallet_handler.go
package handler

import (
	"context"
	"crypto-service/internal/domain"
	"crypto-service/internal/usecase"
	"fmt"

	pb "x/shared/genproto/shared/accounting/cryptopb"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WalletHandler struct {
	pb.UnimplementedWalletServiceServer
	walletUsecase *usecase.WalletUsecase
	systemUsecase   *usecase.SystemUsecase
	logger        *zap.Logger
}

func NewWalletHandler(
	walletUsecase *usecase.WalletUsecase,
	systemUsecase *usecase.SystemUsecase,
	logger *zap. Logger,
) *WalletHandler {
	return &WalletHandler{
		walletUsecase: walletUsecase,
		systemUsecase:   systemUsecase,
		logger:        logger,
	}
}

// CreateWallet creates a new crypto wallet
func (h *WalletHandler) CreateWallet(
	ctx context.Context,
	req *pb.CreateWalletRequest,
) (*pb.CreateWalletResponse, error) {
	
	h.logger.Info("CreateWallet request",
		zap.String("user_id", req.UserId),
		zap.String("chain", req.Chain. String()),
		zap.String("asset", req.Asset),
	)
	
	// Validate request
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "chain is required")
	}
	if req.Asset == "" {
		return nil, status.Error(codes.InvalidArgument, "asset is required")
	}
	
	// Convert chain enum to string
	chainName := chainEnumToString(req.Chain)
	
	// Create wallet
	wallet, err := h. walletUsecase.CreateWallet(
		ctx,
		req.UserId,
		chainName,
		req. Asset,
		req.Label,
	)
	if err != nil {
		h.logger.Error("Failed to create wallet", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "failed to create wallet: %v", err)
	}
	
	// Convert to protobuf
	pbWallet := walletToProto(wallet)
	
	return &pb.CreateWalletResponse{
		Wallet:   pbWallet,
		Message: fmt.Sprintf("Wallet created successfully for %s on %s", req.Asset, chainName),
	}, nil
}

// GetUserWallets retrieves all wallets for a user
func (h *WalletHandler) GetUserWallets(
	ctx context.Context,
	req *pb.GetUserWalletsRequest,
) (*pb.GetUserWalletsResponse, error) {
	
	h.logger.Info("GetUserWallets request", zap.String("user_id", req.UserId))
	
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	
	// Prepare filters
	var chainFilter, assetFilter *string
	if req.Chain != pb.Chain_CHAIN_UNSPECIFIED {
		chain := chainEnumToString(req.Chain)
		chainFilter = &chain
	}
	if req.Asset != "" {
		assetFilter = &req.Asset
	}
	
	// Get wallets
	wallets, err := h.walletUsecase.GetUserWallets(ctx, req.UserId, chainFilter, assetFilter,req.CreateIfMissing)
	if err != nil {
		h. logger.Error("Failed to get wallets", zap.Error(err))
		return nil, status. Errorf(codes.Internal, "failed to get wallets: %v", err)
	}
	
	// Convert to protobuf
	pbWallets := make([]*pb.Wallet, len(wallets))
	for i, wallet := range wallets {
		pbWallets[i] = walletToProto(wallet)
	}
	
	return &pb.GetUserWalletsResponse{
		Wallets: pbWallets,
		Total:   int32(len(pbWallets)),
	}, nil
}

// GetWallet retrieves a specific wallet
func (h *WalletHandler) GetWallet(
	ctx context. Context,
	req *pb. GetWalletRequest,
) (*pb.GetWalletResponse, error) {
	
	if req.WalletId == 0 {
		return nil, status.Error(codes.InvalidArgument, "wallet_id is required")
	}
	
	// Get wallet by ID
	wallet, err := h.walletUsecase.GetWalletByAddress(ctx, "") // We need to add GetByID method
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "wallet not found: %v", err)
	}
	
	// Verify ownership
	if wallet.UserID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "unauthorized")
	}
	
	return &pb.GetWalletResponse{
		Wallet:  walletToProto(wallet),
	}, nil
}

// GetBalance retrieves wallet balance
func (h *WalletHandler) GetBalance(
	ctx context.Context,
	req *pb.GetBalanceRequest,
) (*pb.GetBalanceResponse, error) {
	
	h.logger.Info("GetBalance request",
		zap.String("user_id", req.UserId),
		zap.String("chain", req.Chain.String()),
		zap.String("asset", req.Asset),
	)
	
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "chain is required")
	}
	if req.Asset == "" {
		return nil, status.Error(codes.InvalidArgument, "asset is required")
	}
	
	chainName := chainEnumToString(req.Chain)
	
	// Get balance
	balance, err := h.walletUsecase.GetWalletBalance(
		ctx,
		req.UserId,
		chainName,
		req.Asset,
		false, // Don't force refresh by default
	)
	if err != nil {
		h.logger.Error("Failed to get balance", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get balance: %v", err)
	}
	
	return &pb.GetBalanceResponse{
		Balance: &pb.Money{
			Amount:    balance. BalanceFormatted,
			Currency: balance.Asset,
			Decimals:  int32(balance.Decimals),
		},
		Address:    balance.Address,
		WalletId:   balance.WalletID,
		UpdatedAt: timestamppb. New(balance.UpdatedAt),
	}, nil
}

// GetWalletByAddress retrieves wallet by address
func (h *WalletHandler) GetWalletByAddress(
	ctx context.Context,
	req *pb.GetWalletByAddressRequest,
) (*pb.GetWalletResponse, error) {
	
	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}
	
	wallet, err := h.walletUsecase.GetWalletByAddress(ctx, req.Address)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "wallet not found: %v", err)
	}
	
	return &pb.GetWalletResponse{
		Wallet: walletToProto(wallet),
	}, nil
}

// RefreshBalance forces balance refresh from blockchain
func (h *WalletHandler) RefreshBalance(
	ctx context.Context,
	req *pb.RefreshBalanceRequest,
) (*pb.RefreshBalanceResponse, error) {
	
	if req.WalletId == 0 || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_id and user_id are required")
	}
	
	newBalance, previousBalance, err := h.walletUsecase.RefreshBalance(
		ctx,
		req. WalletId,
		req.UserId,
	)
	if err != nil {
		h.logger.Error("Failed to refresh balance", zap. Error(err))
		return nil, status.Errorf(codes.Internal, "failed to refresh balance: %v", err)
	}
	
	return &pb.RefreshBalanceResponse{
		Balance: &pb.Money{
			Amount:   newBalance.String(),
			Currency: "units", // Would get actual currency
		},
		PreviousBalance:  &pb.Money{
			Amount:   previousBalance.String(),
			Currency: "units",
		},
		UpdatedAt: timestamppb.Now(),
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func walletToProto(wallet *domain.CryptoWallet) *pb.Wallet {
	pbWallet := &pb.Wallet{
		Id:        wallet. ID,
		UserId:    wallet.UserID,
		Chain:     stringToChainEnum(wallet.Chain),
		Asset:     wallet.Asset,
		Address:   wallet.Address,
		IsPrimary: wallet.IsPrimary,
		IsActive:  wallet.IsActive,
		Balance:  &pb.Money{
			Amount:   wallet.Balance.String(),
			Currency: wallet.Asset,
		},
		CreatedAt: timestamppb. New(wallet.CreatedAt),
	}
	
	if wallet.Label != nil {
		pbWallet.Label = *wallet.Label
	}
	
	if wallet.LastBalanceUpdate != nil {
		pbWallet.LastBalanceUpdate = timestamppb.New(*wallet.LastBalanceUpdate)
	}
	
	return pbWallet
}

func chainEnumToString(chain pb.Chain) string {
	switch chain {
	case pb. Chain_CHAIN_TRON:
		return "TRON"
	case pb.Chain_CHAIN_BITCOIN:
		return "BITCOIN"
	case pb.Chain_CHAIN_ETHEREUM: 
		return "ETHEREUM"
	default:
		return ""
	}
}

func stringToChainEnum(chain string) pb.Chain {
	switch chain {
	case "TRON":
		return pb. Chain_CHAIN_TRON
	case "BITCOIN": 
		return pb.Chain_CHAIN_BITCOIN
	case "ETHEREUM":
		return pb.Chain_CHAIN_ETHEREUM
	default:
		return pb.Chain_CHAIN_UNSPECIFIED
	}
}

// internal/handler/wallet_handler.go

// CreateWallets creates multiple wallets in batch
func (h *WalletHandler) CreateWallets(
	ctx context.Context,
	req *pb.CreateWalletsRequest,
) (*pb.CreateWalletsResponse, error) {
	
	h.logger.Info("Creating multiple wallets",
		zap.String("user_id", req.UserId),
		zap.Int("count", len(req.Wallets)))
	
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	
	if len(req.Wallets) == 0 {
		return nil, status.Error(codes. InvalidArgument, "at least one wallet spec required")
	}
	
	var createdWallets []*pb. Wallet
	var errors []*pb.WalletError
	
	// Create each wallet
	for _, spec := range req.Wallets {
		chainName := chainEnumToString(spec.Chain)
		
		wallet, err := h.walletUsecase.CreateWallet(
			ctx,
			req.UserId,
			chainName,
			spec. Asset,
			spec.Label,
		)
		
		if err != nil {
			h.logger. Warn("Failed to create wallet",
				zap.String("chain", chainName),
				zap.String("asset", spec.Asset),
				zap.Error(err))
			
			errors = append(errors, &pb.WalletError{
				Chain:        spec.Chain,
				Asset:        spec.Asset,
				ErrorMessage: err.Error(),
			})
			continue
		}
		
		createdWallets = append(createdWallets, walletToProto(wallet))
	}
	
	successCount := len(createdWallets)
	failedCount := len(errors)
	
	message := fmt.Sprintf("Created %d wallets successfully", successCount)
	if failedCount > 0 {
		message = fmt.Sprintf("Created %d wallets, %d failed", successCount, failedCount)
	}
	
	h.logger.Info("Batch wallet creation completed",
		zap.Int("success", successCount),
		zap.Int("failed", failedCount))
	
	return &pb. CreateWalletsResponse{
		Wallets:       createdWallets,
		Errors:       errors,
		SuccessCount: int32(successCount),
		FailedCount:  int32(failedCount),
		Message:      message,
	}, nil
}

// InitializeUserWallets creates wallets for all supported chains/assets
func (h *WalletHandler) InitializeUserWallets(
	ctx context. Context,
	req *pb. InitializeUserWalletsRequest,
) (*pb.InitializeUserWalletsResponse, error) {
	
	h.logger. Info("Initializing user wallets",
		zap.String("user_id", req.UserId))
	
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	
	// Determine which chains/assets to create
	specs := h.getWalletSpecs(req. Chains, req.Assets)
	
	var createdWallets []*pb.Wallet
	var errors []*pb.WalletError
	var skippedCount int32
	
	// Create each wallet
	for _, spec := range specs {
		chainName := chainEnumToString(spec.Chain)
		
		// Check if wallet already exists
		if req.SkipExisting {
			existing, _ := h.walletUsecase.GetUserWallet(ctx, req.UserId, chainName, spec.Asset)
			if existing != nil {
				h.logger.Info("Wallet already exists, skipping",
					zap. String("chain", chainName),
					zap.String("asset", spec.Asset))
				skippedCount++
				continue
			}
		}
		
		// Create wallet
		wallet, err := h.walletUsecase. CreateWallet(
			ctx,
			req.UserId,
			chainName,
			spec. Asset,
			fmt.Sprintf("%s %s Wallet", chainName, spec.Asset),
		)
		
		if err != nil {
			h.logger. Warn("Failed to create wallet",
				zap.String("chain", chainName),
				zap.String("asset", spec.Asset),
				zap.Error(err))
			
			errors = append(errors, &pb.WalletError{
				Chain:         spec.Chain,
				Asset:        spec.Asset,
				ErrorMessage: err.Error(),
			})
			continue
		}
		
		createdWallets = append(createdWallets, walletToProto(wallet))
	}
	
	totalCreated := int32(len(createdWallets))
	totalFailed := int32(len(errors))
	
	message := fmt.Sprintf("Initialized %d wallets for user", totalCreated)
	if skippedCount > 0 {
		message += fmt.Sprintf(" (%d skipped)", skippedCount)
	}
	if totalFailed > 0 {
		message += fmt.Sprintf(" (%d failed)", totalFailed)
	}
	
	h.logger.Info("User wallet initialization completed",
		zap.String("user_id", req.UserId),
		zap.Int32("created", totalCreated),
		zap.Int32("skipped", skippedCount),
		zap.Int32("failed", totalFailed))
	
	return &pb.InitializeUserWalletsResponse{
		Wallets:       createdWallets,
		Errors:        errors,
		TotalCreated:  totalCreated,
		TotalSkipped:   skippedCount,
		TotalFailed:   totalFailed,
		Message:       message,
	}, nil
}

// getWalletSpecs returns the list of wallets to create
func (h *WalletHandler) getWalletSpecs(chains []pb.Chain, assets []string) []*pb.WalletSpec {
	// Default supported configurations
	defaultSpecs := []*pb.WalletSpec{
		// TRON
		{Chain: pb.Chain_CHAIN_TRON, Asset: "TRX"},
		{Chain: pb.Chain_CHAIN_TRON, Asset: "USDT"},
		
		// Bitcoin
		{Chain: pb.Chain_CHAIN_BITCOIN, Asset: "BTC"},
		
		// Add more as you support them
		// {Chain: pb.Chain_CHAIN_ETHEREUM, Asset: "ETH"},
		// {Chain: pb.Chain_CHAIN_ETHEREUM, Asset: "USDT"},
		// {Chain: pb. Chain_CHAIN_ETHEREUM, Asset: "USDC"},
	}
	
	// If specific chains/assets requested, filter
	if len(chains) > 0 || len(assets) > 0 {
		var filtered []*pb.WalletSpec
		
		for _, spec := range defaultSpecs {
			// Check chain filter
			if len(chains) > 0 {
				chainMatch := false
				for _, c := range chains {
					if c == spec.Chain {
						chainMatch = true
						break
					}
				}
				if ! chainMatch {
					continue
				}
			}
			
			// Check asset filter
			if len(assets) > 0 {
				assetMatch := false
				for _, a := range assets {
					if a == spec.Asset {
						assetMatch = true
						break
					}
				}
				if !assetMatch {
					continue
				}
			}
			
			filtered = append(filtered, spec)
		}
		
		return filtered
	}
	
	return defaultSpecs
}
// internal/handler/crypto_handler.go
package handler

import (
	"context"
	registry "crypto-service/internal/chains/registry"
	"crypto-service/internal/usecase"
	"fmt"

	pb "x/shared/genproto/shared/accounting/cryptopb"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CryptoHandler struct {
	pb.UnimplementedCryptoServiceServer
	chainRegistry  *registry.Registry
	walletUsecase  *usecase.WalletUsecase
	systemUsecase  *usecase.SystemUsecase
	logger         *zap.Logger
	version        string
}

func NewCryptoHandler(
	chainRegistry *registry.Registry,
	walletUsecase *usecase. WalletUsecase,
	systemUsecase  *usecase.SystemUsecase,
	logger *zap.Logger,
) *CryptoHandler {
	return &CryptoHandler{
		chainRegistry:  chainRegistry,
		walletUsecase: walletUsecase,
		systemUsecase:  systemUsecase,
		logger:        logger,
		version:       "1.0.0",
	}
}

// HealthCheck returns service health status
func (h *CryptoHandler) HealthCheck(
	ctx context.Context,
	req *pb.HealthCheckRequest,
) (*pb.HealthCheckResponse, error) {
	
	h.logger.Debug("HealthCheck request received")
	
	// Get all registered chains
	chains := h.chainRegistry.List()
	
	// Check each chain status
	chainStatuses := make([]*pb.ChainStatus, len(chains))
	for i, chainName := range chains {
		chain, err := h.chainRegistry. Get(chainName)
		if err != nil {
			chainStatuses[i] = &pb.ChainStatus{
				Chain:     stringToChainEnum(chainName),
				Available: false,
				Message:   fmt.Sprintf("Chain not available: %v", err),
			}
			continue
		}
		
		// Try to get chain name (basic health check)
		available := chain.Name() != ""
		
		chainStatuses[i] = &pb.ChainStatus{
			Chain:        stringToChainEnum(chainName),
			Available:    available,
			CurrentBlock: 0, // Would implement actual block height check
			Message:      "Operational",
		}
	}
	
	return &pb.HealthCheckResponse{
		Status:   "healthy",
		Version:  h.version,
		Chains:  chainStatuses,
	}, nil
}

// GetSupportedAssets returns list of supported assets
func (h *CryptoHandler) GetSupportedAssets(
	ctx context.Context,
	req *pb.GetSupportedAssetsRequest,
) (*pb.GetSupportedAssetsResponse, error) {
	
	h.logger.Debug("GetSupportedAssets request received")
	
	// Define supported assets (would come from config/database)
	assets := []*pb.Asset{
		{
			Code:     "TRX",
			Name:     "TRON",
			Chain:    pb.Chain_CHAIN_TRON,
			Decimals: 6,
		},
		{
			Code:             "USDT",
			Name:            "Tether USD",
			Chain:           pb.Chain_CHAIN_TRON,
			ContractAddress: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			Decimals:        6,
		},
		{
			Code:     "BTC",
			Name:      "Bitcoin",
			Chain:     pb.Chain_CHAIN_BITCOIN,
			Decimals:  8,
		},
		// Add more as needed
	}
	
	return &pb.GetSupportedAssetsResponse{
		Assets:  assets,
	}, nil
}

// ValidateAddress validates a blockchain address
func (h *CryptoHandler) ValidateAddress(
	ctx context.Context,
	req *pb.ValidateAddressRequest,
) (*pb.ValidateAddressResponse, error) {
	
	h.logger. Info("ValidateAddress request",
		zap.String("chain", req.Chain. String()),
		zap.String("address", req.Address),
	)
	
	// Validate inputs
	if req.Chain == pb.Chain_CHAIN_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "chain is required")
	}
	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}
	
	chainName := chainEnumToString(req.Chain)
	
	// Validate using usecase
	isValid, message, err := h.walletUsecase.ValidateAddress(ctx, chainName, req.Address)
	if err != nil {
		h.logger.Error("Address validation failed", zap.Error(err))
		return &pb.ValidateAddressResponse{
			IsValid:          false,
			Message:          fmt.Sprintf("Validation error: %v", err),
			FormattedAddress: "",
		}, nil
	}
	
	return &pb.ValidateAddressResponse{
		IsValid:          isValid,
		Message:          message,
		FormattedAddress: req.Address, // Would standardize format
	}, nil
}
	// GetSystemWallets returns all system wallets (admin only)
func (h *CryptoHandler) GetSystemWallets(
	ctx context.Context,
	req *pb.GetSystemWalletsRequest,
) (*pb.GetSystemWalletsResponse, error) {
	
	// TODO: Add admin authentication check here
	
	wallets, err := h.systemUsecase.GetAllSystemWallets(ctx)
	if err != nil {
		return nil, status. Errorf(codes.Internal, "failed to get system wallets: %v", err)
	}
	
	pbWallets := make([]*pb. Wallet, len(wallets))
	for i, wallet := range wallets {
		pbWallets[i] = walletToProto(wallet)
	}
	
	return &pb.GetSystemWalletsResponse{
		Wallets: pbWallets,
	}, nil
}
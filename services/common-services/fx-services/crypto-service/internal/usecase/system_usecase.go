// internal/usecase/system_usecase.go
package usecase

import (
	"context"
	registry "crypto-service/internal/chains/registry"
		"crypto-service/pkg/utils"

	"crypto-service/internal/domain"
	"crypto-service/internal/repository"
	"crypto-service/internal/security"
	"fmt"
	"math/big"
	"time"

	"go.uber.org/zap"
)

const (
	SystemUserID = "SYSTEM"
)

type SystemUsecase struct {
	walletRepo    *repository.CryptoWalletRepository
	chainRegistry *registry.Registry
	encryption    *security.Encryption
	logger        *zap.Logger
}

func NewSystemUsecase(
	walletRepo *repository.CryptoWalletRepository,
	chainRegistry *registry.Registry,
	encryption *security.Encryption,
	logger *zap.Logger,
) *SystemUsecase {
	return &SystemUsecase{
		walletRepo:    walletRepo,
		chainRegistry: chainRegistry,
		encryption:    encryption,
		logger:        logger,
	}
}

// InitializeSystemWallets creates system wallets for all supported chains/assets
func (uc *SystemUsecase) InitializeSystemWallets(ctx context.Context) error {
	uc.logger.Info("Initializing system wallets")

	// Define supported assets per chain
	supportedAssets := map[string][]string{
		"TRON": {"TRX", "USDT"},
		"BITCOIN": {"BTC"},
		"ETHEREUM": {"ETH", "USDC"},
		// Future chains
	}

	createdCount := 0
	existingCount := 0

	for chainName, assets := range supportedAssets {
		// Check if chain is registered
		chain, err := uc.chainRegistry.Get(chainName)
		if err != nil {
			uc. logger.Warn("Chain not registered, skipping",
				zap. String("chain", chainName),
				zap.Error(err))
			continue
		}

		for _, assetCode := range assets {
			// Check if system wallet already exists
			existingWallet, err := uc.walletRepo.GetUserWalletByChainAsset(
				ctx,
				SystemUserID,
				chainName,
				assetCode,
			)

			if err == nil && existingWallet != nil {
				uc.logger.Info("System wallet already exists",
					zap.String("chain", chainName),
					zap.String("asset", assetCode),
					zap.String("address", existingWallet.Address))
				existingCount++
				continue
			}

			// Create new system wallet
			uc.logger.Info("Creating system wallet",
				zap.String("chain", chainName),
				zap.String("asset", assetCode))

			walletCtx := uc.prepareWalletContext(ctx, chainName, assetCode, "SYSTEM")

			wallet, err := chain.GenerateWallet(walletCtx)
			if err != nil {
				uc.logger.Error("Failed to generate system wallet",
					zap.String("chain", chainName),
					zap.String("asset", assetCode),
					zap.Error(err))
				continue
			}

			// Encrypt private key
			encryptedPrivateKey, err := uc.encryption.Encrypt(wallet. PrivateKey)
			if err != nil {
				uc.logger.Error("Failed to encrypt private key",
					zap. Error(err))
				continue
			}

			// Create wallet record
			label := fmt.Sprintf("System %s Wallet", assetCode)
			systemWallet := &domain.CryptoWallet{
				UserID:              SystemUserID,
				Chain:                chainName,
				Asset:               assetCode,
				Address:              wallet.Address,
				PublicKey:           &wallet.PublicKey,
				EncryptedPrivateKey: encryptedPrivateKey,
				EncryptionVersion:   "v1",
				Label:               &label,
				IsPrimary:           true,
				IsActive:            true,
				Balance:             big.NewInt(0),
			}

			// Save to database
			if err := uc. walletRepo.Create(ctx, systemWallet); err != nil {
				uc. logger.Error("Failed to save system wallet",
					zap.String("chain", chainName),
					zap.String("asset", assetCode),
					zap.Error(err))
				continue
			}

			uc.logger.Info("System wallet created successfully",
				zap. String("chain", chainName),
				zap.String("asset", assetCode),
				zap.String("address", wallet.Address),
				zap.Int64("wallet_id", systemWallet.ID))

			createdCount++
		}
	}

	uc. logger.Info("System wallet initialization completed",
		zap.Int("created", createdCount),
		zap.Int("existing", existingCount),
		zap.Int("total", createdCount+existingCount))

	return nil
}

func (uc *SystemUsecase) prepareWalletContext(
	ctx context.Context,
	chainName, assetCode, userID string,
) context.Context {
	// Add user ID to context
	ctx = context.WithValue(ctx, domain.UserIDKey, userID)
	
	// Add asset to context
	ctx = context.WithValue(ctx, domain.AssetKey, assetCode)
	
	// Add chain to context
	ctx = context.WithValue(ctx, domain.ChainKey, chainName)
	
	//  Determine wallet type based on chain and asset
	walletType := "standard" // Default
	
	// Use Circle for USDC on Ethereum
	if chainName == "ETHEREUM" && assetCode == "USDC" {
		walletType = "circle"
		uc.logger.Info("Wallet type determined",
			zap.String("type", walletType),
			zap.String("reason", "USDC on Ethereum with Circle enabled"))
	}
	
	ctx = context.WithValue(ctx, domain.WalletTypeKey, walletType)
	
	return ctx
}

// GetSystemWallet retrieves system wallet for chain/asset
func (uc *SystemUsecase) GetSystemWallet(
	ctx context.Context,
	chainName, assetCode string,
) (*domain.CryptoWallet, error) {
	return uc.walletRepo.GetUserPrimaryWallet(ctx, SystemUserID, chainName, assetCode)
}

// GetAllSystemWallets retrieves all system wallets
func (uc *SystemUsecase) GetAllSystemWallets(ctx context.Context) ([]*domain.CryptoWallet, error) {
	return uc.walletRepo.GetUserWallets(ctx, SystemUserID)
}

// GetSystemBalance gets total system balance for asset
func (uc *SystemUsecase) GetSystemBalance(
	ctx context.Context,
	chainName, assetCode string,
) (*domain.WalletBalance, error) {
	
	wallet, err := uc.GetSystemWallet(ctx, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("system wallet not found: %w", err)
	}

	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return nil, err
	}

	// Get fresh balance from blockchain
	asset := utils.AssetFromChainAndCode(chainName, assetCode)
	privateKey, err := uc.encryption.Decrypt(wallet.EncryptedPrivateKey)
	if err != nil {

		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}
	balance, err := chain.GetBalance(ctx, wallet.Address, privateKey, asset)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Update cached balance
	uc.walletRepo.UpdateBalance(ctx, wallet.ID, balance. Amount)

	return &domain.WalletBalance{
		WalletID:         wallet.ID,
		Address:          wallet.Address,
		Chain:            wallet.Chain,
		Asset:            wallet.Asset,
		Balance:          balance.Amount,
		Decimals:         asset.Decimals,
		BalanceFormatted: utils.FormatBalance(balance.Amount, asset.Decimals, assetCode),
		UpdatedAt:        time.Now(),
	}, nil
}
// internal/usecase/wallet_usecase.go
package usecase

import (
	"context"
	registry "crypto-service/internal/chains/registry"
	"crypto-service/internal/domain"
	"crypto-service/internal/repository"
	"crypto-service/internal/security"
	"fmt"
	"math/big"
	"time"

	"go.uber.org/zap"
)

type WalletUsecase struct {
	walletRepo    *repository.CryptoWalletRepository
	chainRegistry *registry.Registry
	encryption    *security.Encryption
	logger        *zap.Logger
}

func NewWalletUsecase(
	walletRepo *repository.CryptoWalletRepository,
	chainRegistry *registry.Registry,
	encryption *security.Encryption,
	logger *zap.Logger,
) *WalletUsecase {
	return &WalletUsecase{
		walletRepo:    walletRepo,
		chainRegistry: chainRegistry,
		encryption:    encryption,
		logger:        logger,
	}
}

// CreateWallet creates a new crypto wallet for user
func (uc *WalletUsecase) CreateWallet(
	ctx context.Context,
	userID, chainName, assetCode, label string,
) (*domain.CryptoWallet, error) {
	
	uc.logger.Info("Creating wallet",
		zap.String("user_id", userID),
		zap.String("chain", chainName),
		zap.String("asset", assetCode),
	)
	
	// 1. Check if user already has a wallet for this chain/asset
	existingWallet, err := uc.walletRepo.GetUserWalletByChainAsset(ctx, userID, chainName, assetCode)
	if err == nil && existingWallet != nil {
		uc.logger.Info("Wallet already exists", zap.String("address", existingWallet.Address))
		return existingWallet, nil
	}
	
	// 2. Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return nil, fmt.Errorf("unsupported chain %s: %w", chainName, err)
	}
	
	// 3. Generate new wallet using chain interface
	walletKeys, err := chain.GenerateWallet(ctx) // ✅ Updated to GenerateWallet
	if err != nil {
		return nil, fmt.Errorf("failed to generate wallet: %w", err)
	}
	
	uc.logger.Info("Generated new address",
		zap.String("address", walletKeys.Address),
		zap.String("chain", chainName),
	)
	
	// 4. Encrypt private key
	encryptedPrivateKey, err := uc. encryption.Encrypt(walletKeys.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}
	
	// 5. Create wallet record
	wallet := &domain.CryptoWallet{
		UserID:              userID,
		Chain:               chainName,
		Asset:               assetCode,
		Address:             walletKeys.Address,
		PublicKey:           &walletKeys.PublicKey,
		EncryptedPrivateKey: encryptedPrivateKey,
		EncryptionVersion:   "v1",
		Label:               &label,
		IsPrimary:           true,
		IsActive:            true,
		Balance:              big.NewInt(0),
	}
	
	// 6. Save to database
	if err := uc. walletRepo.Create(ctx, wallet); err != nil {
		return nil, fmt. Errorf("failed to save wallet: %w", err)
	}
	
	uc.logger.Info("Wallet created successfully",
		zap. Int64("wallet_id", wallet.ID),
		zap.String("address", wallet.Address),
	)
	
	return wallet, nil
}

// internal/usecase/wallet_usecase.go

// CreateWallets creates multiple wallets in batch
func (uc *WalletUsecase) CreateWallets(
	ctx context.Context,
	userID string,
	specs []WalletSpec,
) ([]*domain.CryptoWallet, []error) {
	
	var wallets []*domain.CryptoWallet
	var errors []error
	
	for _, spec := range specs {
		wallet, err := uc.CreateWallet(ctx, userID, spec.Chain, spec.Asset, spec.Label)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s %s: %w", spec.Chain, spec.Asset, err))
			continue
		}
		wallets = append(wallets, wallet)
	}
	
	return wallets, errors
}

type WalletSpec struct {
	Chain string
	Asset string
	Label string
}

// GetUserWallet retrieves user's wallet for chain/asset
func (uc *WalletUsecase) GetUserWallet(
	ctx context.Context,
	userID, chain, asset string,
) (*domain.CryptoWallet, error) {
	
	return uc.walletRepo.GetUserWalletByChainAsset(ctx, userID, chain, asset)
}

// GetUserWallets retrieves all wallets for a user
func (uc *WalletUsecase) GetUserWallets(
	ctx context.Context,
	userID string,
	chainFilter, assetFilter *string,
) ([]*domain.CryptoWallet, error) {
	
	// Get all user wallets
	wallets, err := uc.walletRepo. GetUserWallets(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallets:  %w", err)
	}
	
	// Apply filters if provided
	var filtered []*domain.CryptoWallet
	for _, wallet := range wallets {
		if chainFilter != nil && wallet.Chain != *chainFilter {
			continue
		}
		if assetFilter != nil && wallet. Asset != *assetFilter {
			continue
		}
		filtered = append(filtered, wallet)
	}
	
	return filtered, nil
}

// GetWalletBalance gets current balance (cached or fresh from blockchain)
func (uc *WalletUsecase) GetWalletBalance(
	ctx context.Context,
	userID, chainName, assetCode string,
	forceRefresh bool,
) (*domain.WalletBalance, error) {
	
	// Get wallet
	wallet, err := uc. walletRepo.GetUserPrimaryWallet(ctx, userID, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}
	
	// Get asset configuration
	asset := assetFromCode(assetCode)
	if asset == nil {
		return nil, fmt.Errorf("unsupported asset:  %s", assetCode)
	}
	
	// Check if we need to refresh from blockchain
	shouldRefresh := forceRefresh ||
		wallet.LastBalanceUpdate == nil ||
		time.Since(*wallet.LastBalanceUpdate) > 5*time.Minute
	
	if shouldRefresh {
		// Get fresh balance from blockchain
		chain, err := uc.chainRegistry.Get(chainName)
		if err != nil {
			return nil, err
		}
		
		// ✅ Updated to match domain. Chain interface
		balance, err := chain.GetBalance(ctx, wallet. Address, asset)
		if err != nil {
			uc.logger. Warn("Failed to fetch blockchain balance, using cached",
				zap.Error(err),
				zap.String("address", wallet.Address),
			)
		} else {
			// Update cached balance
			wallet.Balance = balance. Amount
			if err := uc.walletRepo. UpdateBalance(ctx, wallet. ID, balance. Amount); err != nil {
				uc.logger.Error("Failed to update balance cache", zap.Error(err))
			}
		}
	}
	
	return &domain.WalletBalance{
		WalletID:         wallet.ID,
		Address:          wallet.Address,
		Chain:            wallet.Chain,
		Asset:            wallet.Asset,
		Balance:          wallet.Balance,
		Decimals:         asset.Decimals,
		BalanceFormatted: formatBalance(wallet. Balance, asset.Decimals, assetCode),
		UpdatedAt:        time.Now(),
	}, nil
}

// RefreshBalance forces a balance refresh from blockchain
func (uc *WalletUsecase) RefreshBalance(
	ctx context.Context,
	walletID int64,
	userID string,
) (*big.Int, *big.Int, error) {
	
	// Get wallet
	wallet, err := uc.walletRepo.GetByID(ctx, walletID)
	if err != nil {
		return nil, nil, fmt. Errorf("wallet not found:  %w", err)
	}
	
	// Verify ownership
	if wallet.UserID != userID {
		return nil, nil, fmt.Errorf("unauthorized:  wallet does not belong to user")
	}
	
	// Get asset configuration
	asset := assetFromCode(wallet.Asset)
	if asset == nil {
		return nil, nil, fmt.Errorf("unsupported asset: %s", wallet.Asset)
	}
	
	previousBalance := new(big.Int).Set(wallet.Balance)
	
	// Get blockchain implementation
	chain, err := uc.chainRegistry.Get(wallet. Chain)
	if err != nil {
		return nil, nil, err
	}
	
	// ✅ Fetch fresh balance using domain. Chain interface
	balanceResp, err := chain.GetBalance(ctx, wallet.Address, asset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch balance: %w", err)
	}
	
	// Update database
	if err := uc. walletRepo.UpdateBalance(ctx, walletID, balanceResp.Amount); err != nil {
		return nil, nil, fmt.Errorf("failed to update balance: %w", err)
	}
	
	uc.logger.Info("Balance refreshed",
		zap.Int64("wallet_id", walletID),
		zap.String("previous", previousBalance.String()),
		zap.String("current", balanceResp.Amount.String()),
	)
	
	return balanceResp. Amount, previousBalance, nil
}

// GetWalletByAddress retrieves wallet by blockchain address
func (uc *WalletUsecase) GetWalletByAddress(
	ctx context.Context,
	address string,
) (*domain.CryptoWallet, error) {
	return uc.walletRepo.GetByAddress(ctx, address)
}

// ValidateAddress validates if an address is valid for the chain
func (uc *WalletUsecase) ValidateAddress(
	ctx context.Context,
	chainName, address string,
) (bool, string, error) {
	
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return false, "", fmt.Errorf("unsupported chain: %w", err)
	}
	
	// ✅ Updated to match domain. Chain interface (returns error)
	err = chain.ValidateAddress(address)
	if err != nil {
		return false, fmt.Sprintf("Invalid address: %s", err. Error()), nil
	}
	
	return true, "Address is valid", nil
}
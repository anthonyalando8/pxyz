// internal/usecase/deposit_usecase.go
package usecase

import (
	"context"
	registry "crypto-service/internal/chains/registry"
	"crypto-service/internal/domain"
	"crypto-service/internal/repository"
	"crypto-service/internal/security"
	"crypto-service/pkg/utils"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DepositUsecase struct {
	depositRepo     *repository.CryptoDepositRepository
	walletRepo      *repository.CryptoWalletRepository
	transactionRepo *repository.CryptoTransactionRepository
	chainRegistry   *registry.Registry
	encryption      *security. Encryption
	logger          *zap. Logger
}

func NewDepositUsecase(
	depositRepo *repository.CryptoDepositRepository,
	walletRepo *repository.CryptoWalletRepository,
	transactionRepo *repository.CryptoTransactionRepository,
	chainRegistry *registry. Registry,
	encryption *security.Encryption,
	logger *zap.Logger,
) *DepositUsecase {
	return &DepositUsecase{
		depositRepo:     depositRepo,
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		chainRegistry:   chainRegistry,
		encryption:      encryption,
		logger:          logger,
	}
}

// ============================================================================
// DEPOSIT MONITORING
// ============================================================================

// MonitorDeposits scans blockchain for incoming deposits
// This should be called by a background worker/cron job
func (uc *DepositUsecase) MonitorDeposits(ctx context.Context, chainName string) error {
	uc.logger.Info("Starting deposit monitoring", zap.String("chain", chainName))
	
	// Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return fmt.Errorf("unsupported chain: %w", err)
	}
	
	// Get wallets that need deposit checking
	wallets, err := uc.walletRepo.GetWalletsForDepositCheck(ctx, 100) // Check 100 wallets at a time
	if err != nil {
		return fmt.Errorf("failed to get wallets:  %w", err)
	}
	
	uc.logger.Info("Checking deposits for wallets", zap.Int("count", len(wallets)))
	
	for _, wallet := range wallets {
		// Skip if not for this chain
		if wallet.Chain != chainName {
			continue
		}
		
		// Check for deposits on this wallet
		if err := uc.checkWalletDeposits(ctx, wallet, chain); err != nil {
			uc.logger.Error("Failed to check wallet deposits",
				zap.Error(err),
				zap.Int64("wallet_id", wallet.ID),
				zap.String("address", wallet.Address),
			)
			// Continue with other wallets even if one fails
			continue
		}
		
		// Update last deposit check timestamp
		uc.walletRepo.UpdateLastDepositCheck(ctx, wallet. ID)
	}
	
	return nil
}

// checkWalletDeposits checks a single wallet for new deposits
func (uc *DepositUsecase) checkWalletDeposits(
	ctx context.Context,
	wallet *domain.CryptoWallet,
	chain domain.Chain,
) error {
	
	// Get asset configuration
	asset := utils.AssetFromChainAndCode(wallet.Chain, wallet.Asset)
	if asset == nil {
		return fmt.Errorf("unsupported asset: %s", wallet.Asset)
	}
	privateKey, err := uc.encryption.Decrypt(wallet.EncryptedPrivateKey)
	if err != nil {

		return  fmt.Errorf("failed to decrypt private key: %w", err)
	}
	// Get current balance from blockchain
	balance, err := chain.GetBalance(ctx, wallet. Address, privateKey, asset)
	if err != nil {
		return fmt. Errorf("failed to get balance: %w", err)
	}
	
	// Check if balance increased (potential deposit)
	if balance.Amount.Cmp(wallet.Balance) > 0 {
		uc.logger.Info("Balance increased, potential deposit detected",
			zap.String("address", wallet.Address),
			zap.String("previous", wallet.Balance.String()),
			zap.String("current", balance.Amount.String()),
		)
		
		// Calculate deposit amount
		depositAmount := new(big.Int).Sub(balance.Amount, wallet.Balance)
		
		// TODO: Get actual transaction details from blockchain
		// For now, we create a deposit record based on balance change
		// In production, you'd fetch actual transaction from blockchain explorer/API
		
		deposit := &domain.CryptoDeposit{
			DepositID:              uuid.New().String(),
			WalletID:               wallet.ID,
			UserID:                 wallet.UserID,
			Chain:                  wallet.Chain,
			Asset:                  wallet.Asset,
			FromAddress:            "unknown", // Would get from blockchain tx
			ToAddress:              wallet. Address,
			Amount:                 depositAmount,
			TxHash:                 "pending_lookup", // Would get from blockchain
			BlockNumber:            0,                 // Would get from blockchain
			Confirmations:          0,
			RequiredConfirmations:   utils.GetRequiredConfirmations(wallet. Chain),
			Status:                 domain. DepositStatusDetected,
			UserNotified:           false,
			NotificationSent:       false,
			DetectedAt:             time.Now(),
		}
		
		// Check if deposit already exists
		exists, _ := uc.depositRepo. DepositExists(ctx, deposit. TxHash, deposit.ToAddress)
		if !exists {
			// Save deposit
			if err := uc. depositRepo.Create(ctx, deposit); err != nil {
				return fmt.Errorf("failed to create deposit: %w", err)
			}
			
			uc.logger.Info("New deposit recorded",
				zap.String("deposit_id", deposit.DepositID),
				zap.String("amount", depositAmount.String()),
			)
		}
		
		// Update wallet balance cache
		uc.walletRepo.UpdateBalance(ctx, wallet. ID, balance.Amount)
	}
	
	return nil
}

// ScanBlockchainForDeposits scans blockchain for transactions to our wallets
// More accurate than balance checking
func (uc *DepositUsecase) ScanBlockchainForDeposits(
	ctx context.Context,
	chainName string,
	fromBlock, toBlock int64,
) error {
	
	uc.logger.Info("Scanning blockchain for deposits",
		zap.String("chain", chainName),
		zap.Int64("from_block", fromBlock),
		zap.Int64("to_block", toBlock),
	)
	
	// Get blockchain implementation
	chain, err := uc. chainRegistry.Get(chainName)
	if err != nil {
		return fmt.Errorf("unsupported chain: %w", err)
	}
	_ = chain
	
	// Get all active wallets for this chain
	wallets, err := uc.walletRepo. GetWalletsByChain(ctx, chainName)
	if err != nil {
		return fmt.Errorf("failed to get wallets: %w", err)
	}
	
	// Build address map for quick lookup
	addressMap := make(map[string]*domain.CryptoWallet)
	for _, wallet := range wallets {
		addressMap[wallet.Address] = wallet
	}
	
	// TODO: Implement blockchain scanning logic
	// This would use chain-specific APIs to fetch transactions
	// For TRON:  Use TronGrid API to get transactions by block range
	// For BTC: Use block explorer API or run full node
	
	uc.logger.Info("Blockchain scan completed",
		zap.Int("wallets_checked", len(wallets)),
	)
	
	return nil
}

// ============================================================================
// DEPOSIT PROCESSING
// ============================================================================

// ProcessPendingDeposits processes deposits waiting for confirmations
func (uc *DepositUsecase) ProcessPendingDeposits(ctx context.Context) error {
	// Get all pending deposits
	deposits, err := uc.depositRepo. GetPendingDeposits(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending deposits:  %w", err)
	}
	
	uc.logger.Info("Processing pending deposits", zap.Int("count", len(deposits)))
	
	for _, deposit := range deposits {
		if err := uc.processDeposit(ctx, deposit); err != nil {
			uc.logger. Error("Failed to process deposit",
				zap.Error(err),
				zap.String("deposit_id", deposit. DepositID),
			)
			continue
		}
	}
	
	return nil
}

// processDeposit processes a single deposit
func (uc *DepositUsecase) processDeposit(ctx context. Context, deposit *domain.CryptoDeposit) error {
	// Get blockchain implementation
	chain, err := uc.chainRegistry.Get(deposit. Chain)
	if err != nil {
		return fmt.Errorf("unsupported chain: %w", err)
	}
	
	// Get transaction details from blockchain
	tx, err := chain.GetTransaction(ctx, deposit.TxHash)
	if err != nil {
		uc.logger. Warn("Failed to get transaction from blockchain",
			zap.Error(err),
			zap.String("tx_hash", deposit. TxHash),
		)
		return nil // Don't fail, just skip for now
	}
	
	// Update confirmations
	deposit.Confirmations = tx.Confirmations
	deposit.BlockNumber = *tx.BlockNumber
	deposit. BlockTimestamp = &tx.Timestamp
	
	// Check if enough confirmations
	if deposit. Confirmations >= deposit.RequiredConfirmations {
		// Mark as confirmed
		deposit.Status = domain.DepositStatusConfirmed
		
		// Credit user account
		if err := uc. creditDeposit(ctx, deposit); err != nil {
			return fmt.Errorf("failed to credit deposit: %w", err)
		}
	} else {
		// Still pending, update status
		deposit.Status = domain.DepositStatusPending
	}
	
	// Update deposit record
	if err := uc. depositRepo.Update(ctx, deposit); err != nil {
		return fmt.Errorf("failed to update deposit: %w", err)
	}
	
	return nil
}

// creditDeposit credits a confirmed deposit to user's account
func (uc *DepositUsecase) creditDeposit(ctx context.Context, deposit *domain.CryptoDeposit) error {
	uc.logger.Info("Crediting deposit to user",
		zap.String("deposit_id", deposit. DepositID),
		zap.String("user_id", deposit.UserID),
		zap.String("amount", deposit.Amount.String()),
	)
	
	// Get wallet
	wallet, err := uc. walletRepo.GetByID(ctx, deposit.WalletID)
	if err != nil {
		return fmt.Errorf("wallet not found: %w", err)
	}
	
	// Create transaction record for the deposit
	tx := &domain.CryptoTransaction{
		TransactionID:           uuid.New().String(),
		UserID:                 deposit.UserID,
		Type:                   domain.TransactionTypeDeposit,
		Chain:                  deposit.Chain,
		Asset:                  deposit.Asset,
		FromAddress:            deposit.FromAddress,
		ToWalletID:             &deposit.WalletID,
		ToAddress:              deposit.ToAddress,
		IsInternal:             false, // External deposit
		Amount:                 deposit.Amount,
		NetworkFee:             big.NewInt(0), // User doesn't pay for deposits
		PlatformFee:            big.NewInt(0), // No fee on deposits
		TotalFee:               big.NewInt(0),
		TxHash:                 &deposit.TxHash,
		BlockNumber:            &deposit.BlockNumber,
		BlockTimestamp:         deposit.BlockTimestamp,
		Confirmations:          deposit.Confirmations,
		RequiredConfirmations:   deposit.RequiredConfirmations,
		Status:                 domain.TransactionStatusConfirmed,
		InitiatedAt:            deposit.DetectedAt,
	}
	
	now := time.Now()
	tx. ConfirmedAt = &now
	tx.CompletedAt = &now
	
	// Save transaction
	if err := uc. transactionRepo.Create(ctx, tx); err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	
	// Update wallet balance
	newBalance := new(big.Int).Add(wallet.Balance, deposit.Amount)
	if err := uc.walletRepo.UpdateBalance(ctx, wallet.ID, newBalance); err != nil {
		return fmt. Errorf("failed to update wallet balance: %w", err)
	}
	
	// Mark deposit as credited
	if err := uc.depositRepo. MarkAsCredited(ctx, deposit.ID, tx.ID); err != nil {
		return fmt.Errorf("failed to mark deposit as credited: %w", err)
	}
	
	uc.logger.Info("Deposit credited successfully",
		zap.String("deposit_id", deposit.DepositID),
		zap.String("tx_id", tx.TransactionID),
		zap.String("new_balance", newBalance.String()),
	)
	
	// TODO:  Trigger notification to user
	// This would call cashier service or notification service
	
	return nil
}

// ============================================================================
// USER-FACING METHODS
// ============================================================================

// GetUserDeposits retrieves user's deposit history
func (uc *DepositUsecase) GetUserDeposits(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*domain.CryptoDeposit, error) {
	return uc.depositRepo.GetUserDeposits(ctx, userID, limit, offset)
}

// GetDeposit retrieves a specific deposit
func (uc *DepositUsecase) GetDeposit(
	ctx context.Context,
	depositID, userID string,
) (*domain.CryptoDeposit, error) {
	
	deposit, err := uc.depositRepo.GetByDepositID(ctx, depositID)
	if err != nil {
		return nil, fmt.Errorf("deposit not found: %w", err)
	}
	
	// Verify ownership
	if deposit.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}
	
	return deposit, nil
}

// GetPendingDeposits retrieves user's pending deposits
func (uc *DepositUsecase) GetPendingDeposits(
	ctx context.Context,
	userID string,
) ([]*domain.CryptoDeposit, error) {
	
	// Get all user deposits
	allDeposits, err := uc.depositRepo.GetUserDeposits(ctx, userID, 100, 0)
	if err != nil {
		return nil, err
	}
	
	// Filter pending/detected
	var pending []*domain.CryptoDeposit
	for _, deposit := range allDeposits {
		if deposit.Status == domain. DepositStatusDetected || deposit.Status == domain.DepositStatusPending {
			pending = append(pending, deposit)
		}
	}
	
	return pending, nil
}

// GetDepositAddress gets user's deposit address for asset
func (uc *DepositUsecase) GetDepositAddress(
	ctx context.Context,
	userID, chainName, assetCode string,
) (string, error) {
	
	// Get or create wallet
	wallet, err := uc.walletRepo.GetUserPrimaryWallet(ctx, userID, chainName, assetCode)
	if err != nil {
		return "", fmt.Errorf("no wallet found: %w", err)
	}
	
	return wallet.Address, nil
}

// ============================================================================
// NOTIFICATION MANAGEMENT
// ============================================================================

// GetUnnotifiedDeposits gets deposits that need user notification
func (uc *DepositUsecase) GetUnnotifiedDeposits(ctx context.Context) ([]*domain.CryptoDeposit, error) {
	return uc.depositRepo.GetUnnotifiedDeposits(ctx)
}

// MarkDepositAsNotified marks a deposit as user notified
func (uc *DepositUsecase) MarkDepositAsNotified(ctx context.Context, depositID int64) error {
	return uc.depositRepo.MarkAsNotified(ctx, depositID)
}

// NotifyPendingDeposits sends notifications for unnotified deposits
// This would be called by a background worker
func (uc *DepositUsecase) NotifyPendingDeposits(ctx context. Context) error {
	deposits, err := uc.GetUnnotifiedDeposits(ctx)
	if err != nil {
		return err
	}
	
	uc.logger.Info("Notifying users of deposits", zap.Int("count", len(deposits)))
	
	for _, deposit := range deposits {
		// TODO: Send notification via cashier service
		// For now, just mark as notified
		
		notification := &domain. DepositNotification{
			UserID:        deposit.UserID,
			DepositID:     deposit. DepositID,
			Chain:         deposit.Chain,
			Asset:         deposit.Asset,
			Amount:        utils.FormatAmount(deposit.Amount, deposit.Asset),
			TxHash:        deposit.TxHash,
			Confirmations: deposit.Confirmations,
			Required:      deposit.RequiredConfirmations,
			Status:        deposit.Status,
			DetectedAt:    deposit.DetectedAt,
		}
		
		uc.logger.Info("Deposit notification",
			zap.String("user_id", notification.UserID),
			zap.String("amount", notification.Amount),
			zap.Int("confirmations", notification.Confirmations),
		)
		
		// Mark as notified
		if err := uc. MarkDepositAsNotified(ctx, deposit.ID); err != nil {
			uc.logger.Error("Failed to mark as notified", zap.Error(err))
		}
	}
	
	return nil
}
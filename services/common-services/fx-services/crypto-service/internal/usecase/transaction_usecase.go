// internal/usecase/transaction_usecase.go
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

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TransactionUsecase struct {
	transactionRepo *repository.CryptoTransactionRepository
	walletRepo      *repository.CryptoWalletRepository
	chainRegistry   *registry.Registry
	encryption      *security. Encryption
	systemUsecase   *SystemUsecase
	logger          *zap.Logger
}

func NewTransactionUsecase(
	transactionRepo *repository.CryptoTransactionRepository,
	walletRepo *repository.CryptoWalletRepository,
	chainRegistry *registry.Registry,
	encryption *security.Encryption,
	systemUsecase *SystemUsecase,
	logger *zap.Logger,
) *TransactionUsecase {
	return &TransactionUsecase{
		transactionRepo: transactionRepo,
		walletRepo:       walletRepo,
		chainRegistry:   chainRegistry,
		encryption:      encryption,
		systemUsecase:   systemUsecase,
		logger:          logger,
	}
}

// ============================================================================
// NETWORK FEE ESTIMATION (for accounting module)
// ============================================================================

// EstimateNetworkFee estimates blockchain network fee for accounting
func (uc *TransactionUsecase) EstimateNetworkFee(
	ctx context.Context,
	chainName, assetCode, amount string,
	toAddress string, // Optional, can be empty
) (*NetworkFeeEstimate, error) {
	
	uc.logger.Info("Estimating network fee",
		zap.String("chain", chainName),
		zap.String("asset", assetCode),
		zap.String("amount", amount))
	
	// Get system hot wallet (all withdrawals come from here)
	systemWallet, err := uc.systemUsecase.GetSystemWallet(ctx, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("system wallet not found: %w", err)
	}
	
	// Parse amount
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount format")
	}
	
	// Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return nil, fmt.Errorf("unsupported chain: %w", err)
	}
	
	// Use a dummy address if none provided
	if toAddress == "" {
		toAddress = systemWallet.Address // Use self as dummy
	}
	
	// Validate destination address
	if err := chain.ValidateAddress(toAddress); err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}
	
	// Estimate network fee from blockchain
	feeEstimate, err := chain.EstimateFee(ctx, &domain.TransactionRequest{
		From:   systemWallet.Address, // ✅ From system wallet
		To:     toAddress,
		Asset:  assetFromCode(assetCode),
		Amount: amountBig,
		Priority: domain.TxPriorityNormal,
	})
	
	if err != nil {
		uc.logger. Warn("Failed to estimate fee from chain, using defaults",
			zap.Error(err),
			zap.String("chain", chainName))
		
		// Return conservative defaults
		feeEstimate = uc.getDefaultFeeEstimate(chainName, assetCode)
	}
	
	estimate := &NetworkFeeEstimate{
		Chain:          chainName,
		Asset:          assetCode,
		FeeAmount:      feeEstimate.Amount,
		FeeCurrency:    feeEstimate.Currency,
		FeeFormatted:   formatAmount(feeEstimate.Amount, feeEstimate.Currency),
		EstimatedAt:    time.Now(),
		ValidFor:       5 * time.Minute,
	}
	
	uc. logger.Info("Network fee estimated",
		zap.String("fee", estimate.FeeFormatted),
		zap.String("currency", estimate.FeeCurrency))
	
	return estimate, nil
}

// GetWithdrawalQuote provides fee estimate (for accounting to show users)
func (uc *TransactionUsecase) GetWithdrawalQuote(
	ctx context.Context,
	chainName, assetCode, amount, toAddress string,
) (*WithdrawalQuote, error) {
	
	// Just estimate network fee - accounting handles platform fee
	networkFee, err := uc. EstimateNetworkFee(ctx, chainName, assetCode, amount, toAddress)
	if err != nil {
		return nil, err
	}
	
	amountBig, _ := new(big.Int).SetString(amount, 10)
	
	quote := &WithdrawalQuote{
		QuoteID:            uuid.New().String(),
		Chain:              chainName,
		Asset:              assetCode,
		Amount:             amountBig,
		NetworkFee:         networkFee. FeeAmount,
		NetworkFeeCurrency: networkFee.FeeCurrency,
		Explanation:        fmt.Sprintf("Network fee: %s %s", networkFee.FeeFormatted, networkFee. FeeCurrency),
		ValidUntil:         time.Now().Add(5 * time.Minute),
	}
	
	return quote, nil
}

// ============================================================================
// WITHDRAWAL (from system hot wallet to external address)
// ============================================================================

// Withdraw sends crypto from SYSTEM hot wallet to external address
// NOTE:  Accounting module should have already: 
//   1. Verified user has sufficient virtual balance
//   2. Deducted amount + fees from user's virtual wallet
//   3. Called this method to execute blockchain transaction
func (uc *TransactionUsecase) Withdraw(
	ctx context.Context,
	accountingTxID  string, // From accounting module for idempotency
	chainName, assetCode, amount, toAddress, memo string,
	userID string, // For tracking who requested it
) (*domain.CryptoTransaction, error) {
	
	uc.logger.Info("Processing withdrawal from system hot wallet",
		zap.String("accounting_tx_id", accountingTxID),
		zap.String("user_id", userID),
		zap.String("chain", chainName),
		zap.String("asset", assetCode),
		zap.String("to", toAddress))
	
	// 1. Check for duplicate request (idempotency)
	existingTx, err := uc.transactionRepo. GetByAccountingTxID(ctx, accountingTxID)
	if err == nil && existingTx != nil {
		uc.logger.Info("Duplicate withdrawal request detected",
			zap.String("accounting_tx_id", accountingTxID),
			zap.String("existing_tx", existingTx.TransactionID))
		return existingTx, nil
	}
	
	// 2. Get system hot wallet
	systemWallet, err := uc.systemUsecase.GetSystemWallet(ctx, chainName, assetCode)
	if err != nil {
		return nil, fmt. Errorf("system wallet not found: %w", err)
	}
	
	// 3. Parse amount
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount format")
	}
	
	// 4. Estimate network fee
	feeEstimate, err := uc.EstimateNetworkFee(ctx, chainName, assetCode, amount, toAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to estimate fee: %w", err)
	}
	
	// 5. Check system wallet has enough (amount + network fee)
	totalNeeded := new(big.Int).Add(amountBig, feeEstimate.FeeAmount)
	if systemWallet.Balance.Cmp(totalNeeded) < 0 {
		return nil, fmt.Errorf("insufficient system wallet balance:  have %s, need %s",
			formatAmount(systemWallet.Balance, assetCode),
			formatAmount(totalNeeded, assetCode))
	}
	
	// 6. Create transaction record
	tx := &domain. CryptoTransaction{
		TransactionID:          uuid.New().String(),
		AccountingTxID:         &accountingTxID, // Link to accounting module
		UserID:                 userID,      // Who requested it
		Type:                   domain.TransactionTypeWithdrawal,
		Chain:                  chainName,
		Asset:                  assetCode,
		FromWalletID:           &systemWallet.ID,
		FromAddress:            systemWallet. Address, // ✅ From system wallet
		ToAddress:              toAddress,            // To external address
		IsInternal:             false,
		Amount:                 amountBig,
		NetworkFee:             feeEstimate.FeeAmount,
		NetworkFeeCurrency:     &feeEstimate.FeeCurrency,
		PlatformFee:            big.NewInt(0), // Handled by accounting
		TotalFee:               feeEstimate.FeeAmount,
		Status:                 domain.TransactionStatusPending,
		RequiredConfirmations:  getRequiredConfirmations(chainName),
		Memo:                   &memo,
		InitiatedAt:            time.Now(),
	}
	
	// 7. Save to database
	if err := uc.transactionRepo. Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	
	// 8. Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, fmt.Sprintf("Unsupported chain: %v", err))
		return nil, err
	}
	
	// 9. Decrypt system wallet private key
	privateKey, err := uc.encryption.Decrypt(systemWallet.EncryptedPrivateKey)
	if err != nil {
		uc.transactionRepo.MarkAsFailed(ctx, tx. ID, "Failed to decrypt system wallet key")
		return nil, fmt.Errorf("failed to decrypt key: %w", err)
	}
	
	// 10. Execute blockchain transaction
	uc.transactionRepo.UpdateStatus(ctx, tx.ID, domain.TransactionStatusBroadcasting, nil)
	
	txResult, err := chain.Send(ctx, &domain.TransactionRequest{
		From:       systemWallet.Address, // ✅ System wallet signs
		To:         toAddress,
		Asset:      assetFromCode(assetCode),
		Amount:     amountBig,
		PrivateKey: privateKey,
		Memo:       &memo,
		Priority:   domain.TxPriorityNormal,
	})
	
	if err != nil {
		uc.logger.Error("Blockchain transaction failed",
			zap. Error(err),
			zap.String("tx_id", tx.TransactionID),
			zap.String("accounting_tx_id", accountingTxID))
		
		uc.transactionRepo.MarkAsFailed(ctx, tx. ID, err.Error())
		
		// TODO:  Notify accounting module of failure for refund
		
		return nil, fmt.Errorf("blockchain transaction failed: %w", err)
	}
	
	// 11. Update transaction with blockchain details
	tx.TxHash = &txResult.TxHash
	tx.Status = domain.TransactionStatusBroadcasted
	now := time.Now()
	tx.BroadcastedAt = &now
	
	if err := uc.transactionRepo.Update(ctx, tx); err != nil {
		uc.logger.Error("Failed to update transaction", zap.Error(err))
	}
	
	// 12. Update system wallet balance
	actualCost := new(big.Int).Add(amountBig, txResult.Fee)
	newSystemBalance := new(big.Int).Sub(systemWallet.Balance, actualCost)
	uc.walletRepo.UpdateBalance(ctx, systemWallet.ID, newSystemBalance)
	
	uc.logger.Info("Withdrawal broadcasted successfully",
		zap. String("tx_hash", txResult.TxHash),
		zap.String("tx_id", tx.TransactionID),
		zap.String("accounting_tx_id", accountingTxID),
		zap.String("new_system_balance", newSystemBalance.String()))
	
	// TODO: Notify accounting module of success
	
	return tx, nil
}

// ============================================================================
// SWEEP (user address → system wallet)
// ============================================================================

// SweepUserWallet sweeps funds from user's deposit address to system wallet
func (uc *TransactionUsecase) SweepUserWallet(
	ctx context.Context,
	userID, chainName, assetCode string,
) (*domain.CryptoTransaction, error) {
	
	uc.logger.Info("Sweeping user wallet to system",
		zap.String("user_id", userID),
		zap.String("chain", chainName),
		zap.String("asset", assetCode))
	
	// 1. Get user's deposit address
	userWallet, err := uc.walletRepo.GetUserPrimaryWallet(ctx, userID, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("user wallet not found: %w", err)
	}
	
	// 2. Get system wallet
	systemWallet, err := uc.systemUsecase.GetSystemWallet(ctx, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("system wallet not found: %w", err)
	}
	
	// 3. Check if user wallet has balance worth sweeping
	minSweepAmount := getMinimumSweepAmount(chainName, assetCode)
	if userWallet.Balance.Cmp(minSweepAmount) < 0 {
		return nil, fmt.Errorf("balance too low to sweep: %s", userWallet.Balance.String())
	}
	
	// 4. Estimate network fee
	feeEstimate, err := uc.EstimateNetworkFee(ctx, chainName, assetCode, userWallet.Balance.String(), systemWallet.Address)
	if err != nil {
		return nil, err
	}
	
	// 5. Calculate amount to sweep (balance - fee)
	sweepAmount := new(big.Int).Sub(userWallet.Balance, feeEstimate.FeeAmount)
	if sweepAmount. Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("balance not enough to cover network fee")
	}
	
	// 6. Create transaction record
	tx := &domain.CryptoTransaction{
		TransactionID:           uuid.New().String(),
		UserID:                 userID,
		Type:                   domain.TransactionTypeSweep,
		Chain:                   chainName,
		Asset:                  assetCode,
		FromWalletID:           &userWallet.ID,
		FromAddress:            userWallet.Address, // User's deposit address
		ToWalletID:             &systemWallet.ID,
		ToAddress:              systemWallet.Address, // System wallet
		IsInternal:             true, // Internal sweep
		Amount:                 sweepAmount,
		NetworkFee:              feeEstimate.FeeAmount,
		NetworkFeeCurrency:     &feeEstimate. FeeCurrency,
		PlatformFee:            big. NewInt(0),
		TotalFee:               feeEstimate.FeeAmount,
		Status:                 domain.TransactionStatusPending,
		RequiredConfirmations:  getRequiredConfirmations(chainName),
		InitiatedAt:            time.Now(),
	}
	
	// 7. Save to database
	if err := uc.transactionRepo.Create(ctx, tx); err != nil {
		return nil, err
	}
	
	// 8. Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		uc.transactionRepo. MarkAsFailed(ctx, tx.ID, fmt.Sprintf("Unsupported chain: %v", err))
		return nil, err
	}
	
	// 9. Decrypt user wallet private key
	privateKey, err := uc.encryption.Decrypt(userWallet.EncryptedPrivateKey)
	if err != nil {
		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, "Failed to decrypt key")
		return nil, err
	}
	
	// 10. Execute blockchain transaction
	uc.transactionRepo.UpdateStatus(ctx, tx.ID, domain.TransactionStatusBroadcasting, nil)
	
	txResult, err := chain. Send(ctx, &domain.TransactionRequest{
		From:       userWallet.Address, // ✅ From user's deposit address
		To:         systemWallet.Address, // ✅ To system wallet
		Asset:      assetFromCode(assetCode),
		Amount:     sweepAmount,
		PrivateKey: privateKey,
		Priority:   domain.TxPriorityLow, // Low priority for sweeps
	})
	
	if err != nil {
		uc.logger.Error("Sweep transaction failed",
			zap. Error(err),
			zap.String("user_id", userID))
		
		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, err.Error())
		return nil, err
	}
	
	// 11. Update transaction
	tx.TxHash = &txResult.TxHash
	tx.Status = domain.TransactionStatusBroadcasted
	now := time.Now()
	tx.BroadcastedAt = &now
	uc.transactionRepo.Update(ctx, tx)
	
	// 12. Update balances
	uc.walletRepo.UpdateBalance(ctx, userWallet.ID, big.NewInt(0)) // User address now empty
	newSystemBalance := new(big.Int).Add(systemWallet.Balance, sweepAmount)
	uc.walletRepo.UpdateBalance(ctx, systemWallet.ID, newSystemBalance)
	
	uc.logger.Info("Sweep completed successfully",
		zap.String("tx_hash", txResult. TxHash),
		zap.String("amount_swept", sweepAmount.String()),
		zap.String("new_system_balance", newSystemBalance.String()))
	
	return tx, nil
}

// SweepAllUsers sweeps all user wallets for a chain/asset (batch operation)
func (uc *TransactionUsecase) SweepAllUsers(
	ctx context.Context,
	chainName, assetCode string,
) ([]*domain.CryptoTransaction, error) {
	
	uc.logger.Info("Starting batch sweep",
		zap.String("chain", chainName),
		zap.String("asset", assetCode))
	
	// Get all user wallets with balance
	wallets, err := uc.walletRepo.GetWalletsWithBalance(ctx, chainName, assetCode, getMinimumSweepAmount(chainName, assetCode))
	if err != nil {
		return nil, err
	}
	
	uc.logger.Info("Found wallets to sweep", zap.Int("count", len(wallets)))
	
	var swept []*domain.CryptoTransaction
	var failed int
	
	for _, wallet := range wallets {
		tx, err := uc.SweepUserWallet(ctx, wallet.UserID, chainName, assetCode)
		if err != nil {
			uc.logger. Warn("Failed to sweep wallet",
				zap.Error(err),
				zap.String("user_id", wallet.UserID),
				zap.String("address", wallet.Address))
			failed++
			continue
		}
		swept = append(swept, tx)
	}
	
	uc.logger.Info("Batch sweep completed",
		zap.Int("successful", len(swept)),
		zap.Int("failed", failed))
	
	return swept, nil
}

// ============================================================================
// TRANSACTION QUERIES
// ============================================================================

func (uc *TransactionUsecase) GetUserTransactions(
	ctx context. Context,
	userID string,
	limit, offset int,
) ([]*domain.CryptoTransaction, error) {
	return uc.transactionRepo.GetUserTransactions(ctx, userID, limit, offset)
}

func (uc *TransactionUsecase) GetTransaction(
	ctx context.Context,
	transactionID string,
) (*domain.CryptoTransaction, error) {
	return uc.transactionRepo.GetByTransactionID(ctx, transactionID)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func (uc *TransactionUsecase) getDefaultFeeEstimate(chainName, assetCode string) *domain.Fee {
	// Conservative default fees
	defaults := map[string]map[string]int64{
		"TRON": {
			"TRX":   100000,   // 0.1 TRX
			"USDT": 5000000,  // 5 TRX equivalent
		},
		"BITCOIN": {
			"BTC":  5000, // 5000 satoshis
		},
	}
	
	if chainDefaults, ok := defaults[chainName]; ok {
		if fee, ok := chainDefaults[assetCode]; ok {
			return &domain.Fee{
				Amount:   big.NewInt(fee),
				Currency: chainName,
			}
		}
	}
	
	return &domain.Fee{
		Amount:   big.NewInt(10000),
		Currency: chainName,
	}
}

func getMinimumSweepAmount(chainName, assetCode string) *big.Int {
	// Minimum amount worth sweeping (to avoid dust)
	mins := map[string]map[string]int64{
		"TRON": {
			"TRX":  10000000, // 10 TRX
			"USDT": 10000000, // 10 USDT
		},
		"BITCOIN": {
			"BTC": 100000, // 0.001 BTC
		},
	}
	
	if chainMins, ok := mins[chainName]; ok {
		if min, ok := chainMins[assetCode]; ok {
			return big.NewInt(min)
		}
	}
	
	return big.NewInt(1000000) // Default 1 unit
}

// Types for accounting module integration

type NetworkFeeEstimate struct {
	Chain        string
	Asset        string
	FeeAmount    *big.Int
	FeeCurrency  string
	FeeFormatted string
	EstimatedAt  time.Time
	ValidFor     time.Duration
}

type WithdrawalQuote struct {
	QuoteID            string
	Chain              string
	Asset              string
	Amount             *big.Int
	NetworkFee         *big.Int
	NetworkFeeCurrency string
	Explanation        string
	ValidUntil         time.Time
}
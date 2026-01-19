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
	encryption      *security.Encryption
	systemUsecase   *SystemUsecase
	logger          *zap.Logger
}

func NewTransactionUsecase(
	transactionRepo *repository.CryptoTransactionRepository,
	walletRepo *repository.CryptoWalletRepository,
	chainRegistry *registry. Registry,
	encryption *security. Encryption,
	systemUsecase   *SystemUsecase,
	logger *zap.Logger,
) *TransactionUsecase {
	return &TransactionUsecase{
		transactionRepo: transactionRepo,
		walletRepo:      walletRepo,
		chainRegistry:   chainRegistry,
		encryption:      encryption,
		systemUsecase:   systemUsecase,
		logger:           logger,
	}
}

// GetWithdrawalQuote estimates fees for a withdrawal
func (uc *TransactionUsecase) GetWithdrawalQuote(
	ctx context.Context,
	userID, chainName, assetCode, amount, toAddress string,
) (*WithdrawalQuote, error) {
	
	uc.logger.Info("Getting withdrawal quote",
		zap.String("user_id", userID),
		zap.String("chain", chainName),
		zap.String("asset", assetCode),
		zap.String("amount", amount),
	)
	
	// 1. Get user's wallet
	wallet, err := uc.walletRepo.GetUserPrimaryWallet(ctx, userID, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}
	
	// 2. Parse amount
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount format")
	}
	
	// 3. Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return nil, fmt.Errorf("unsupported chain: %w", err)
	}
	
	// 4. Validate destination address
	err = chain.ValidateAddress(toAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid destination address")
	}
	
	// 5. Estimate network fees from blockchain
	feeEstimate, err := chain.EstimateFee(ctx, &domain.TransactionRequest{
		From:   wallet.Address,
		To:     toAddress,
		Asset:  assetFromCode(assetCode),
		Amount: amountBig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to estimate network fee: %w", err)
	}
	
	uc.logger.Info("Network fee estimated",
		zap.String("fee", feeEstimate.Amount.String()),
		zap.String("currency", feeEstimate.Currency),
	)
	
	// 6. Calculate platform fee (fixed for now, would come from accounting service)
	platformFee := uc.calculatePlatformFee(assetCode, amountBig)
	
	// 7. Calculate total
	totalFee := new(big.Int).Add(feeEstimate.Amount, platformFee)
	requiredBalance := new(big.Int).Add(amountBig, totalFee)
	
	// 8. Check if user has sufficient balance
	hasBalance := wallet.Balance.Cmp(requiredBalance) >= 0
	
	quote := &WithdrawalQuote{
		QuoteID:         uuid.New().String(),
		Amount:          amountBig,
		NetworkFee:      feeEstimate.Amount,
		NetworkFeeCurrency: feeEstimate.Currency,
		PlatformFee:     platformFee,
		TotalFee:        totalFee,
		TotalCost:       requiredBalance,
		UserHasBalance:  hasBalance,
		ValidUntil:      time.Now().Add(5 * time. Minute),
		Explanation:     fmt.Sprintf("Network fee: %s, Platform fee: %s", 
			formatAmount(feeEstimate.Amount, assetCode),
			formatAmount(platformFee, assetCode),
		),
	}
	
	return quote, nil
}

// Withdraw sends crypto to external address (BLOCKCHAIN TRANSACTION)
func (uc *TransactionUsecase) Withdraw(
	ctx context.Context,
	userID, chainName, assetCode, amount, toAddress, memo string,
) (*domain.CryptoTransaction, error) {
	
	uc.logger. Info("Processing withdrawal",
		zap.String("user_id", userID),
		zap.String("chain", chainName),
		zap.String("asset", assetCode),
		zap.String("to", toAddress),
	)
	
	// 1. Get wallet and validate
	wallet, err := uc.walletRepo.GetUserPrimaryWallet(ctx, userID, chainName, assetCode)
	if err != nil {
		return nil, fmt. Errorf("wallet not found: %w", err)
	}
	
	// 2. Parse amount
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	
	// 3. Get quote
	quote, err := uc.GetWithdrawalQuote(ctx, userID, chainName, assetCode, amount, toAddress)
	if err != nil {
		return nil, err
	}
	
	if ! quote.UserHasBalance {
		return nil, fmt.Errorf("insufficient balance:  need %s, have %s",
			formatAmount(quote.TotalCost, assetCode),
			formatAmount(wallet.Balance, assetCode),
		)
	}
	
	// 4. Create transaction record
	tx := &domain.CryptoTransaction{
		TransactionID:          uuid.New().String(),
		UserID:                userID,
		Type:                  domain.TransactionTypeWithdrawal,
		Chain:                 chainName,
		Asset:                 assetCode,
		FromWalletID:          &wallet.ID,
		FromAddress:           wallet.Address,
		ToAddress:             toAddress,
		IsInternal:            false, // External withdrawal
		Amount:                amountBig,
		NetworkFee:            quote.NetworkFee,
		NetworkFeeCurrency:    &quote.NetworkFeeCurrency,
		PlatformFee:           quote.PlatformFee,
		PlatformFeeCurrency:   &assetCode,
		TotalFee:              quote.TotalFee,
		Status:                domain.TransactionStatusPending,
		RequiredConfirmations: getRequiredConfirmations(chainName),
		Memo:                  &memo,
		InitiatedAt:           time.Now(),
	}
	
	// 5. Save to database
	if err := uc.transactionRepo. Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	
	// 6. Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return nil, err
	}
	
	// 7. Decrypt private key
	privateKey, err := uc.encryption.Decrypt(wallet.EncryptedPrivateKey)
	if err != nil {
		uc.transactionRepo. MarkAsFailed(ctx, tx.ID, "Failed to decrypt private key")
		return nil, fmt.Errorf("failed to decrypt key: %w", err)
	}
	
	// 8. Execute blockchain transaction
	uc.transactionRepo.UpdateStatus(ctx, tx.ID, domain.TransactionStatusBroadcasting, nil)
	
	txResult, err := chain.Send(ctx, &domain.TransactionRequest{
		From:       wallet.Address,
		To:         toAddress,
		Asset:       assetFromCode(assetCode),
		Amount:     amountBig,
		PrivateKey: privateKey,
		Memo:       &memo,
	})
	
	if err != nil {
		uc.logger.Error("Blockchain transaction failed",
			zap.Error(err),
			zap.String("tx_id", tx.TransactionID),
		)
		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, err.Error())
		return nil, fmt.Errorf("blockchain transaction failed: %w", err)
	}
	
	// 9. Update transaction with blockchain details
	tx.TxHash = &txResult.TxHash
	tx.Status = domain.TransactionStatusBroadcasted
	now := time.Now()
	tx.BroadcastedAt = &now
	
	if err := uc.transactionRepo.Update(ctx, tx); err != nil {
		uc. logger.Error("Failed to update transaction", zap.Error(err))
	}
	
	uc.logger.Info("Withdrawal broadcasted to blockchain",
		zap.String("tx_hash", txResult.TxHash),
		zap.String("tx_id", tx.TransactionID),
	)
	
	// 10. Update wallet balance (optimistic)
	newBalance := new(big.Int).Sub(wallet.Balance, quote.TotalCost)
	uc.walletRepo.UpdateBalance(ctx, wallet.ID, newBalance)
	
	return tx, nil
}

// InternalTransfer transfers between users (LEDGER ONLY, NO BLOCKCHAIN)
func (uc *TransactionUsecase) InternalTransfer(
	ctx context.Context,
	fromUserID, toUserID, chainName, assetCode, amount, memo string,
) (*domain.CryptoTransaction, error) {
	
	uc.logger.Info("Processing internal transfer",
		zap.String("from_user", fromUserID),
		zap.String("to_user", toUserID),
		zap.String("asset", assetCode),
	)
	
	// 1. Get both users' wallets
	fromWallet, err := uc.walletRepo.GetUserPrimaryWallet(ctx, fromUserID, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("sender wallet not found: %w", err)
	}
	
	toWallet, err := uc.walletRepo.GetUserPrimaryWallet(ctx, toUserID, chainName, assetCode)
	if err != nil {
		return nil, fmt. Errorf("recipient wallet not found: %w", err)
	}
	
	// 2. Parse amount
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	
	// 3. Estimate network fee (for display, not actually charged on blockchain)
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		return nil, err
	}
	
	estimatedNetworkFee, _ := chain.EstimateFee(ctx, &domain.TransactionRequest{
		From:   fromWallet.Address,
		To:     toWallet.Address,
		Asset:  assetFromCode(assetCode),
		Amount:  amountBig,
	})
	
	// 4. Calculate platform fee (small fee for internal transfer)
	platformFee := uc.calculateInternalTransferFee(assetCode, amountBig, estimatedNetworkFee.Amount)
	totalFee := platformFee
	
	// 5. Check balance
	requiredBalance := new(big.Int).Add(amountBig, totalFee)
	if fromWallet.Balance.Cmp(requiredBalance) < 0 {
		return nil, fmt.Errorf("insufficient balance")
	}
	
	// 6. Create transaction record (INTERNAL, no blockchain)
	tx := &domain.CryptoTransaction{
		TransactionID:       uuid.New().String(),
		UserID:              fromUserID,
		Type:                domain.TransactionTypeInternalTransfer,
		Chain:                chainName,
		Asset:                assetCode,
		FromWalletID:        &fromWallet.ID,
		FromAddress:         fromWallet.Address,
		ToWalletID:          &toWallet.ID,
		ToAddress:           toWallet.Address,
		IsInternal:          true, // ✅ Internal transfer (no blockchain)
		Amount:              amountBig,
		NetworkFee:          big.NewInt(0), // No actual network fee
		PlatformFee:         platformFee,
		PlatformFeeCurrency: &assetCode,
		TotalFee:            totalFee,
		Status:               domain.TransactionStatusCompleted, // Instant
		Memo:                &memo,
		InitiatedAt:         time. Now(),
	}
	
	now := time.Now()
	tx.CompletedAt = &now
	
	// 7. Save transaction
	if err := uc.transactionRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	
	// 8. Update balances (ledger-based)
	fromNewBalance := new(big.Int).Sub(fromWallet.Balance, requiredBalance)
	toNewBalance := new(big.Int).Add(toWallet.Balance, amountBig)
	
	if err := uc.walletRepo.UpdateBalance(ctx, fromWallet.ID, fromNewBalance); err != nil {
		return nil, fmt.Errorf("failed to update sender balance: %w", err)
	}
	
	if err := uc.walletRepo. UpdateBalance(ctx, toWallet.ID, toNewBalance); err != nil {
		return nil, fmt.Errorf("failed to update recipient balance: %w", err)
	}
	
	uc.logger.Info("Internal transfer completed",
		zap.String("tx_id", tx.TransactionID),
		zap.String("from", fromWallet.Address),
		zap.String("to", toWallet.Address),
	)
	
	return tx, nil
}

// GetUserTransactions retrieves transaction history
func (uc *TransactionUsecase) GetUserTransactions(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]*domain.CryptoTransaction, error) {
	return uc.transactionRepo.GetUserTransactions(ctx, userID, limit, offset)
}

// GetTransaction retrieves a specific transaction
func (uc *TransactionUsecase) GetTransaction(
	ctx context.Context,
	transactionID, userID string,
) (*domain.CryptoTransaction, error) {
	
	tx, err := uc.transactionRepo.GetByTransactionID(ctx, transactionID)
	if err != nil {
		return nil, err
	}
	
	// Verify ownership
	if tx.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}
	
	return tx, nil
}

// CancelTransaction cancels a pending transaction
func (uc *TransactionUsecase) CancelTransaction(
	ctx context.Context,
	transactionID, userID, reason string,
) error {
	
	tx, err := uc.GetTransaction(ctx, transactionID, userID)
	if err != nil {
		return err
	}
	
	// Can only cancel pending transactions
	if tx.Status != domain.TransactionStatusPending {
		return fmt.Errorf("can only cancel pending transactions")
	}
	
	return uc.transactionRepo.UpdateStatus(ctx, tx.ID, domain.TransactionStatusCancelled, &reason)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func (uc *TransactionUsecase) calculatePlatformFee(assetCode string, amount *big. Int) *big.Int {
	// Fixed fee for now (would come from accounting service)
	// Example: 1 USDT = 1,000,000 SUN
	fixedFees := map[string]int64{
		"USDT": 1000000,  // 1 USDT
		"TRX":  100000,   // 0.1 TRX
		"BTC":  1000,     // 0.00001 BTC
	}
	
	if fee, ok := fixedFees[assetCode]; ok {
		return big.NewInt(fee)
	}
	
	return big.NewInt(0)
}

func (uc *TransactionUsecase) calculateInternalTransferFee(
	assetCode string,
	amount *big.Int,
	estimatedNetworkFee *big. Int,
) *big.Int {
	// CEO requirement: charge estimated network fee + small markup
	// Example: network fee + 5% platform fee
	
	markup := new(big.Float).Mul(
		new(big.Float).SetInt(estimatedNetworkFee),
		big.NewFloat(1.05), // 5% markup
	)
	
	fee, _ := markup.Int(nil)
	return fee
}

// WithdrawalQuote represents withdrawal cost estimate
type WithdrawalQuote struct {
	QuoteID            string
	Amount             *big.Int
	NetworkFee         *big.Int
	NetworkFeeCurrency string
	PlatformFee        *big.Int
	TotalFee           *big.Int
	TotalCost          *big.Int
	UserHasBalance     bool
	ValidUntil         time.Time
	Explanation        string
}

// When collecting platform fees, credit system wallet
func (uc *TransactionUsecase) creditPlatformFee(
	ctx context.Context,
	chainName, assetCode string,
	feeAmount *big.Int,
) error {
	
	// Get system wallet
	systemWallet, err := uc.systemUsecase.GetSystemWallet(ctx, chainName, assetCode)
	if err != nil {
		return fmt.Errorf("failed to get system wallet: %w", err)
	}
	
	// Update system wallet balance
	newBalance := new(big.Int).Add(systemWallet.Balance, feeAmount)
	if err := uc.walletRepo.UpdateBalance(ctx, systemWallet.ID, newBalance); err != nil {
		return fmt. Errorf("failed to update system balance: %w", err)
	}
	
	uc.logger.Info("Platform fee credited to system wallet",
		zap. String("chain", chainName),
		zap.String("asset", assetCode),
		zap.String("amount", feeAmount.String()),
		zap.String("new_balance", newBalance.String()))
	
	return nil
}
// internal/usecase/transaction_usecase.go
package usecase

import (
	"context"
	registry "crypto-service/internal/chains/registry"
	"crypto-service/internal/risk"
	"crypto-service/pkg/utils"

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
	approvalRepo    *repository.WithdrawalApprovalRepository
	chainRegistry   *registry.Registry
	encryption      *security.Encryption
	systemUsecase   *SystemUsecase
	riskAssessor    *risk.RiskAssessor
	logger          *zap.Logger
}

func NewTransactionUsecase(
	transactionRepo *repository.CryptoTransactionRepository,
	walletRepo *repository.CryptoWalletRepository,
	approvalRepo *repository.WithdrawalApprovalRepository,
	chainRegistry *registry.Registry,
	encryption *security.Encryption,
	systemUsecase *SystemUsecase,
	riskAssessor *risk.RiskAssessor,
	logger *zap.Logger,
) *TransactionUsecase {
	return &TransactionUsecase{
		transactionRepo: transactionRepo,
		walletRepo:      walletRepo,
		approvalRepo:    approvalRepo,
		chainRegistry:   chainRegistry,
		encryption:      encryption,
		systemUsecase:   systemUsecase,
		riskAssessor:    riskAssessor,
		logger:          logger,
	}
}

// ============================================================================
// NETWORK FEE ESTIMATION (for accounting module)
// ============================================================================

// EstimateNetworkFee estimates blockchain network fee for accounting

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

	//  Get decimals for this asset
	decimals := utils.GetAssetDecimals(assetCode)

	//  Parse amount using parseAmount (handles decimals)
	amountBig, err := utils.ParseAmount(amount, decimals)
	if err != nil {
		uc.logger.Error("Failed to parse amount",
			zap.String("amount", amount),
			zap.Int("decimals", decimals),
			zap.Error(err))
		return nil, fmt.Errorf("invalid amount format: %w", err)
	}

	uc.logger.Info("Amount parsed successfully",
		zap.String("amount_input", amount),
		zap.String("amount_parsed", amountBig.String()),
		zap.Int("decimals", decimals))

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
		From:     systemWallet.Address, //  From system wallet
		To:       toAddress,
		Asset:    utils.AssetFromChainAndCode(chainName, assetCode),
		Amount:   amountBig,
		Priority: domain.TxPriorityNormal,
	})

	if err != nil {
		uc.logger.Warn("Failed to estimate fee from chain, using defaults",
			zap.Error(err),
			zap.String("chain", chainName))

		// Return conservative defaults
		feeEstimate = uc.getDefaultFeeEstimate(chainName, assetCode)
	}

	estimate := &NetworkFeeEstimate{
		Chain:        chainName,
		Asset:        assetCode,
		FeeAmount:    feeEstimate.Amount,
		FeeCurrency:  feeEstimate.Currency,
		FeeFormatted: utils.FormatAmount(feeEstimate.Amount, feeEstimate.Currency),
		EstimatedAt:  time.Now(),
		ValidFor:     5 * time.Minute,
	}

	uc.logger.Info("Network fee estimated",
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
	networkFee, err := uc.EstimateNetworkFee(ctx, chainName, assetCode, amount, toAddress)
	if err != nil {
		return nil, err
	}

	//  Get decimals for this asset
	decimals := utils.GetAssetDecimals(assetCode)

	//  Parse amount using parseAmount (handles decimals)
	amountBig, err := utils.ParseAmount(amount, decimals)
	if err != nil {
		uc.logger.Error("Failed to parse amount",
			zap.String("amount", amount),
			zap.Int("decimals", decimals),
			zap.Error(err))
		return nil, fmt.Errorf("invalid amount format: %w", err)
	}

	quote := &WithdrawalQuote{
		QuoteID:            uuid.New().String(),
		Chain:              chainName,
		Asset:              assetCode,
		Amount:             amountBig,
		NetworkFee:         networkFee.FeeAmount,
		NetworkFeeCurrency: networkFee.FeeCurrency,
		Explanation:        fmt.Sprintf("Network fee: %s %s", networkFee.FeeFormatted, networkFee.FeeCurrency),
		ValidUntil:         time.Now().Add(5 * time.Minute),
	}

	return quote, nil
}

// internal/usecase/transaction_usecase.go

// ============================================================================
// WITHDRAWAL (from system hot wallet to external address)
// ============================================================================

// Withdraw sends crypto from SYSTEM hot wallet to external address
// NOTE: Accounting module should have already:
//  1. Verified user has sufficient virtual balance
//  2. Deducted amount + fees from user's virtual wallet
//  3. Called this method to execute blockchain transaction
//     UPDATED: Now includes approval flow based on risk assessment
func (uc *TransactionUsecase) Withdraw(
	ctx context.Context,
	accountingTxID string, // From accounting module for idempotency
	chainName, assetCode, amount, toAddress, memo string,
	userID string, // For tracking who requested it
) (*domain.CryptoTransaction, error) {

	uc.logger.Info("Processing withdrawal request",
		zap.String("accounting_tx_id", accountingTxID),
		zap.String("user_id", userID),
		zap.String("chain", chainName),
		zap.String("asset", assetCode),
		zap.String("to", toAddress))

	// 1. Check for duplicate request (idempotency)
	existingTx, err := uc.transactionRepo.GetByAccountingTxID(ctx, accountingTxID)
	if err == nil && existingTx != nil {
		uc.logger.Info("Duplicate withdrawal request detected",
			zap.String("accounting_tx_id", accountingTxID),
			zap.String("existing_tx", existingTx.TransactionID))
		return existingTx, nil
	}

	// 2. Parse amount
	decimals := utils.GetAssetDecimals(assetCode)
	amountBig, err := utils.ParseAmount(amount, decimals)
	if err != nil {
		uc.logger.Error("Failed to parse amount",
			zap.String("amount", amount),
			zap.Int("decimals", decimals),
			zap.Error(err))
		return nil, fmt.Errorf("invalid amount format: %w", err)
	}

	//  3. RISK ASSESSMENT
	riskAssessment, err := uc.riskAssessor.AssessWithdrawal(
		ctx, userID, chainName, assetCode, amountBig, toAddress)
	if err != nil {
		uc.logger.Error("Risk assessment failed", zap.Error(err))
		// Don't fail - continue with default approval required
		riskAssessment = &risk.RiskAssessment{
			RiskScore:        50,
			RequiresApproval: true,
			Explanation:      "Risk assessment unavailable - manual review required",
		}
	}

	uc.logger.Info("Risk assessment completed",
		zap.Int("risk_score", riskAssessment.RiskScore),
		zap.Bool("requires_approval", riskAssessment.RequiresApproval),
		zap.String("explanation", riskAssessment.Explanation))

	// 4. Get system hot wallet
	systemWallet, err := uc.systemUsecase.GetSystemWallet(ctx, chainName, assetCode)
	if err != nil {
		return nil, fmt.Errorf("system wallet not found: %w", err)
	}

	// 5. Estimate network fee
	feeEstimate, err := uc.EstimateNetworkFee(ctx, chainName, assetCode, amount, toAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to estimate fee: %w", err)
	}

	// 6. Check system wallet has enough (amount + network fee)
	totalNeeded := new(big.Int).Add(amountBig, feeEstimate.FeeAmount)
	if systemWallet.Balance.Cmp(totalNeeded) < 0 {
		return nil, fmt.Errorf("insufficient system wallet balance: have %s, need %s",
			utils.FormatAmount(systemWallet.Balance, assetCode),
			utils.FormatAmount(totalNeeded, assetCode))
	}

	// 7. Create transaction record (stays PENDING if approval needed)
	tx := &domain.CryptoTransaction{
		TransactionID:         uuid.New().String(),
		AccountingTxID:        &accountingTxID,
		UserID:                userID,
		Type:                  domain.TransactionTypeWithdrawal,
		Chain:                 chainName,
		Asset:                 assetCode,
		FromWalletID:          &systemWallet.ID,
		FromAddress:           systemWallet.Address,
		ToAddress:             toAddress,
		IsInternal:            false,
		Amount:                amountBig,
		NetworkFee:            feeEstimate.FeeAmount,
		NetworkFeeCurrency:    &feeEstimate.FeeCurrency,
		PlatformFee:           big.NewInt(0), // Handled by accounting
		TotalFee:              feeEstimate.FeeAmount,
		Status:                domain.TransactionStatusPending, //  Stays pending if approval needed
		RequiredConfirmations: utils.GetRequiredConfirmations(chainName),
		Memo:                  &memo,
		InitiatedAt:           time.Now(),
	}

	// 8. Save transaction
	if err := uc.transactionRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	//  9. CREATE APPROVAL RECORD
	approval := &domain.WithdrawalApproval{
		TransactionID:    tx.ID,
		UserID:           userID,
		Amount:           amountBig,
		Asset:            assetCode,
		ToAddress:        toAddress,
		RiskScore:        riskAssessment.RiskScore,
		RiskFactors:      convertRiskFactors(riskAssessment.RiskFactors),
		RequiresApproval: riskAssessment.RequiresApproval,
		Status:           determineApprovalStatus(riskAssessment),
	}

	if !riskAssessment.RequiresApproval {
		explanation := riskAssessment.Explanation
		approval.AutoApprovedReason = &explanation
	}

	if err := uc.approvalRepo.Create(ctx, approval); err != nil {
		uc.logger.Error("Failed to create approval record", zap.Error(err))
		// Don't fail transaction - continue
	}

	//  10. DECISION POINT: Auto-approve or wait for manual approval?
	if !riskAssessment.RequiresApproval {
		//  AUTO-APPROVE: Execute immediately
		uc.logger.Info("Withdrawal auto-approved - executing",
			zap.String("tx_id", tx.TransactionID),
			zap.String("reason", riskAssessment.Explanation))

		go uc.executeWithdrawal(ctx, tx, systemWallet)

		return tx, nil
	}

	//  REQUIRES APPROVAL: Keep in pending state
	uc.logger.Info("Withdrawal requires manual approval",
		zap.String("tx_id", tx.TransactionID),
		zap.Int("risk_score", riskAssessment.RiskScore),
		zap.Int64("approval_id", approval.ID))

	// TODO: Notify admins of pending approval
	// go uc.notifyAdminsOfPendingApproval(ctx, tx, approval)

	return tx, nil
}

//	executeWithdrawal executes the actual blockchain withdrawal
//
// Called either immediately (auto-approved) or after manual approval
func (uc *TransactionUsecase) executeWithdrawal(
	ctx context.Context,
	tx *domain.CryptoTransaction,
	systemWallet *domain.CryptoWallet,
) {
	uc.logger.Info("Executing withdrawal on blockchain",
		zap.String("tx_id", tx.TransactionID),
		zap.String("chain", tx.Chain),
		zap.String("asset", tx.Asset))

	// 1. Get blockchain implementation
	chain, err := uc.chainRegistry.Get(tx.Chain)
	if err != nil {
		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, fmt.Sprintf("Unsupported chain: %v", err))
		uc.logger.Error("Failed to get chain", zap.Error(err))
		return
	}

	// 2. Decrypt system wallet private key
	privateKey, err := uc.encryption.Decrypt(systemWallet.EncryptedPrivateKey)
	if err != nil {
		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, "Failed to decrypt system wallet key")
		uc.logger.Error("Failed to decrypt private key", zap.Error(err))
		return
	}

	// 3. Update status to broadcasting
	uc.transactionRepo.UpdateStatus(ctx, tx.ID, domain.TransactionStatusBroadcasting, nil)

	// 4. Execute blockchain transaction
	txResult, err := chain.Send(ctx, &domain.TransactionRequest{
		From:       systemWallet.Address,
		To:         tx.ToAddress,
		Asset:      utils.AssetFromChainAndCode(tx.Chain, tx.Asset),
		Amount:     tx.Amount,
		PrivateKey: privateKey, // For Circle, this is wallet ID
		Memo:       tx.Memo,
		Priority:   domain.TxPriorityNormal,
	})

	if err != nil {
		uc.logger.Error("Blockchain transaction failed",
			zap.Error(err),
			zap.String("tx_id", tx.TransactionID))

		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, err.Error())

		// TODO: Notify accounting module of failure for refund
		return
	}

	// 5. Update transaction with blockchain details
	tx.TxHash = &txResult.TxHash
	tx.Status = domain.TransactionStatusBroadcasted
	now := time.Now()
	tx.BroadcastedAt = &now

	if err := uc.transactionRepo.Update(ctx, tx); err != nil {
		uc.logger.Error("Failed to update transaction", zap.Error(err))
	}

	// 6. Update system wallet balance
	actualCost := new(big.Int).Add(tx.Amount, txResult.Fee)
	newSystemBalance := new(big.Int).Sub(systemWallet.Balance, actualCost)

	uc.walletRepo.UpdateBalance(ctx, systemWallet.ID, newSystemBalance)

	uc.logger.Info("Withdrawal broadcasted successfully",
		zap.String("tx_hash", txResult.TxHash),
		zap.String("tx_id", tx.TransactionID),
		zap.String("new_system_balance", newSystemBalance.String()))

	// 7. Start monitoring confirmations
	go uc.monitorTransactionConfirmations(tx.TransactionID, tx.Chain, *tx.TxHash)
}

// ============================================================================
// APPROVAL OPERATIONS
// ============================================================================

// ApproveWithdrawal approves a pending withdrawal (admin action)
func (uc *TransactionUsecase) ApproveWithdrawal(
	ctx context.Context,
	approvalID int64,
	approvedBy, notes string,
) (*domain.CryptoTransaction, error) {

	uc.logger.Info("Admin approving withdrawal",
		zap.Int64("approval_id", approvalID),
		zap.String("approved_by", approvedBy))

	// 1. Get approval record
	approval, err := uc.approvalRepo.GetByID(ctx, approvalID)
	if err != nil {
		return nil, fmt.Errorf("approval not found: %w", err)
	}

	// 2. Check if already processed
	if approval.Status != domain.ApprovalStatusPendingReview {
		return nil, fmt.Errorf("approval already processed: %s", approval.Status)
	}

	// 3. Get transaction
	tx, err := uc.transactionRepo.GetByID(ctx, approval.TransactionID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	// 4. Verify transaction is still pending
	if tx.Status != domain.TransactionStatusPending {
		return nil, fmt.Errorf("transaction is not pending: %s", tx.Status)
	}

	// 5. Mark approval as approved
	if err := uc.approvalRepo.Approve(ctx, approvalID, approvedBy, notes); err != nil {
		return nil, fmt.Errorf("failed to approve: %w", err)
	}

	// 6. Get system wallet
	systemWallet, err := uc.systemUsecase.GetSystemWallet(ctx, tx.Chain, tx.Asset)
	if err != nil {
		return nil, fmt.Errorf("system wallet not found: %w", err)
	}

	// 7. Execute withdrawal asynchronously
	go uc.executeWithdrawal(ctx, tx, systemWallet)

	uc.logger.Info("Withdrawal approved and executing",
		zap.String("tx_id", tx.TransactionID),
		zap.String("approved_by", approvedBy))

	return tx, nil
}

// RejectWithdrawal rejects a pending withdrawal (admin action)
func (uc *TransactionUsecase) RejectWithdrawal(
	ctx context.Context,
	approvalID int64,
	rejectedBy, reason string,
) error {

	uc.logger.Info("Admin rejecting withdrawal",
		zap.Int64("approval_id", approvalID),
		zap.String("rejected_by", rejectedBy),
		zap.String("reason", reason))

	// 1. Get approval
	approval, err := uc.approvalRepo.GetByID(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("approval not found: %w", err)
	}

	// 2. Check if already processed
	if approval.Status != domain.ApprovalStatusPendingReview {
		return fmt.Errorf("approval already processed: %s", approval.Status)
	}

	// 3. Get transaction
	tx, err := uc.transactionRepo.GetByID(ctx, approval.TransactionID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	// 4. Verify transaction is still pending
	if tx.Status != domain.TransactionStatusPending {
		return fmt.Errorf("transaction is not pending: %s", tx.Status)
	}

	// 5. Mark approval as rejected
	if err := uc.approvalRepo.Reject(ctx, approvalID, rejectedBy, reason); err != nil {
		return fmt.Errorf("failed to reject: %w", err)
	}

	// 6. Mark transaction as cancelled
	if err := uc.transactionRepo.MarkAsCancelled(ctx, approval.TransactionID, reason); err != nil {
		return fmt.Errorf("failed to cancel transaction: %w", err)
	}

	// 7. TODO: Notify accounting to refund user
	// go uc.notifyAccountingOfRejection(ctx, approval, tx)

	uc.logger.Info("Withdrawal rejected",
		zap.Int64("approval_id", approvalID),
		zap.String("tx_id", tx.TransactionID),
		zap.String("rejected_by", rejectedBy))

	return nil
}

// GetPendingApprovals retrieves pending withdrawal approvals for admin dashboard
func (uc *TransactionUsecase) GetPendingApprovals(
	ctx context.Context,
	limit, offset int,
) ([]*domain.WithdrawalApproval, error) {
	return uc.approvalRepo.GetPendingApprovals(ctx, limit, offset)
}

// GetApprovalByID retrieves a specific approval
func (uc *TransactionUsecase) GetApprovalByID(
	ctx context.Context,
	approvalID int64,
) (*domain.WithdrawalApproval, error) {
	return uc.approvalRepo.GetByID(ctx, approvalID)
}

// GetApprovalStats gets approval statistics for admin dashboard
func (uc *TransactionUsecase) GetApprovalStats(ctx context.Context) (*repository.ApprovalStats, error) {
	return uc.approvalRepo.GetApprovalStats(ctx)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// determineApprovalStatus determines approval status based on risk assessment
func determineApprovalStatus(assessment *risk.RiskAssessment) domain.ApprovalStatus {
	if !assessment.RequiresApproval {
		return domain.ApprovalStatusAutoApproved
	}
	return domain.ApprovalStatusPendingReview
}

// convertRiskFactors converts risk.RiskFactor to domain.RiskFactor
func convertRiskFactors(factors []domain.RiskFactor) []domain.RiskFactor {
	domainFactors := make([]domain.RiskFactor, len(factors))
	for i, factor := range factors {
		domainFactors[i] = domain.RiskFactor{
			Factor:      factor.Factor,
			Description: factor.Description,
			Score:       factor.Score,
		}
	}
	return domainFactors
}

// notifyAdminsOfPendingApproval sends notification to admins (TODO)
func (uc *TransactionUsecase) notifyAdminsOfPendingApproval(
	ctx context.Context,
	tx *domain.CryptoTransaction,
	approval *domain.WithdrawalApproval,
) {
	// TODO: Implement notification system
	// - Send email to admins
	// - Push notification
	// - WebSocket notification
	// - Slack/Discord webhook

	uc.logger.Info("Admin notification sent",
		zap.String("tx_id", tx.TransactionID),
		zap.Int64("approval_id", approval.ID),
		zap.Int("risk_score", approval.RiskScore))
}

// notifyAccountingOfRejection notifies accounting to refund user (TODO)
func (uc *TransactionUsecase) notifyAccountingOfRejection(
	ctx context.Context,
	approval *domain.WithdrawalApproval,
	tx *domain.CryptoTransaction,
) {
	// TODO: Call accounting service to refund
	// - Reverse the deduction
	// - Credit user's account
	// - Update ledger

	uc.logger.Info("Accounting refund notification sent",
		zap.String("tx_id", tx.TransactionID),
		zap.String("user_id", approval.UserID),
		zap.String("accounting_tx_id", *tx.AccountingTxID))
}

func (uc *TransactionUsecase) monitorTransactionConfirmations(
	transactionID, chainName, txHash string,
) {
	ctx := context.Background()

	uc.logger.Info("Starting confirmation monitoring",
		zap.String("tx_id", transactionID),
		zap.String("chain", chainName),
		zap.String("tx_hash", txHash))

	// Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		uc.logger.Error("Failed to get chain for monitoring",
			zap.String("chain", chainName),
			zap.Error(err))
		return
	}

	// Get required confirmations
	tx, err := uc.transactionRepo.GetByTransactionID(ctx, transactionID)
	if err != nil {
		uc.logger.Error("Failed to get transaction",
			zap.String("tx_id", transactionID),
			zap.Error(err))
		return
	}

	requiredConfs := tx.RequiredConfirmations
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	timeout := time.After(2 * time.Hour) // Timeout after 2 hours

	for {
		select {
		case <-timeout:
			uc.logger.Warn("Transaction monitoring timeout",
				zap.String("tx_id", transactionID),
				zap.String("tx_hash", txHash))

			uc.transactionRepo.UpdateStatus(ctx, tx.ID,
				domain.TransactionStatusPending,
				utils.StringPtr("Confirmation monitoring timeout - requires manual review"))
			return

		case <-ticker.C:
			// Check transaction status on blockchain
			txStatus, err := chain.GetTransaction(ctx, txHash)
			if err != nil {
				uc.logger.Warn("Failed to get transaction status",
					zap.String("tx_hash", txHash),
					zap.Error(err))
				continue
			}

			uc.logger.Info("Transaction status checked",
				zap.String("tx_hash", txHash),
				zap.Int("confirmations", txStatus.Confirmations),
				zap.Int("required", requiredConfs),
				zap.String("status", string(txStatus.Status)))

			// Update confirmations count
			uc.transactionRepo.UpdateConfirmations(ctx, tx.ID, txStatus.Confirmations)

			// Check if transaction is confirmed
			if txStatus.Confirmations >= requiredConfs {
				//  Transaction confirmed!
				uc.logger.Info("Transaction confirmed on blockchain",
					zap.String("tx_id", transactionID),
					zap.String("tx_hash", txHash),
					zap.Int("confirmations", txStatus.Confirmations))

				// Update status to confirmed
				uc.transactionRepo.MarkAsConfirmed(ctx, tx.ID,
					*txStatus.BlockNumber, *tx.BlockTimestamp)

				// TODO: Notify accounting module that withdrawal is confirmed
				// uc.notifyAccountingConfirmed(ctx, tx)

				return // Stop monitoring
			}

			// Check if transaction failed
			if txStatus.Status == "failed" || txStatus.Status == "reverted" {
				uc.logger.Error("Transaction failed on blockchain",
					zap.String("tx_id", transactionID),
					zap.String("tx_hash", txHash),
					zap.String("reason", "unknown"))

				uc.transactionRepo.MarkAsFailed(ctx, tx.ID,
					fmt.Sprintf("Blockchain transaction failed: %s", "unkwnown reason"))

				// TODO: Notify accounting module of failure
				// uc.notifyAccountingFailed(ctx, tx)

				return // Stop monitoring
			}

			// Update status to confirming if we have at least 1 confirmation
			if txStatus.Confirmations > 0 && tx.Status == domain.TransactionStatusBroadcasted {
				uc.transactionRepo.UpdateStatus(ctx, tx.ID,
					domain.TransactionStatusConfirming, nil)
			}
		}
	}
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
	if sweepAmount.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("balance not enough to cover network fee")
	}

	// 6. Create transaction record
	tx := &domain.CryptoTransaction{
		TransactionID:         uuid.New().String(),
		UserID:                userID,
		Type:                  domain.TransactionTypeSweep,
		Chain:                 chainName,
		Asset:                 assetCode,
		FromWalletID:          &userWallet.ID,
		FromAddress:           userWallet.Address, // User's deposit address
		ToWalletID:            &systemWallet.ID,
		ToAddress:             systemWallet.Address, // System wallet
		IsInternal:            true,                 // Internal sweep
		Amount:                sweepAmount,
		NetworkFee:            feeEstimate.FeeAmount,
		NetworkFeeCurrency:    &feeEstimate.FeeCurrency,
		PlatformFee:           big.NewInt(0),
		TotalFee:              feeEstimate.FeeAmount,
		Status:                domain.TransactionStatusPending,
		RequiredConfirmations: utils.GetRequiredConfirmations(chainName),
		InitiatedAt:           time.Now(),
	}

	// 7. Save to database
	if err := uc.transactionRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	// 8. Get blockchain implementation
	chain, err := uc.chainRegistry.Get(chainName)
	if err != nil {
		uc.transactionRepo.MarkAsFailed(ctx, tx.ID, fmt.Sprintf("Unsupported chain: %v", err))
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

	txResult, err := chain.Send(ctx, &domain.TransactionRequest{
		From:       userWallet.Address,   //  From user's deposit address
		To:         systemWallet.Address, //  To system wallet
		Asset:      utils.AssetFromChainAndCode(chainName, assetCode),
		Amount:     sweepAmount,
		PrivateKey: privateKey,
		Priority:   domain.TxPriorityLow, // Low priority for sweeps
	})

	if err != nil {
		uc.logger.Error("Sweep transaction failed",
			zap.Error(err),
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
		zap.String("tx_hash", txResult.TxHash),
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

	var swept []*domain.CryptoTransaction
	var failed int

	for _, wallet := range wallets {
		tx, err := uc.SweepUserWallet(ctx, wallet.UserID, chainName, assetCode)
		if err != nil {
			uc.logger.Warn("Failed to sweep wallet",
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
	ctx context.Context,
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
			"TRX":  100000,  // 0.1 TRX
			"USDT": 5000000, // 5 TRX equivalent
		},
		"BITCOIN": {
			"BTC": 5000, // 5000 satoshis
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

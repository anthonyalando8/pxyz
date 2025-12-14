package usecase

import (
	"context"
	"fmt"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"

	"accounting-service/internal/domain"
	//xerrors "x/shared/utils/errors"
)

// ===============================
// ACCOUNT FETCHING HELPERS
// ===============================

// fetchSystemAndUserAccounts gets system and user accounts for credit/debit operations
func (uc *TransactionUsecase) fetchSystemAndUserAccounts(
	ctx context.Context,
	currency string,
	purpose domain.AccountPurpose,
	userAccountNumber string,
) (*domain.Account, *domain. Account, error) {
	// ✅ Step 1: Fetch user account first
	userAccount, err := uc.accountUC.GetByAccountNumber(ctx, userAccountNumber)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user account: %w", err)
	}

	// ✅ Step 2: Infer currency from user account if not provided
	if currency == "" {
		if userAccount.Currency == "" {
			return nil, nil, fmt. Errorf("currency not provided and user account has no currency")
		}
		currency = userAccount.Currency
	}

	// ✅ Step 3: Fetch system account with currency
	systemAccount, err := uc.accountUC.GetSystemAccount(ctx, currency, purpose)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get system account for currency %s: %w", currency, err)
	}

	// ✅ Step 4: Validate currency match
	if userAccount.Currency != systemAccount.Currency {
		return nil, nil, fmt.Errorf("currency mismatch: user account (%s) vs system account (%s)", 
			userAccount.Currency, systemAccount.Currency)
	}

	return systemAccount, userAccount, nil
}

// fetchTransferAccounts gets source and destination accounts for transfers
func (uc *TransactionUsecase) fetchTransferAccounts(
	ctx context.Context,
	fromAccount, toAccount string,
) (*domain.Account, *domain.Account, error) {
	sourceAccount, err := uc.accountUC.GetByAccountNumber(ctx, fromAccount)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get source account: %w", err)
	}

	destAccount, err := uc.accountUC.GetByAccountNumber(ctx, toAccount)
	if err != nil {
		return nil, nil, fmt. Errorf("failed to get destination account: %w", err)
	}

	return sourceAccount, destAccount, nil
}

// ===============================
// TRANSACTION REQUEST BUILDERS
// ===============================

// buildCreditDoubleEntry creates a system → user double-entry transaction
func buildCreditDoubleEntry(
	req *domain.CreditRequest,
	systemAccount *domain.Account,
	userAccount *domain. Account,
) *domain.TransactionRequest {
	return &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeDeposit,
		AccountType:         req.AccountType,
		Description:         &req.Description,
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		IsSystemTransaction: true,
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: systemAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrDebit,
				Currency:      req.Currency,
				Description:   &req.Description,
				Metadata:      req.Metadata,
			},
			{
				AccountNumber: userAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrCredit,
				Currency:      req.Currency,
				Description:   &req. Description,
				Metadata:      req.Metadata,
			},
		},
		GenerateReceipt: true,
	}
}

// buildDebitDoubleEntry creates a user → system double-entry transaction
func buildDebitDoubleEntry(
	req *domain.DebitRequest,
	systemAccount *domain.Account,
	userAccount *domain.Account,
) *domain.TransactionRequest {
	return &domain. TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeWithdrawal,
		AccountType:         req.AccountType,
		Description:         &req.Description,
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		IsSystemTransaction: true,
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: userAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrDebit,
				Currency:      req.Currency,
				Description:   &req.Description,
				Metadata:      req. Metadata,
			},
			{
				AccountNumber: systemAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrCredit,
				Currency:      req.Currency,
				Description:   &req.Description,
				Metadata:      req. Metadata,
			},
		},
		GenerateReceipt: true,
	}
}

// buildTransferDoubleEntry creates a user → user transfer transaction
func buildTransferDoubleEntry(
	req *domain.TransferRequest,
	sourceAccount *domain. Account,
	destAccount *domain.Account,
) *domain. TransactionRequest {
	// Validate same currency
	if sourceAccount.Currency != destAccount.Currency {
		return nil
	}

	return &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain. TransactionTypeTransfer,
		AccountType:         req.AccountType,
		Description:         &req.Description,
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		AgentExternalID:     req.AgentExternalID,
		IsSystemTransaction: false, // FEES APPLY
		Entries: []*domain. LedgerEntryRequest{
			{
				AccountNumber: sourceAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrDebit,
				Currency:      sourceAccount.Currency,
				Description:   &req.Description,
				Metadata:      req.Metadata,
			},
			{
				AccountNumber: destAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrCredit,
				Currency:      destAccount.Currency,
				Description:   &req.Description,
				Metadata:      req.Metadata,
			},
		},
		GenerateReceipt: true,
	}
}

// buildConversionDoubleEntry creates a currency conversion transaction
func buildConversionDoubleEntry(
	req *domain.ConversionRequest,
	sourceAccount *domain.Account,
	destAccount *domain.Account,
) *domain.TransactionRequest {
	return &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeConversion,
		AccountType:         req.AccountType,
		Description:         ptrString(fmt.Sprintf("Currency conversion: %s to %s",
			sourceAccount.Currency, destAccount.Currency)),
		CreatedByExternalID: &req. CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		AgentExternalID:     req. AgentExternalID,
		IsSystemTransaction: false,
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: sourceAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain. DrCrDebit,
				Currency:      sourceAccount.Currency,
				Description:   ptrString(fmt.Sprintf("Convert from %s", sourceAccount.Currency)),
				Metadata:      req.Metadata,
			},
			{
				AccountNumber: destAccount.AccountNumber,
				Amount:        req. Amount,
				DrCr:          domain.DrCrCredit,
				Currency:      destAccount.Currency,
				Description:   ptrString(fmt.Sprintf("Convert to %s", destAccount.Currency)),
				Metadata:      req.Metadata,
			},
		},
		GenerateReceipt: true,
	}
}

// buildTradeDoubleEntry creates trade win/loss transaction
func buildTradeDoubleEntry(
	req *domain.TradeRequest,
	systemAccount *domain.Account,
	userAccount *domain.Account,
	result string, // "win" or "loss"
) *domain.TransactionRequest {
	metadata := buildTradeMetadata(req, result)

	var txType domain.TransactionType
	var userDrCr domain.DrCr
	var systemDrCr domain.DrCr

	if result == "win" {
		txType = domain.TransactionTypeDeposit
		userDrCr = domain.DrCrCredit
		systemDrCr = domain.DrCrDebit
	} else {
		txType = domain.TransactionTypeWithdrawal
		userDrCr = domain.DrCrDebit
		systemDrCr = domain.DrCrCredit
	}

	return &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     txType,
		AccountType:         req.AccountType,
		Description:         ptrString(fmt. Sprintf("Trade %s: %s (%s)", result, req.TradeID, req.TradeType)),
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		IsSystemTransaction: true,
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: systemAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          systemDrCr,
				Currency:      req.Currency,
				Description:   ptrString(fmt.Sprintf("System %s for trade %s", result, result)),
				Metadata:      metadata,
			},
			{
				AccountNumber: userAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          userDrCr,
				Currency:      req.Currency,
				Description:   ptrString(fmt.Sprintf("Trade %s: %s", result, req.TradeID)),
				Metadata:      metadata,
			},
		},
		GenerateReceipt: true,
	}
}

// buildCommissionDoubleEntry creates commission payment transaction
func buildCommissionDoubleEntry(
	req *domain. AgentCommissionRequest,
	systemFeeAccount *domain.Account,
	agentAccount *domain.Account,
) *domain.TransactionRequest {
	metadata := buildCommissionMetadata(req)

	return &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeCommission,
		AccountType:         domain.AccountTypeReal,
		Description:         ptrString(fmt.Sprintf("Agent commission: %s", req.TransactionRef)),
		CreatedByExternalID: ptrString("system"),
		CreatedByType:       ptrOwnerType(domain.OwnerTypeSystem),
		IsSystemTransaction: true,
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: systemFeeAccount.AccountNumber,
				Amount:        req.CommissionAmount,
				DrCr:          domain.DrCrDebit,
				Currency:      req.Currency,
				Description:   ptrString(fmt.Sprintf("Commission payment for %s", req.TransactionRef)),
				Metadata:      metadata,
			},
			{
				AccountNumber: agentAccount.AccountNumber,
				Amount:        req.CommissionAmount,
				DrCr:          domain. DrCrCredit,
				Currency:      req.Currency,
				Description:   ptrString(fmt.Sprintf("Commission earned: %s", req.TransactionRef)),
				Metadata:      metadata,
			},
		},
		GenerateReceipt: true,
	}
}

// ===============================
// METADATA BUILDERS
// ===============================

// buildTradeMetadata creates metadata for trade transactions
func buildTradeMetadata(req *domain.TradeRequest, result string) map[string]interface{} {
	metadata := req.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["trade_id"] = req.TradeID
	metadata["trade_type"] = req.TradeType
	metadata["trade_result"] = result
	return metadata
}

// buildCommissionMetadata creates metadata for commission transactions
func buildCommissionMetadata(req *domain.AgentCommissionRequest) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["agent_id"] = req.AgentExternalID
	metadata["original_txn"] = req.TransactionRef
	metadata["transaction_amount"] = req.TransactionAmount
	if req.CommissionRate != nil {
		metadata["commission_rate"] = *req.CommissionRate
	}
	return metadata
}

// ===============================
// GENERIC TRANSACTION EXECUTION
// ===============================

// executeWithReceipt handles the common transaction execution pattern
func (uc *TransactionUsecase) executeWithReceipt(
	ctx context.Context,
	txReq *domain.TransactionRequest,
	repoExecutor func(context.Context) (*domain.LedgerAggregate, error),
	eventPublisher func(*domain.LedgerAggregate),
) (*domain.LedgerAggregate, error) {
	// Generate receipt code
	receiptCode, err := uc.generateReceiptCode(ctx, txReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate receipt: %w", err)
	}

	txReq. ReceiptCode = &receiptCode
	
	// Update TransactionFee receipt code if fee exists
	if txReq.TransactionFee != nil {
		txReq.TransactionFee. ReceiptCode = receiptCode
	}
	
	// Track status
	uc.statusTracker.Track(receiptCode, "processing")
	uc.logTransactionStart(receiptCode, txReq)

	// Execute transaction
	aggregate, err := repoExecutor(ctx)
	if err != nil {
		uc. handleTransactionFailure(receiptCode, err)
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	// Handle success
	uc.handleTransactionSuccess(ctx, receiptCode, aggregate, eventPublisher)

	return aggregate, nil
}

// handleTransactionSuccess updates status, logs, publishes events, invalidates caches
func (uc *TransactionUsecase) handleTransactionSuccess(
	ctx context.Context,
	receiptCode string,
	aggregate *domain. LedgerAggregate,
	eventPublisher func(*domain. LedgerAggregate),
) {
	// Update status
	uc.statusTracker.Update(receiptCode, "completed", "")
	uc.receiptBatcher.UpdateStatus(receiptCode, receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED, "")

	// Log success
	uc.logTransactionSuccess(receiptCode, aggregate. Journal. ID)

	// Publish event asynchronously
	if eventPublisher != nil {
		go eventPublisher(aggregate)
	}

	// Invalidate caches
	uc.invalidateTransactionCaches(ctx, aggregate)
}

// handleTransactionFailure updates status and logs error
func (uc *TransactionUsecase) handleTransactionFailure(receiptCode string, err error) {
	uc.statusTracker.Update(receiptCode, "failed", err.Error())
	uc. receiptBatcher.UpdateStatus(receiptCode, receiptpb.TransactionStatus_TRANSACTION_STATUS_FAILED, err.Error())
	uc.logTransactionError(receiptCode, err)
}


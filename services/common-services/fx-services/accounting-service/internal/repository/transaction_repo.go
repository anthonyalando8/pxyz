package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"accounting-service/internal/domain"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type TransactionRepository interface {
	// Core transaction execution
	ExecuteTransaction(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error)
	ExecuteTransactionOptimistic(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error)
	ExecuteTransactionPessimistic(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error)
	
	// Simple operations
	Credit(ctx context.Context, req *domain.CreditRequest) (*domain.LedgerAggregate, error)
	Debit(ctx context.Context, req *domain.DebitRequest) (*domain.LedgerAggregate, error)
	Transfer(ctx context.Context, req *domain.TransferRequest) (*domain.LedgerAggregate, error)
	
	// Currency conversion
	ConvertAndTransfer(ctx context.Context, req *domain.ConversionRequest) (*domain.LedgerAggregate, error)
	
	// Trading operations
	ProcessTradeWin(ctx context.Context, req *domain.TradeRequest) (*domain.LedgerAggregate, error)
	ProcessTradeLoss(ctx context.Context, req *domain.TradeRequest) (*domain.LedgerAggregate, error)
	
	// Agent operations
	ProcessAgentCommission(ctx context.Context, req *domain.AgentCommissionRequest) (*domain.LedgerAggregate, error)
	
	// Idempotency and transaction management
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.LedgerAggregate, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type transactionRepo struct {
	db           *pgxpool.Pool
	accountRepo  AccountRepository
	journalRepo  JournalRepository
	ledgerRepo   LedgerRepository
	balanceRepo  BalanceRepository
	currencyRepo CurrencyRepository  // Use existing currency repo for FX
	feeRepo      TransactionFeeRepository
	logger       *zap.Logger
}

func NewTransactionRepo(
	db *pgxpool.Pool,
	accountRepo AccountRepository,
	journalRepo JournalRepository,
	ledgerRepo LedgerRepository,
	balanceRepo BalanceRepository,
	currencyRepo CurrencyRepository,  // FX operations
	feeRepo TransactionFeeRepository,
	logger *zap.Logger,
) TransactionRepository {
	return &transactionRepo{
		db:           db,
		accountRepo:  accountRepo,
		journalRepo:  journalRepo,
		ledgerRepo:   ledgerRepo,
		balanceRepo:  balanceRepo,
		currencyRepo: currencyRepo,
		feeRepo:      feeRepo,
		logger:       logger,
	}
}

func (r *transactionRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// ========================================
// SIMPLE OPERATIONS
// ========================================

// Credit adds money to an account (trade win, deposit, bonus)
// System → User (NO FEES)
func (r *transactionRepo) Credit(
	ctx context.Context,
	req *domain.CreditRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get system liquidity account for currency
	systemAccount, err := r.accountRepo.GetSystemAccount(ctx, req.Currency, req.AccountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get system account: %w", err)
	}

	// Get user account
	userAccount, err := r.accountRepo.GetByAccountNumber(ctx, req.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %w", err)
	}

	// Build transaction request
	txReq := &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeDeposit,
		AccountType:         req.AccountType,
		ExternalRef:         req.ExternalRef,
		Description:         strPtr(req.Description),
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		IsSystemTransaction: true, // NO FEES
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: systemAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrDebit,
				Currency:      req.Currency,
				ReceiptCode: req.ReceiptCode,
				Description:   strPtr(req.Description),
				Metadata:      req.Metadata,
			},
			{
				AccountNumber: userAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrCredit,
				Currency:      req.Currency,
				ReceiptCode: req.ReceiptCode,
				Description:   strPtr(req.Description),
				Metadata:      req.Metadata,
			},
		},
		GenerateReceipt: true,
	}

	return r.ExecuteTransaction(ctx, txReq)
}

// Debit removes money from account (trade loss, withdrawal)
// User → System (NO FEES)
func (r *transactionRepo) Debit(
	ctx context.Context,
	req *domain.DebitRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get system liquidity account
	systemAccount, err := r.accountRepo.GetSystemAccount(ctx, req.Currency, req.AccountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get system account: %w", err)
	}

	// Get user account
	userAccount, err := r.accountRepo.GetByAccountNumber(ctx, req.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %w", err)
	}

	// Build transaction request
	txReq := &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeWithdrawal,
		AccountType:         req.AccountType,
		ExternalRef:         req.ExternalRef,
		Description:         strPtr(req.Description),
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		IsSystemTransaction: true, // NO FEES
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: userAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrDebit,
				Currency:      req.Currency,
				ReceiptCode: req.ReceiptCode,

				Description:   strPtr(req.Description),
				Metadata:      req.Metadata,
			},
			{
				AccountNumber: systemAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrCredit,
				Currency:      req.Currency,
				ReceiptCode: req.ReceiptCode,
				Description:   strPtr(req.Description),
				Metadata:      req.Metadata,
			},
		},
		GenerateReceipt: true,
	}

	return r.ExecuteTransaction(ctx, txReq)
}

// Transfer moves money between user accounts (P2P)
// User A → User B (FEES APPLY, optional agent commission)
func (r *transactionRepo) Transfer(
	ctx context.Context,
	req *domain.TransferRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get source account
	sourceAccount, err := r.accountRepo.GetByAccountNumber(ctx, req.FromAccountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get source account: %w", err)
	}

	// Get destination account
	destAccount, err := r.accountRepo.GetByAccountNumber(ctx, req.ToAccountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get destination account: %w", err)
	}

	// Validate same currency
	if sourceAccount.Currency != destAccount.Currency {
		return nil, xerrors.ErrCurrencyMismatch
	}

	// Build transaction request
	txReq := &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeTransfer,
		AccountType:         req.AccountType,
		ExternalRef:         req.ExternalRef,
		Description:         strPtr(req.Description),
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		AgentExternalID:     req.AgentExternalID,
		IsSystemTransaction: false, // FEES APPLY
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: sourceAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrDebit,
				Currency:      sourceAccount.Currency,
				ReceiptCode: req.ReceiptCode,
				Description:   strPtr(req.Description),
				Metadata:      req.Metadata,
			},
			{
				AccountNumber: destAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrCredit,
				ReceiptCode: req.ReceiptCode,
				Currency:      destAccount.Currency,
				Description:   strPtr(req.Description),
				Metadata:      req.Metadata,
			},
		},
		GenerateReceipt: true,
	}

	return r.ExecuteTransaction(ctx, txReq)
}

// ========================================
// CURRENCY CONVERSION
// ========================================

// ConvertAndTransfer performs currency conversion using FX rates
// USD Account → EUR Account (FEES APPLY)
func (r *transactionRepo) ConvertAndTransfer(
	ctx context.Context,
	req *domain.ConversionRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get source account
	sourceAccount, err := r.accountRepo.GetByAccountNumber(ctx, req.FromAccountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get source account: %w", err)
	}

	// Get destination account
	destAccount, err := r.accountRepo.GetByAccountNumber(ctx, req.ToAccountNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get destination account: %w", err)
	}

	// Currencies must be different
	if sourceAccount.Currency == destAccount.Currency {
		return nil, errors.New("use Transfer for same currency operations")
	}

	// Get current FX rate using currency repository
	fxRate, err := r.currencyRepo.GetCurrentFXRate(ctx, sourceAccount.Currency, destAccount.Currency)
	if err != nil {
		return nil, fmt.Errorf("failed to get FX rate for %s/%s: %w", 
			sourceAccount.Currency, destAccount.Currency, err)
	}
	// Calculate converted amount
	rate, err := strconv.ParseFloat(fxRate.Rate, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid fx rate %q: %w", fxRate.Rate, err)
	}
	convertedAmount := r.calculateConversion(req.Amount, rate)
	//convertedAmount := r.calculateConversion(req.Amount, fxRate.Rate)

	// Build metadata with FX information
	metadata := req.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["fx_rate"] = fxRate.Rate
	metadata["fx_rate_id"] = fxRate.ID
	metadata["source_currency"] = sourceAccount.Currency
	metadata["dest_currency"] = destAccount.Currency
	metadata["source_amount"] = req.Amount
	metadata["converted_amount"] = convertedAmount
	if fxRate.BidRate != nil {
		metadata["bid_rate"] = *fxRate.BidRate
	}
	if fxRate.AskRate != nil {
		metadata["ask_rate"] = *fxRate.AskRate
	}

	// Build transaction request
	txReq := &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeConversion,
		AccountType:         req.AccountType,
		ExternalRef:         req.ExternalRef,
		Description:         strPtr(fmt.Sprintf("Currency conversion: %s to %s (Rate: %.6s)", 
			sourceAccount.Currency, destAccount.Currency, fxRate.Rate)),
		CreatedByExternalID: &req.CreatedByExternalID,
		CreatedByType:       &req.CreatedByType,
		AgentExternalID:     req.AgentExternalID,
		IsSystemTransaction: false, // FEES APPLY
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: sourceAccount.AccountNumber,
				Amount:        req.Amount,
				DrCr:          domain.DrCrDebit,
				Currency:      sourceAccount.Currency,
				ReceiptCode: req.ReceiptCode,
				Description:   strPtr(fmt.Sprintf("Convert from %s", sourceAccount.Currency)),
				Metadata:      metadata,
			},
			{
				AccountNumber: destAccount.AccountNumber,
				Amount:        convertedAmount,
				DrCr:          domain.DrCrCredit,
				Currency:      destAccount.Currency,
				ReceiptCode: req.ReceiptCode,
				Description:   strPtr(fmt.Sprintf("Convert to %s", destAccount.Currency)),
				Metadata:      metadata,
			},
		},
		GenerateReceipt: true,
	}

	return r.ExecuteTransaction(ctx, txReq)
}

// calculateConversion applies FX rate to amount
func (r *transactionRepo) calculateConversion(amount int64, rate float64) int64 {
	// Convert to float, apply rate, round to nearest int64
	converted := float64(amount) * rate
	return int64(converted + 0.5) // Round to nearest
}

// ========================================
// TRADING OPERATIONS
// ========================================

// ProcessTradeWin credits user account for trade win
// System → User (NO FEES)
func (r *transactionRepo) ProcessTradeWin(
	ctx context.Context,
	req *domain.TradeRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Build metadata with trade information
	metadata := req.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["trade_id"] = req.TradeID
	metadata["trade_type"] = req.TradeType
	metadata["trade_result"] = "win"

	// Create credit request
	creditReq := &domain.CreditRequest{
		AccountNumber:       req.AccountNumber,
		Amount:              req.Amount,
		Currency:            req.Currency,
		AccountType:         req.AccountType,
		Description:         fmt.Sprintf("Trade win: %s (%s)", req.TradeID, req.TradeType),
		IdempotencyKey:      req.IdempotencyKey,
		ExternalRef:         &req.TradeID,
		ReceiptCode: req.ReceiptCode,
		CreatedByExternalID: req.CreatedByExternalID,
		CreatedByType:       req.CreatedByType,
		Metadata:            metadata,
	}

	return r.Credit(ctx, creditReq)
}

// ProcessTradeLoss debits user account for trade loss
// User → System (NO FEES)
func (r *transactionRepo) ProcessTradeLoss(
	ctx context.Context,
	req *domain.TradeRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Build metadata with trade information
	metadata := req.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["trade_id"] = req.TradeID
	metadata["trade_type"] = req.TradeType
	metadata["trade_result"] = "loss"

	// Create debit request
	debitReq := &domain.DebitRequest{
		AccountNumber:       req.AccountNumber,
		Amount:              req.Amount,
		Currency:            req.Currency,
		AccountType:         req.AccountType,
		ReceiptCode: req.ReceiptCode,
		Description:         fmt.Sprintf("Trade loss: %s (%s)", req.TradeID, req.TradeType),
		IdempotencyKey:      req.IdempotencyKey,
		ExternalRef:         &req.TradeID,
		CreatedByExternalID: req.CreatedByExternalID,
		CreatedByType:       req.CreatedByType,
		Metadata:            metadata,
	}

	return r.Debit(ctx, debitReq)
}

// ========================================
// AGENT COMMISSION
// ========================================

// ProcessAgentCommission pays commission to agent
// System Fee Account → Agent Commission Account (NO FEES)
func (r *transactionRepo) ProcessAgentCommission(
	ctx context.Context,
	req *domain.AgentCommissionRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get agent commission account
	agentAccount, err := r.accountRepo.GetAgentAccount(ctx, req.AgentExternalID, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent account: %w", err)
	}

	// Get system fee account
	systemFeeAccount, err := r.accountRepo.GetSystemFeeAccount(ctx, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("failed to get system fee account: %w", err)
	}

	// Build metadata
	metadata := make(map[string]interface{})
	metadata["agent_id"] = req.AgentExternalID
	metadata["original_txn"] = req.TransactionRef
	metadata["transaction_amount"] = req.TransactionAmount
	if req.CommissionRate != nil {
		metadata["commission_rate"] = *req.CommissionRate
	}

	// Build transaction request
	txReq := &domain.TransactionRequest{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     domain.TransactionTypeCommission,
		AccountType:         domain.AccountTypeReal, // Commissions always real
		ExternalRef:         &req.TransactionRef,
		Description:         strPtr(fmt.Sprintf("Agent commission: %s", req.TransactionRef)),
		CreatedByExternalID: strPtr("system"),
		CreatedByType:       ptrOwnerType(domain.OwnerTypeSystem),
		IsSystemTransaction: true, // NO FEES on commission payment
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: systemFeeAccount.AccountNumber,
				Amount:        req.CommissionAmount,
				DrCr:          domain.DrCrDebit,
				Currency:      req.Currency,
				ReceiptCode: req.ReceiptCode,
				Description:   strPtr(fmt.Sprintf("Commission payment for %s", req.TransactionRef)),
				Metadata:      metadata,
			},
			{
				AccountNumber: agentAccount.AccountNumber,
				Amount:        req.CommissionAmount,
				DrCr:          domain.DrCrCredit,
				ReceiptCode: req.ReceiptCode,
				Currency:      req.Currency,
				Description:   strPtr(fmt.Sprintf("Commission earned: %s", req.TransactionRef)),
				Metadata:      metadata,
			},
		},
		GenerateReceipt: true,
	}

	return r.ExecuteTransaction(ctx, txReq)
}

// ========================================
// CORE TRANSACTION EXECUTION
// ========================================

// ExecuteTransaction executes with automatic locking strategy
func (r *transactionRepo) ExecuteTransaction(
	ctx context.Context, 
	req *domain.TransactionRequest,
) (*domain.LedgerAggregate, error) {
	return r.ExecuteTransactionPessimistic(ctx, req)
}

// ExecuteTransactionOptimistic uses version-based locking
func (r *transactionRepo) ExecuteTransactionOptimistic(
	ctx context.Context, 
	req *domain.TransactionRequest,
) (*domain.LedgerAggregate, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}
	
	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := r.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, xerrors.ErrNotFound) {
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
	}
	
	// Start transaction
	tx, err := r.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	
	// Create journal
	journal, err := r.createJournal(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	
	// Fetch accounts without locking
	accountMap, accountVersions, err := r.fetchAccountsOptimistic(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	
	// Validate balances
	if err := r.validateBalances(ctx, accountMap, req); err != nil {
		return nil, err
	}
	
	// Create ledgers
	ledgers, err := r.createLedgers(ctx, tx, journal.ID, accountMap, req)
	if err != nil {
		return nil, err
	}
	
	// Update balances with optimistic locking
	if err := r.updateBalancesOptimistic(ctx, tx, accountMap, accountVersions, req, ledgers); err != nil {
		return nil, err
	}
	
	// Create fees if not system transaction
	if err := r.createFees(ctx, tx, journal, req); err != nil {
		return nil, err
	}
	
	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}, nil
}

// ExecuteTransactionPessimistic uses SELECT FOR UPDATE locking
func (r *transactionRepo) ExecuteTransactionPessimistic(
	ctx context.Context, 
	req *domain.TransactionRequest,
) (*domain.LedgerAggregate, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}
	
	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := r.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, xerrors.ErrNotFound) {
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
	}
	
	// Start transaction
	tx, err := r.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	
	// Create journal
	journal, err := r.createJournal(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	
	// Lock accounts in deterministic order
	accountMap, balanceMap, err := r.lockAccountsPessimistic(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	
	// Validate balances
	if err := r.validateBalancesPessimistic(accountMap, balanceMap, req); err != nil {
		return nil, err
	}
	
	// Create ledgers with balance_after
	ledgers, err := r.createLedgersWithBalance(ctx, tx, journal.ID, accountMap, balanceMap, req)
	if err != nil {
		return nil, err
	}
	
	// Update balances (already locked)
	if err := r.updateBalancesPessimistic(ctx, tx, ledgers); err != nil {
		return nil, err
	}
	
	// Create fees if not system transaction
	if err := r.createFees(ctx, tx, journal, req); err != nil {
		return nil, err
	}
	
	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}, nil
}

// ========================================
// HELPER METHODS
// ========================================

// createJournal creates a journal entry
func (r *transactionRepo) createJournal(
	ctx context.Context, 
	tx pgx.Tx, 
	req *domain.TransactionRequest,
) (*domain.Journal, error) {
	journalCreate := &domain.JournalCreate{
		IdempotencyKey:      req.IdempotencyKey,
		TransactionType:     req.TransactionType,
		AccountType:         req.AccountType,
		ExternalRef:         req.ExternalRef,
		Description:         req.Description,
		CreatedByExternalID: req.CreatedByExternalID,
		CreatedByType:       req.CreatedByType,
		IPAddress:           req.IPAddress,
		UserAgent:           req.UserAgent,
	}
	
	journal, err := r.journalRepo.Create(ctx, tx, journalCreate)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, xerrors.ErrDuplicateIdempotencyKey
		}
		return nil, fmt.Errorf("failed to create journal: %w", err)
	}
	
	return journal, nil
}

// fetchAccountsOptimistic fetches accounts without locking
func (r *transactionRepo) fetchAccountsOptimistic(
	ctx context.Context, 
	tx pgx.Tx, 
	req *domain.TransactionRequest,
) (map[string]*domain.Account, map[int64]int64, error) {
	accountMap := make(map[string]*domain.Account)
	accountVersions := make(map[int64]int64)
	
	for _, entry := range req.Entries {
		if _, exists := accountMap[entry.AccountNumber]; exists {
			continue
		}
		
		account, err := r.accountRepo.GetByAccountNumberTx(ctx, entry.AccountNumber, tx)
		if err != nil {
			return nil, nil, fmt.Errorf("account %s not found: %w", entry.AccountNumber, err)
		}
		
		if err := r.validateAccount(account, req.AccountType); err != nil {
			return nil, nil, fmt.Errorf("account %s: %w", entry.AccountNumber, err)
		}
		
		accountMap[entry.AccountNumber] = account
		
		balance, err := r.balanceRepo.GetByAccountID(ctx, account.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get balance for %s: %w", entry.AccountNumber, err)
		}
		accountVersions[account.ID] = balance.Version
	}
	
	return accountMap, accountVersions, nil
}

// lockAccountsPessimistic locks accounts in sorted order
func (r *transactionRepo) lockAccountsPessimistic(
	ctx context.Context, 
	tx pgx.Tx, 
	req *domain.TransactionRequest,
) (map[string]*domain.Account, map[int64]*domain.Balance, error) {
	// Get unique account numbers
	accountNumbers := make([]string, 0, len(req.Entries))
	accountNumberSet := make(map[string]bool)
	
	for _, entry := range req.Entries {
		if !accountNumberSet[entry.AccountNumber] {
			accountNumbers = append(accountNumbers, entry.AccountNumber)
			accountNumberSet[entry.AccountNumber] = true
		}
	}
	
	// Sort to prevent deadlocks
	sortAccountNumbers(accountNumbers)
	
	// Lock accounts and balances
	accountMap := make(map[string]*domain.Account)
	balanceMap := make(map[int64]*domain.Balance)
	
	for _, accountNumber := range accountNumbers {
		account, err := r.accountRepo.GetByAccountNumberTx(ctx, accountNumber, tx)
		if err != nil {
			return nil, nil, fmt.Errorf("account %s not found: %w", accountNumber, err)
		}
		
		if err := r.validateAccount(account, req.AccountType); err != nil {
			return nil, nil, fmt.Errorf("account %s: %w", accountNumber, err)
		}
		
		accountMap[accountNumber] = account
		
		balance, err := r.balanceRepo.GetByAccountIDWithLock(ctx, tx, account.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to lock balance for %s: %w", accountNumber, err)
		}
		balanceMap[account.ID] = balance
	}
	
	return accountMap, balanceMap, nil
}

// validateAccount checks account status
func (r *transactionRepo) validateAccount(account *domain.Account, requiredType domain.AccountType) error {
	if !account.IsActive {
		return xerrors.ErrAccountInactive
	}
	if account.IsLocked {
		return xerrors.ErrAccountLocked
	}
	if account.AccountType != requiredType {
		return errors.New("account type mismatch")
	}
	return nil
}

// validateBalances checks sufficient balance for debits
func (r *transactionRepo) validateBalances(
	ctx context.Context, 
	accountMap map[string]*domain.Account, 
	req *domain.TransactionRequest,
) error {
	for _, entry := range req.Entries {
		if entry.DrCr != domain.DrCrDebit {
			continue
		}
		
		account := accountMap[entry.AccountNumber]
		balance, err := r.balanceRepo.GetByAccountID(ctx, account.ID)
		if err != nil {
			return fmt.Errorf("failed to get balance: %w", err)
		}
		
		availableWithOverdraft := balance.AvailableBalance + account.OverdraftLimit
		if availableWithOverdraft < entry.Amount {
			return fmt.Errorf("account %s: %w (available: %d, required: %d)", 
				entry.AccountNumber, xerrors.ErrInsufficientBalance, 
				balance.AvailableBalance, entry.Amount)
		}
	}
	return nil
}

// validateBalancesPessimistic checks balances with locked data
func (r *transactionRepo) validateBalancesPessimistic(
	accountMap map[string]*domain.Account, 
	balanceMap map[int64]*domain.Balance, 
	req *domain.TransactionRequest,
) error {
	for _, entry := range req.Entries {
		if entry.DrCr != domain.DrCrDebit {
			continue
		}
		
		account := accountMap[entry.AccountNumber]
		balance := balanceMap[account.ID]
		
		availableWithOverdraft := balance.AvailableBalance + account.OverdraftLimit
		if availableWithOverdraft < entry.Amount {
			return fmt.Errorf("account %s: %w (available: %d, required: %d)", 
				entry.AccountNumber, xerrors.ErrInsufficientBalance, 
				balance.AvailableBalance, entry.Amount)
		}
	}
	return nil
}
// createLedgers creates ledger entries
func (r *transactionRepo) createLedgers(
	ctx context.Context, 
	tx pgx.Tx, 
	journalID int64, 
	accountMap map[string]*domain.Account, 
	req *domain.TransactionRequest,
) ([]*domain.Ledger, error) {
	var ledgers []*domain.Ledger
	
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		
		metadata, err := marshalMetadata(entry.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		
		ledgerCreate := &domain.LedgerCreate{
			JournalID:   journalID,
			AccountID:   account.ID,
			AccountType: req.AccountType,
			Amount:      entry.Amount,
			DrCr:        entry.DrCr,
			Currency:    entry.Currency,
			ReceiptCode: entry.ReceiptCode,
			Description: entry.Description,
			Metadata:    metadata,
		}
		
		ledger, err := r.ledgerRepo.Create(ctx, tx, ledgerCreate)
		if err != nil {
			return nil, fmt.Errorf("failed to create ledger: %w", err)
		}
		
		ledgers = append(ledgers, ledger)
	}
	
	return ledgers, nil
}
// createLedgersWithBalance creates ledgers with balance_after
func (r *transactionRepo) createLedgersWithBalance(
	ctx context.Context, 
	tx pgx.Tx, 
	journalID int64, 
	accountMap map[string]*domain.Account, 
	balanceMap map[int64]*domain.Balance, 
	req *domain.TransactionRequest,
) ([]*domain.Ledger, error) {
	var ledgers []*domain.Ledger
	
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		balance := balanceMap[account.ID]
		
		// Calculate new balance
		var newBalance int64
		if entry.DrCr == domain.DrCrCredit {
			newBalance = balance.Balance + entry.Amount
		} else {
			newBalance = balance.Balance - entry.Amount
		}
		
		metadata, err := marshalMetadata(entry.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		
		ledgerCreate := &domain.LedgerCreate{
			JournalID:    journalID,
			AccountID:    account.ID,
			AccountType:  req.AccountType,
			Amount:       entry.Amount,
			DrCr:         entry.DrCr,
			Currency:     entry.Currency,
			ReceiptCode:  entry.ReceiptCode,
			BalanceAfter: &newBalance,
			Description:  entry.Description,
			Metadata:     metadata,
		}
		
		ledger, err := r.ledgerRepo.Create(ctx, tx, ledgerCreate)
		if err != nil {
			return nil, fmt.Errorf("failed to create ledger: %w", err)
		}
		
		ledgers = append(ledgers, ledger)
		
		// Update local balance for next entry
		balance.Balance = newBalance
		if entry.DrCr == domain.DrCrCredit {
			balance.AvailableBalance += entry.Amount
		} else {
			balance.AvailableBalance -= entry.Amount
		}
	}
	
	return ledgers, nil
}

// updateBalancesOptimistic updates with version checking
func (r *transactionRepo) updateBalancesOptimistic(
	ctx context.Context, 
	tx pgx.Tx, 
	accountMap map[string]*domain.Account, 
	accountVersions map[int64]int64, 
	req *domain.TransactionRequest, 
	ledgers []*domain.Ledger,
) error {
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		expectedVersion := accountVersions[account.ID]
		
		update := &domain.BalanceUpdate{
			AccountID: account.ID,
			Amount:    entry.Amount,
			DrCr:      string(entry.DrCr),
			LedgerID:  ledgers[0].ID,
		}
		
		err := r.balanceRepo.UpdateBalanceOptimistic(ctx, tx, update, expectedVersion)
		if err != nil {
			if errors.Is(err, xerrors.ErrVersionMismatch) {
				return xerrors.ErrConcurrentModification
			}
			return fmt.Errorf("failed to update balance: %w", err)
		}
	}
	return nil
}

// updateBalancesPessimistic updates locked balances
func (r *transactionRepo) updateBalancesPessimistic(
	ctx context.Context, 
	tx pgx.Tx, 
	ledgers []*domain.Ledger,
) error {
	for _, ledger := range ledgers {
		update := &domain.BalanceUpdate{
			AccountID: ledger.AccountID,
			Amount:    ledger.Amount,
			DrCr:      string(ledger.DrCr),
			LedgerID:  ledger.ID,
		}
		
		err := r.balanceRepo.UpdateBalance(ctx, tx, update)
		if err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}
	}
	return nil
}

// createFees creates transaction fees
func (r *transactionRepo) createFees(
	ctx context.Context, 
	tx pgx.Tx, 
	journal *domain. Journal, 
	req *domain.TransactionRequest,
) error {
	if r.feeRepo == nil {
		return nil // Fee repo not configured
	}
	
	// 🔥 FIX: Safely determine receipt code with proper nil checks
	var receiptCode string
	
	// Priority 1: Use req.ReceiptCode if available
	if req.ReceiptCode != nil && *req.ReceiptCode != "" {
		receiptCode = *req.ReceiptCode
	} else if journal.ExternalRef != nil && *journal.ExternalRef != "" {
		// Priority 2: Fall back to journal.ExternalRef
		receiptCode = *journal.ExternalRef
	} else if req. ExternalRef != nil && *req.ExternalRef != "" {
		// Priority 3: Fall back to req.ExternalRef

		receiptCode = *req. ExternalRef
	}
	
	// 🔥 CRITICAL: If no receipt code available, return error or skip fee creation
	if receiptCode == "" {
		// Option 1: Return error (strict)
		// return fmt.Errorf("cannot create fee: no receipt code available")
		
		// Option 2: Skip fee creation (lenient)
		r.logger.Warn("skipping fee creation: no receipt code available",
			zap.Int64("journal_id", journal.ID))
		return nil
	}
	
	if req.IsSystemTransaction {
		// Create 0-fee record for system transactions
		fee := &domain.TransactionFee{
			ReceiptCode: receiptCode,  // ✅ Safe - guaranteed non-empty
			FeeType:     domain.FeeTypePlatform,
			Amount:      0,
			Currency:    req.GetCurrency(),
		}
		return r.feeRepo.Create(ctx, tx, fee)
	}
	
	// Calculate and create actual fees for non-system transactions
	// This would call your fee calculation logic
	// For now, just create a placeholder
	
	return nil
}
// GetByIdempotencyKey retrieves transaction by idempotency key
func (r *transactionRepo) GetByIdempotencyKey(
	ctx context.Context, 
	key string,
) (*domain.LedgerAggregate, error) {
	journal, err := r.journalRepo.GetByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, err
	}
	
	ledgers, err := r.ledgerRepo.ListByJournal(ctx, journal.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledgers: %w", err)
	}

	return &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}, nil
}
// sortAccountNumbers sorts account numbers alphabetically
func sortAccountNumbers(numbers []string) {
	for i := 0; i < len(numbers); i++ {
		for j := i + 1; j < len(numbers); j++ {
			if numbers[i] > numbers[j] {
				numbers[i], numbers[j] = numbers[j], numbers[i]
			}
		}
	}
}

// marshalMetadata converts map[string]interface{} to json.RawMessage
func marshalMetadata(m map[string]interface{}) (json.RawMessage, error) {
	if m == nil {
		return nil, nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// Helper pointer functions
func strPtr(s string) *string {
	return &s
}

func ptrOwnerType(t domain.OwnerType) *domain.OwnerType {
	return &t
}
package repository

import (
	"context"
	"errors"
	"fmt"
	//"time"

	"accounting-service/internal/domain"
	xerrors "x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository interface {
	// Execute a complete transaction atomically
	ExecuteTransaction(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error)
	
	// Execute with explicit locking strategy
	ExecuteTransactionOptimistic(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error)
	ExecuteTransactionPessimistic(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error)
	
	// Idempotency check
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.LedgerAggregate, error)
	
	// Transaction management
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type transactionRepo struct {
	db          *pgxpool.Pool
	accountRepo AccountRepository
	journalRepo JournalRepository
	ledgerRepo  LedgerRepository
	balanceRepo BalanceRepository
	// receiptRepo ReceiptRepository // Add when receipts are implemented
	// feeRepo     TransactionFeeRepository // Add when fees are implemented
}

func NewTransactionRepo(
	db *pgxpool.Pool,
	accountRepo AccountRepository,
	journalRepo JournalRepository,
	ledgerRepo LedgerRepository,
	balanceRepo BalanceRepository,
) TransactionRepository {
	return &transactionRepo{
		db:          db,
		accountRepo: accountRepo,
		journalRepo: journalRepo,
		ledgerRepo:  ledgerRepo,
		balanceRepo: balanceRepo,
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

// ExecuteTransaction executes a transaction with automatic locking strategy
// Uses pessimistic locking by default for safety
func (r *transactionRepo) ExecuteTransaction(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error) {
	return r.ExecuteTransactionPessimistic(ctx, req)
}

// ExecuteTransactionOptimistic uses optimistic locking (version-based)
// Better for high-contention scenarios with retry logic
// Suitable for 4000+ req/sec with proper retry handling at service layer
func (r *transactionRepo) ExecuteTransactionOptimistic(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}
	
	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := r.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil // Already processed
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
	
	// Step 1: Create journal
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
		// Check for duplicate idempotency key
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, xerrors.ErrDuplicateIdempotencyKey
		}
		return nil, fmt.Errorf("failed to create journal: %w", err)
	}
	
	// Step 2: Fetch accounts and validate
	accountMap := make(map[string]*domain.Account)
	accountVersions := make(map[int64]int64) // account_id -> expected version
	
	for _, entry := range req.Entries {
		if _, exists := accountMap[entry.AccountNumber]; exists {
			continue // Already fetched
		}
		
		account, err := r.accountRepo.GetByAccountNumberTx(ctx, entry.AccountNumber, tx)
		if err != nil {
			return nil, fmt.Errorf("account %s not found: %w", entry.AccountNumber, err)
		}
		
		// Validate account
		if !account.IsActive {
			return nil, fmt.Errorf("account %s: %w", entry.AccountNumber, xerrors.ErrAccountInactive)
		}
		if account.IsLocked {
			return nil, fmt.Errorf("account %s: %w", entry.AccountNumber, xerrors.ErrAccountLocked)
		}
		if account.AccountType != req.AccountType {
			return nil, fmt.Errorf("account %s: account type mismatch", entry.AccountNumber)
		}
		
		accountMap[entry.AccountNumber] = account
		
		// Get current balance version (no lock)
		balance, err := r.balanceRepo.GetByAccountID(ctx, account.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get balance for account %s: %w", entry.AccountNumber, err)
		}
		accountVersions[account.ID] = balance.Version
	}
	
	// Step 3: Validate balances (pre-check before updates)
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		
		if entry.DrCr == domain.DrCrDebit {
			balance, err := r.balanceRepo.GetByAccountID(ctx, account.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get balance: %w", err)
			}
			
			requiredBalance := entry.Amount
			availableWithOverdraft := balance.AvailableBalance + account.OverdraftLimit
			
			if availableWithOverdraft < requiredBalance {
				return nil, fmt.Errorf("account %s: %w (available: %d, required: %d)", 
					entry.AccountNumber, xerrors.ErrInsufficientBalance, balance.AvailableBalance, requiredBalance)
			}
		}
	}
	
	// Step 4: Create ledger entries
	var ledgers []*domain.Ledger
	
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		
		ledgerCreate := &domain.LedgerCreate{
			JournalID:   journal.ID,
			AccountID:   account.ID,
			AccountType: req.AccountType,
			Amount:      entry.Amount,
			DrCr:        entry.DrCr,
			Currency:    entry.Currency,
			ReceiptCode: entry.ReceiptCode,
			Description: entry.Description,
		}
		
		ledger, err := r.ledgerRepo.Create(ctx, tx, ledgerCreate)
		if err != nil {
			return nil, fmt.Errorf("failed to create ledger entry: %w", err)
		}
		
		ledgers = append(ledgers, ledger)
	}
	
	// Step 5: Update balances (optimistic locking)
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		expectedVersion := accountVersions[account.ID]
		
		update := &domain.BalanceUpdate{
			AccountID: account.ID,
			Amount:    entry.Amount,
			DrCr:      string(entry.DrCr),
			LedgerID:  ledgers[0].ID, // Use first ledger ID
		}
		
		err := r.balanceRepo.UpdateBalanceOptimistic(ctx, tx, update, expectedVersion)
		if err != nil {
			if errors.Is(err, xerrors.ErrVersionMismatch) {
				return nil, xerrors.ErrConcurrentModification
			}
			return nil, fmt.Errorf("failed to update balance for account %s: %w", entry.AccountNumber, err)
		}
	}
	
	// Step 6: Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	// Return aggregate
	return &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}, nil
}

// ExecuteTransactionPessimistic uses pessimistic locking (SELECT FOR UPDATE)
// Safer but slower, good for low-contention scenarios
// Use for critical transactions where race conditions must be prevented
func (r *transactionRepo) ExecuteTransactionPessimistic(ctx context.Context, req *domain.TransactionRequest) (*domain.LedgerAggregate, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}
	
	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := r.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil // Already processed
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
	
	// Step 1: Create journal
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
		// Check for duplicate idempotency key
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, xerrors.ErrDuplicateIdempotencyKey
		}
		return nil, fmt.Errorf("failed to create journal: %w", err)
	}
	
	// Step 2: Lock accounts in deterministic order (prevent deadlocks)
	// Sort account numbers alphabetically
	accountNumbers := make([]string, 0, len(req.Entries))
	accountNumberSet := make(map[string]bool)
	
	for _, entry := range req.Entries {
		if !accountNumberSet[entry.AccountNumber] {
			accountNumbers = append(accountNumbers, entry.AccountNumber)
			accountNumberSet[entry.AccountNumber] = true
		}
	}
	
	// Sort to prevent deadlocks
	for i := 0; i < len(accountNumbers); i++ {
		for j := i + 1; j < len(accountNumbers); j++ {
			if accountNumbers[i] > accountNumbers[j] {
				accountNumbers[i], accountNumbers[j] = accountNumbers[j], accountNumbers[i]
			}
		}
	}
	
	// Lock accounts and fetch balances
	accountMap := make(map[string]*domain.Account)
	balanceMap := make(map[int64]*domain.Balance)
	
	for _, accountNumber := range accountNumbers {
		account, err := r.accountRepo.GetByAccountNumberTx(ctx, accountNumber, tx)
		if err != nil {
			return nil, fmt.Errorf("account %s not found: %w", accountNumber, err)
		}
		
		// Validate account
		if !account.IsActive {
			return nil, fmt.Errorf("account %s: %w", accountNumber, xerrors.ErrAccountInactive)
		}
		if account.IsLocked {
			return nil, fmt.Errorf("account %s: %w", accountNumber, xerrors.ErrAccountLocked)
		}
		if account.AccountType != req.AccountType {
			return nil, fmt.Errorf("account %s: account type mismatch", accountNumber)
		}
		
		accountMap[accountNumber] = account
		
		// Lock balance (SELECT FOR UPDATE)
		balance, err := r.balanceRepo.GetByAccountIDWithLock(ctx, tx, account.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to lock balance for account %s: %w", accountNumber, err)
		}
		balanceMap[account.ID] = balance
	}
	
	// Step 3: Validate balances
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		balance := balanceMap[account.ID]
		
		if entry.DrCr == domain.DrCrDebit {
			requiredBalance := entry.Amount
			availableWithOverdraft := balance.AvailableBalance + account.OverdraftLimit
			
			if availableWithOverdraft < requiredBalance {
				return nil, fmt.Errorf("account %s: %w (available: %d, required: %d)", 
					entry.AccountNumber, xerrors.ErrInsufficientBalance, balance.AvailableBalance, requiredBalance)
			}
		}
	}
	
	// Step 4: Create ledger entries and update balances
	var ledgers []*domain.Ledger
	
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountNumber]
		balance := balanceMap[account.ID]
		
		// Calculate new balance
		var newBalance, newAvailable int64
		if entry.DrCr == domain.DrCrCredit {
			newBalance = balance.Balance + entry.Amount
			newAvailable = balance.AvailableBalance + entry.Amount
		} else {
			newBalance = balance.Balance - entry.Amount
			newAvailable = balance.AvailableBalance - entry.Amount
		}
		
		// Create ledger entry with balance_after
		ledgerCreate := &domain.LedgerCreate{
			JournalID:    journal.ID,
			AccountID:    account.ID,
			AccountType:  req.AccountType,
			Amount:       entry.Amount,
			DrCr:         entry.DrCr,
			Currency:     entry.Currency,
			ReceiptCode:  entry.ReceiptCode,
			BalanceAfter: &newBalance,
			Description:  entry.Description,
		}
		
		ledger, err := r.ledgerRepo.Create(ctx, tx, ledgerCreate)
		if err != nil {
			return nil, fmt.Errorf("failed to create ledger entry: %w", err)
		}
		
		ledgers = append(ledgers, ledger)
		
		// Update balance (already locked)
		update := &domain.BalanceUpdate{
			AccountID: account.ID,
			Amount:    entry.Amount,
			DrCr:      string(entry.DrCr),
			LedgerID:  ledger.ID,
		}
		
		err = r.balanceRepo.UpdateBalance(ctx, tx, update)
		if err != nil {
			return nil, fmt.Errorf("failed to update balance for account %s: %w", entry.AccountNumber, err)
		}
		
		// Update local balance cache
		balance.Balance = newBalance
		balance.AvailableBalance = newAvailable
		balance.Version++
	}
	
	// Step 5: Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	// Return aggregate
	return &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}, nil
}

// GetByIdempotencyKey retrieves a transaction by idempotency key
func (r *transactionRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.LedgerAggregate, error) {
	// Get journal
	journal, err := r.journalRepo.GetByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, err
	}
	
	// Get ledgers
	ledgers, err := r.ledgerRepo.ListByJournal(ctx, journal.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledgers: %w", err)
	}
	
	return &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}, nil
}

// =====================================================================
// HELPER FUNCTIONS FOR COMMON TRANSACTION PATTERNS
// =====================================================================

// ExecuteSimpleTransfer is a convenience method for simple A->B transfers
func (r *transactionRepo) ExecuteSimpleTransfer(
	ctx context.Context,
	fromAccount, toAccount string,
	amount int64,
	currency string,
	accountType domain.AccountType,
	idempotencyKey *string,
) (*domain.LedgerAggregate, error) {
	req := domain.SimpleTransfer(fromAccount, toAccount, amount, currency, accountType)
	req.IdempotencyKey = idempotencyKey
	
	return r.ExecuteTransaction(ctx, req)
}

// ExecuteDeposit creates a deposit transaction (credit user, debit system liquidity)
func (r *transactionRepo) ExecuteDeposit(
	ctx context.Context,
	userAccount string,
	amount int64,
	currency string,
	accountType domain.AccountType,
	externalRef *string,
	idempotencyKey *string,
) (*domain.LedgerAggregate, error) {
	systemAccount := fmt.Sprintf("SYS-LIQ-%s", currency)
	
	req := &domain.TransactionRequest{
		IdempotencyKey:  idempotencyKey,
		TransactionType: domain.TransactionTypeDeposit,
		AccountType:     accountType,
		ExternalRef:     externalRef,
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: systemAccount,
				Amount:        amount,
				DrCr:          domain.DrCrDebit,
				Currency:      currency,
			},
			{
				AccountNumber: userAccount,
				Amount:        amount,
				DrCr:          domain.DrCrCredit,
				Currency:      currency,
			},
		},
		GenerateReceipt: true,
	}
	
	return r.ExecuteTransaction(ctx, req)
}

// ExecuteWithdrawal creates a withdrawal transaction (debit user, credit system liquidity)
func (r *transactionRepo) ExecuteWithdrawal(
	ctx context.Context,
	userAccount string,
	amount int64,
	currency string,
	accountType domain.AccountType,
	externalRef *string,
	idempotencyKey *string,
) (*domain.LedgerAggregate, error) {
	systemAccount := fmt.Sprintf("SYS-LIQ-%s", currency)
	
	req := &domain.TransactionRequest{
		IdempotencyKey:  idempotencyKey,
		TransactionType: domain.TransactionTypeWithdrawal,
		AccountType:     accountType,
		ExternalRef:     externalRef,
		Entries: []*domain.LedgerEntryRequest{
			{
				AccountNumber: userAccount,
				Amount:        amount,
				DrCr:          domain.DrCrDebit,
				Currency:      currency,
			},
			{
				AccountNumber: systemAccount,
				Amount:        amount,
				DrCr:          domain.DrCrCredit,
				Currency:      currency,
			},
		},
		GenerateReceipt: true,
	}
	
	return r.ExecuteTransaction(ctx, req)
}
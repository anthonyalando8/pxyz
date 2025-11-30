package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"
	xerrors "x/shared/utils/errors"
	helpers "x/shared/utils/profile"

	authclient "x/shared/auth"
	receiptclient "x/shared/common/receipt"
	notificationclient "x/shared/notification"
	partnerclient "x/shared/partner"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"
	notificationpb "x/shared/genproto/shared/notificationpb"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

const (
	// Batch configuration
	ReceiptBatchSize      = 50  // Receipts per gRPC call
	NotificationBatchSize = 100 // Notifications per gRPC call
	BatchFlushInterval    = 50 * time.Millisecond

	// Worker pool
	NumProcessorWorkers = 50
	ProcessorQueueSize  = 10000

	// Status tracking
	StatusCacheTTL       = 5 * time.Minute
	StatusUpdateInterval = 1 * time.Second
)

// TransactionUsecase handles all transaction operations
type TransactionUsecase struct {
	// Repositories
	transactionRepo repository.TransactionRepository
	accountRepo     repository.AccountRepository
	balanceRepo     repository.BalanceRepository
	journalRepo     repository.JournalRepository
	ledgerRepo      repository.LedgerRepository
	feeRepo         repository.TransactionFeeRepository

	// Other usecases
	accountUC *AccountUsecase
	feeUC     *TransactionFeeUsecase
	feeRuleUC *TransactionFeeRuleUsecase

	// External clients
	authClient         *authclient.AuthService
	receiptClient      *receiptclient.ReceiptClientV3
	notificationClient *notificationclient.NotificationService
	partnerClient      *partnerclient.PartnerService

	// Infrastructure
	redisClient  *redis.Client
	kafkaWriter  *kafka.Writer
	profileFetch *helpers.ProfileFetcher

	// Batch processing
	receiptBatcher      *ReceiptBatcher
	notificationBatcher *NotificationBatcher
	statusTracker       *TransactionStatusTracker
	processorPool       *ProcessorPool
}

// NewTransactionUsecase creates a new transaction usecase
func NewTransactionUsecase(
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	balanceRepo repository.BalanceRepository,
	journalRepo repository.JournalRepository,
	ledgerRepo repository.LedgerRepository,
	feeRepo repository.TransactionFeeRepository,
	accountUC *AccountUsecase,
	feeUC *TransactionFeeUsecase,
	feeRuleUC *TransactionFeeRuleUsecase,
	authClient *authclient.AuthService,
	receiptClient *receiptclient.ReceiptClientV3,
	notificationClient *notificationclient.NotificationService,
	partnerClient *partnerclient.PartnerService,
	redisClient *redis.Client,
	kafkaWriter *kafka.Writer,
) *TransactionUsecase {
	uc := &TransactionUsecase{
		transactionRepo:    transactionRepo,
		accountRepo:        accountRepo,
		balanceRepo:        balanceRepo,
		journalRepo:        journalRepo,
		ledgerRepo:         ledgerRepo,
		feeRepo:            feeRepo,
		accountUC:          accountUC,
		feeUC:              feeUC,
		feeRuleUC:          feeRuleUC,
		authClient:         authClient,
		receiptClient:      receiptClient,
		notificationClient: notificationClient,
		partnerClient:      partnerClient,
		redisClient:        redisClient,
		kafkaWriter:        kafkaWriter,
		profileFetch:       helpers.NewProfileFetcher(authClient),
	}

	// Initialize batchers
	uc.receiptBatcher = NewReceiptBatcher(uc, receiptClient, ReceiptBatchSize, BatchFlushInterval)
	uc.notificationBatcher = NewNotificationBatcher(notificationClient, NotificationBatchSize, BatchFlushInterval)
	uc.statusTracker = NewTransactionStatusTracker(redisClient, StatusUpdateInterval)

	// Initialize processor pool
	uc.processorPool = NewProcessorPool(NumProcessorWorkers, ProcessorQueueSize, uc)

	// Start background workers
	uc.receiptBatcher.Start()
	uc.notificationBatcher.Start()
	uc.statusTracker.Start()
	uc.processorPool.Start()

	return uc
}

// ===============================
// TRANSACTION EXECUTION
// ===============================

// ExecuteTransaction - main entry point with batched receipt generation
func (uc *TransactionUsecase) ExecuteTransaction(
	ctx context.Context,
	req *domain.TransactionRequest,
) (*domain.TransactionResult, error) {
	startTime := time.Now()

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}

	// Check idempotency (if key provided)
	if req.IdempotencyKey != nil {
		if result := uc.checkIdempotency(ctx, *req.IdempotencyKey); result != nil {
			return result, nil
		}
	}

	// Pre-validate (fast checks)
	if err := uc.preValidateTransaction(ctx, req); err != nil {
		return nil, fmt.Errorf("pre-validation failed: %w", err)
	}

	// Generate receipt code via batch (NON-BLOCKING)
	receiptCodeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	uc.receiptBatcher.Add(&ReceiptRequest{
		TxnReq:     req,
		ResultChan: receiptCodeChan,
		ErrorChan:  errChan,
	})

	// Wait for receipt code (batched call completes quickly)
	var receiptCode string
	select {
	case receiptCode = <-receiptCodeChan:
	case err := <-errChan:
		return nil, fmt.Errorf("failed to generate receipt: %w", err)
	case <-time.After(2 * time.Second):
		return nil, errors.New("receipt generation timeout")
	}

	// Initialize status tracking
	uc.statusTracker.Track(receiptCode, "processing")

	// Return immediately
	result := &domain.TransactionResult{
		ReceiptCode:    receiptCode,
		Status:         "processing",
		Amount:         uc.getTotalAmount(req.Entries),
		Currency:       req.GetCurrency(),
		ProcessingTime: time.Since(startTime),
		CreatedAt:      time.Now(),
	}

	// Log request
	uc.logTransactionStart(receiptCode, req)

	// Queue for async processing
	uc.processorPool.Submit(&ProcessorTask{
		ReceiptCode:    receiptCode,
		Request:        req,
		IdempotencyKey: req.IdempotencyKey,
	})

	return result, nil
}

// ExecuteTransactionSync - for critical operations requiring immediate completion
func (uc *TransactionUsecase) ExecuteTransactionSync(
	ctx context.Context,
	req *domain.TransactionRequest,
) (*domain.TransactionResult, error) {
	startTime := time.Now()

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}

	// Check idempotency
	if req.IdempotencyKey != nil {
		if result := uc.checkIdempotency(ctx, *req.IdempotencyKey); result != nil {
			return result, nil
		}
	}

	// Pre-validate
	if err := uc.preValidateTransaction(ctx, req); err != nil {
		return nil, fmt.Errorf("pre-validation failed: %w", err)
	}

	// Generate receipt synchronously (still batched)
	receiptCodeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	uc.receiptBatcher.Add(&ReceiptRequest{
		TxnReq:     req,
		ResultChan: receiptCodeChan,
		ErrorChan:  errChan,
	})

	var receiptCode string
	select {
	case receiptCode = <-receiptCodeChan:
	case err := <-errChan:
		return nil, fmt.Errorf("failed to generate receipt: %w", err)
	case <-time.After(2 * time.Second):
		return nil, errors.New("receipt generation timeout")
	}

	// Track status
	uc.statusTracker.Track(receiptCode, "processing")
	uc.logTransactionStart(receiptCode, req)

	// Assign receipt to entries
	for _, entry := range req.Entries {
		entry.ReceiptCode = &receiptCode
	}

	// Store receipt code in request for journal creation
	req.ExternalRef = &receiptCode

	// Execute transaction
	aggregate, err := uc.transactionRepo.ExecuteTransaction(ctx, req)
	if err != nil {
		// Update status
		uc.statusTracker.Update(receiptCode, "failed", err.Error())
		uc.receiptBatcher.UpdateStatus(receiptCode, receiptpb.TransactionStatus_TRANSACTION_STATUS_FAILED, err.Error())
		uc.publishTransactionEvent(context.Background(), receiptCode, "failed", err.Error())
		uc.logTransactionError(receiptCode, err)

		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	// Update status
	uc.statusTracker.Update(receiptCode, "completed", "")
	uc.receiptBatcher.UpdateStatus(receiptCode, receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED, "")

	// Queue notifications (batched)
	uc.queueNotifications(receiptCode, aggregate)

	// Publish success
	uc.publishTransactionEvent(context.Background(), receiptCode, "completed", "")
	uc.logTransactionSuccess(receiptCode, aggregate.Journal.ID)

	// Invalidate caches
	uc.invalidateTransactionCaches(context.Background(), aggregate)

	return &domain.TransactionResult{
		ReceiptCode:    receiptCode,
		TransactionID:  aggregate.Journal.ID,
		Status:         "completed",
		Amount:         uc.getTotalAmount(req.Entries),
		Currency:       req.GetCurrency(),
		ProcessingTime: time.Since(startTime),
		CreatedAt:      aggregate.Journal.CreatedAt,
	}, nil
}

// ===============================
// BACKGROUND PROCESSING
// ===============================

// ProcessorTask represents a transaction to process
type ProcessorTask struct {
	ReceiptCode    string
	Request        *domain.TransactionRequest
	IdempotencyKey *string
}

// ProcessorPool manages worker goroutines
type ProcessorPool struct {
	workers  int
	taskChan chan *ProcessorTask
	uc       *TransactionUsecase
	wg       sync.WaitGroup
	stopChan chan struct{}
}

func NewProcessorPool(workers, queueSize int, uc *TransactionUsecase) *ProcessorPool {
	return &ProcessorPool{
		workers:  workers,
		taskChan: make(chan *ProcessorTask, queueSize),
		uc:       uc,
		stopChan: make(chan struct{}),
	}
}

func (p *ProcessorPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *ProcessorPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case task := <-p.taskChan:
			p.uc.processTransaction(id, task)
		case <-p.stopChan:
			return
		}
	}
}

func (p *ProcessorPool) Submit(task *ProcessorTask) {
	select {
	case p.taskChan <- task:
		// Successfully queued
	default:
		// Queue full - log critical error
		fmt.Printf("[CRITICAL] Processor queue full, dropping transaction: %s\n", task.ReceiptCode)
		// Update status to failed
		// TODO: Send alert to monitoring system
	}
}

func (p *ProcessorPool) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}

// processTransaction processes a single transaction
func (uc *TransactionUsecase) processTransaction(workerID int, task *ProcessorTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	receiptCode := task.ReceiptCode
	req := task.Request

	// Update status to executing
	uc.statusTracker.Update(receiptCode, "executing", "")

	// Assign receipt code to entries
	for _, entry := range req.Entries {
		entry.ReceiptCode = &receiptCode
	}

	// Store receipt code in request for journal creation
	req.ExternalRef = &receiptCode

	// Execute transaction
	startTime := time.Now()
	aggregate, err := uc.transactionRepo.ExecuteTransaction(ctx, req)
	duration := time.Since(startTime)

	if err != nil {
		// Transaction failed
		uc.statusTracker.Update(receiptCode, "failed", err.Error())
		uc.receiptBatcher.UpdateStatus(receiptCode, receiptpb.TransactionStatus_TRANSACTION_STATUS_FAILED, err.Error())
		uc.publishTransactionEvent(ctx, receiptCode, "failed", err.Error())
		uc.logTransactionError(receiptCode, err)
		return
	}

	// Transaction succeeded
	uc.statusTracker.Update(receiptCode, "completed", "")
	uc.receiptBatcher.UpdateStatus(receiptCode, receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED, "")

	// Queue notifications (batched)
	uc.queueNotifications(receiptCode, aggregate)

	// Publish success event
	uc.publishTransactionEvent(ctx, receiptCode, "completed", "")

	// Log success with metrics
	uc.logTransactionSuccessWithMetrics(workerID, receiptCode, aggregate.Journal.ID, duration)

	// Invalidate caches
	uc.invalidateTransactionCaches(ctx, aggregate)
}

// ===============================
// IDEMPOTENCY
// ===============================

func (uc *TransactionUsecase) checkIdempotency(ctx context.Context, key string) *domain.TransactionResult {
	// Check cache first
	cacheKey := fmt.Sprintf("idempotency:%s", key)
	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var result domain.TransactionResult
		if json.Unmarshal([]byte(val), &result) == nil {
			return &result
		}
	}

	// Check database
	aggregate, err := uc.transactionRepo.GetByIdempotencyKey(ctx, key)
	if err == nil && aggregate != nil {
		result := &domain.TransactionResult{
			ReceiptCode:   ptrStrToStr(aggregate.Journal.ExternalRef), // receipt code stored in external_ref
			TransactionID: aggregate.Journal.ID,
			Status:        "completed",
			CreatedAt:     aggregate.Journal.CreatedAt,
		}

		// Cache for future requests
		if data, err := json.Marshal(result); err == nil {
			_ = uc.redisClient.Set(ctx, cacheKey, data, 24*time.Hour).Err()
		}

		return result
	}

	return nil
}

// ===============================
// NOTIFICATION QUEUEING
// ===============================

func (uc *TransactionUsecase) queueNotifications(receiptCode string, aggregate *domain.LedgerAggregate) {
	if aggregate == nil || len(aggregate.Ledgers) < 2 {
		return
	}

	ctx := context.Background()
	accountOwners := make(map[string]bool)

	for _, ledger := range aggregate.Ledgers {
		account, err := uc.accountRepo.GetByID(ctx, ledger.AccountID)
		if err != nil || account.OwnerType == domain.OwnerTypeSystem {
			continue
		}

		ownerKey := fmt.Sprintf("%s:%s", account.OwnerType, account.OwnerID)
		if accountOwners[ownerKey] {
			continue
		}
		accountOwners[ownerKey] = true

		// Fetch profile
		profile, err := uc.profileFetch.FetchProfile(ctx, string(account.OwnerType), account.OwnerID)
		if err != nil {
			fmt.Printf("[NOTIFICATION] Failed to fetch profile for %s: %v\n", ownerKey, err)
			continue
		}

		// Determine notification details
		eventType := "transaction.credit"
		title := "Credit Transaction"
		body := fmt.Sprintf("Your account was credited with %d %s", ledger.Amount, ledger.Currency)

		if ledger.DrCr == domain.DrCrDebit {
			eventType = "transaction.debit"
			title = "Debit Transaction"
			body = fmt.Sprintf("Your account was debited %d %s", ledger.Amount, ledger.Currency)
		}

		notif := &notificationpb.Notification{
			RequestId:      fmt.Sprintf("txn-%d-%s", aggregate.Journal.ID, account.OwnerID),
			OwnerType:      string(account.OwnerType),
			OwnerId:        account.OwnerID,
			EventType:      eventType,
			ChannelHint:    []string{"email", "ws", "sms"},
			Title:          title,
			Body:           body,
			Priority:       "high",
			Status:         "pending",
			VisibleInApp:   true,
			RecipientEmail: profile.Email,
			RecipientPhone: profile.Phone,
			RecipientName:  fmt.Sprintf("%s %s", profile.FirstName, profile.LastName),
		}

		uc.notificationBatcher.Add(notif)
	}
}

// ===============================
// VALIDATION
// ===============================

func (uc *TransactionUsecase) preValidateTransaction(ctx context.Context, req *domain.TransactionRequest) error {
	for _, entry := range req.Entries {
		account, err := uc.accountRepo.GetByAccountNumber(ctx, entry.AccountNumber)
		if err != nil {
			return fmt.Errorf("account %s not found: %w", entry.AccountNumber, err)
		}

		if !account.IsActive {
			return fmt.Errorf("account %s is inactive", entry.AccountNumber)
		}

		if account.IsLocked {
			return fmt.Errorf("account %s is locked", entry.AccountNumber)
		}

		if account.AccountType != req.AccountType {
			return fmt.Errorf("account %s type mismatch", entry.AccountNumber)
		}

		// Quick balance check for debits
		if entry.DrCr == domain.DrCrDebit {
			balance, err := uc.balanceRepo.GetByAccountNumber(ctx, entry.AccountNumber)
			if err == nil && balance.AvailableBalance < entry.Amount {
				return fmt.Errorf("insufficient balance in %s: available=%d, required=%d",
					entry.AccountNumber, balance.AvailableBalance, entry.Amount)
			}
		}
	}

	return nil
}

// ===============================
// KAFKA EVENT PUBLISHING
// ===============================

type TransactionEvent struct {
	EventType     string    `json:"event_type"`
	ReceiptCode   string    `json:"receipt_code"`
	TransactionID int64     `json:"transaction_id,omitempty"`
	Status        string    `json:"status"`
	Amount        int64     `json:"amount,omitempty"`
	Currency      string    `json:"currency,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

func (uc *TransactionUsecase) publishTransactionEvent(ctx context.Context, receiptCode, status, errorMsg string) {
	if uc.kafkaWriter == nil {
		return
	}

	event := TransactionEvent{
		EventType:    fmt.Sprintf("transaction.%s", status),
		ReceiptCode:  receiptCode,
		Status:       status,
		ErrorMessage: errorMsg,
		Timestamp:    time.Now(),
	}

	eventBytes, _ := json.Marshal(event)

	err := uc.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(receiptCode),
		Value: eventBytes,
		Time:  time.Now(),
	})

	if err != nil {
		fmt.Printf("[KAFKA ERROR] Failed to publish event for %s: %v\n", receiptCode, err)
	}
}

// ===============================
// CACHE INVALIDATION
// ===============================

func (uc *TransactionUsecase) invalidateTransactionCaches(ctx context.Context, aggregate *domain.LedgerAggregate) {
	if aggregate == nil {
		return
	}

	for _, ledger := range aggregate.Ledgers {
		account, err := uc.accountRepo.GetByID(ctx, ledger.AccountID)
		if err != nil {
			continue
		}

		// Invalidate balance cache
		cacheKey := fmt.Sprintf("balance:account:%s", account.AccountNumber)
		_ = uc.redisClient.Del(ctx, cacheKey).Err()

		// Invalidate account cache
		accountCacheKey := fmt.Sprintf("accounts:number:%s", account.AccountNumber)
		_ = uc.redisClient.Del(ctx, accountCacheKey).Err()
	}
}

// ===============================
// TRANSACTION QUERIES
// ===============================

func (uc *TransactionUsecase) GetTransactionByReceipt(ctx context.Context, receiptCode string) (*domain.LedgerAggregate, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("transaction:receipt:%s", receiptCode)

	if val, err := uc.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var aggregate domain.LedgerAggregate
		if json.Unmarshal([]byte(val), &aggregate) == nil {
			return &aggregate, nil
		}
	}

	// Query by external_ref (receipt code)
	journals, err := uc.journalRepo.List(ctx, &domain.JournalFilter{
		ExternalRef: &receiptCode,
		Limit:       1,
	})

	if err != nil || len(journals) == 0 {
		return nil, xerrors.ErrNotFound
	}

	journal := journals[0]

	// Fetch ledgers
	ledgers, err := uc.ledgerRepo.ListByJournal(ctx, journal.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ledgers: %w", err)
	}

	aggregate := &domain.LedgerAggregate{
		Journal: journal,
		Ledgers: ledgers,
	}

	// Cache result
	if data, err := json.Marshal(aggregate); err == nil {
		_ = uc.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
	}

	return aggregate, nil
}

func (uc *TransactionUsecase) GetTransactionStatus(
	ctx context.Context,
	receiptCode string,
) (*TransactionStatus, error) {
	// Check status tracker first (in-memory + Redis)
	if trackedStatus := uc.statusTracker.Get(receiptCode); trackedStatus != "" {
		return &TransactionStatus{
			ReceiptCode: receiptCode,
			Status:      trackedStatus,
			StartedAt:   time.Now(), // You may want to track this separately
		}, nil
	}

	// Check receipt service
	if uc.receiptClient != nil {
		receipt, err := uc.receiptClient.Client.GetReceipt(ctx, &receiptpb.GetReceiptRequest{
			Code: receiptCode,
		})
		if err == nil && receipt != nil {
			statusStr := uc.convertReceiptStatus(receipt.Status)
			uc.statusTracker.Track(receiptCode, statusStr)

			result := &TransactionStatus{
				ReceiptCode: receiptCode,
				Status:      statusStr,
				StartedAt:   receipt.CreatedAt.AsTime(),
			}

			if receipt.ErrorMessage != "" {
				result.ErrorMessage = receipt.ErrorMessage
			}

			if receipt.UpdatedAt != nil {
				result.UpdatedAt = receipt.UpdatedAt.AsTime()
			}

			return result, nil
		}
	}

	// Check database
	aggregate, err := uc.GetTransactionByReceipt(ctx, receiptCode)
	if err == nil && aggregate != nil {
		return &TransactionStatus{
			ReceiptCode: receiptCode,
			Status:      "completed",
			StartedAt:   aggregate.Journal.CreatedAt,
			UpdatedAt:   aggregate.Journal.CreatedAt,
		}, nil
	}

	// Check if not found or still processing
	if err == xerrors.ErrNotFound {
		// Not in database, check if tracking exists
		if uc.statusTracker.Exists(receiptCode) {
			return &TransactionStatus{
				ReceiptCode: receiptCode,
				Status:      "processing",
				StartedAt:   time.Now(),
			}, nil
		}

		// Completely unknown receipt
		return nil, xerrors.ErrReceiptNotFound
	}

	// Default to processing if other errors
	return &TransactionStatus{
		ReceiptCode: receiptCode,
		Status:      "processing",
		StartedAt:   time.Now(),
	}, nil
}

func (uc *TransactionUsecase) convertReceiptStatus(status receiptpb.TransactionStatus) string {
	switch status {
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED:
		return "completed"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_FAILED:
		return "failed"
	case receiptpb.TransactionStatus_TRANSACTION_STATUS_PROCESSING:
		return "processing"
	default:
		return "unknown"
	}
}

// ===============================
// LOGGING
// ===============================

func (uc *TransactionUsecase) logTransactionStart(receiptCode string, req *domain.TransactionRequest) {
	fmt.Printf("[TRANSACTION START] Receipt: %s | Type: %s | Amount: %d %s | Accounts: %d\n",
		receiptCode, req.TransactionType, uc.getTotalAmount(req.Entries), req.GetCurrency(), len(req.Entries))
}

func (uc *TransactionUsecase) logTransactionSuccess(receiptCode string, journalID int64) {
	fmt.Printf("[TRANSACTION SUCCESS] Receipt: %s | Journal ID: %d\n", receiptCode, journalID)
}

func (uc *TransactionUsecase) logTransactionSuccessWithMetrics(workerID int, receiptCode string, journalID int64, duration time.Duration) {
	fmt.Printf("[TRANSACTION SUCCESS] Worker: %d | Receipt: %s | Journal ID: %d | Duration: %v\n",
		workerID, receiptCode, journalID, duration)
}

func (uc *TransactionUsecase) logTransactionError(receiptCode string, err error) {
	fmt.Printf("[TRANSACTION ERROR] Receipt: %s | Error: %v\n", receiptCode, err)
}

// ===============================
// HELPER METHODS
// ===============================

func (uc *TransactionUsecase) getTotalAmount(entries []*domain.LedgerEntryRequest) int64 {
	for _, entry := range entries {
		if entry.DrCr == domain.DrCrDebit {
			return entry.Amount
		}
	}
	return 0
}

// Shutdown gracefully stops all background workers
func (uc *TransactionUsecase) Shutdown() {
	fmt.Println("[SHUTDOWN] Stopping transaction usecase...")

	// Stop accepting new tasks
	if uc.processorPool != nil {
		fmt.Println("[SHUTDOWN] Stopping processor pool...")
		uc.processorPool.Stop()
	}

	// Flush remaining batches
	if uc.receiptBatcher != nil {
		fmt.Println("[SHUTDOWN] Flushing receipt batches...")
		uc.receiptBatcher.flushCreate()
		uc.receiptBatcher.flushUpdate()
	}

	if uc.notificationBatcher != nil {
		fmt.Println("[SHUTDOWN] Flushing notification batches...")
		uc.notificationBatcher.flush()
	}

	if uc.statusTracker != nil {
		fmt.Println("[SHUTDOWN] Stopping status tracker...")
		uc.statusTracker.Stop()
	}

	fmt.Println("[SHUTDOWN] Transaction usecase stopped successfully")
}

// Credit adds money to an account (system → user, NO FEES)
func (uc *TransactionUsecase) Credit(
	ctx context.Context,
	req *domain.CreditRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credit request: %w", err)
	}
	
	return uc.transactionRepo.Credit(ctx, req)
}

// Debit removes money from account (user → system, NO FEES)
func (uc *TransactionUsecase) Debit(
	ctx context.Context,
	req *domain.DebitRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid debit request: %w", err)
	}
	
	return uc.transactionRepo.Debit(ctx, req)
}

// Transfer moves money between accounts (P2P, FEES APPLY)
func (uc *TransactionUsecase) Transfer(
	ctx context.Context,
	req *domain.TransferRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transfer request: %w", err)
	}
	
	return uc.transactionRepo.Transfer(ctx, req)
}

// ConvertAndTransfer performs currency conversion (FEES APPLY)
func (uc *TransactionUsecase) ConvertAndTransfer(
	ctx context.Context,
	req *domain.ConversionRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid conversion request: %w", err)
	}
	
	return uc.transactionRepo.ConvertAndTransfer(ctx, req)
}

// ProcessTradeWin credits account for trade win (NO FEES)
func (uc *TransactionUsecase) ProcessTradeWin(
	ctx context.Context,
	req *domain.TradeRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid trade request: %w", err)
	}
	
	return uc.transactionRepo.ProcessTradeWin(ctx, req)
}

// ProcessTradeLoss debits account for trade loss (NO FEES)
func (uc *TransactionUsecase) ProcessTradeLoss(
	ctx context.Context,
	req *domain.TradeRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid trade request: %w", err)
	}
	
	return uc.transactionRepo.ProcessTradeLoss(ctx, req)
}

// ProcessAgentCommission pays commission to agent (NO FEES)
func (uc *TransactionUsecase) ProcessAgentCommission(
	ctx context.Context,
	req *domain.AgentCommissionRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid commission request: %w", err)
	}
	
	return uc.transactionRepo.ProcessAgentCommission(ctx, req)
}
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

	publisher "accounting-service/internal/pub"
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

	// Publishers
	eventPublisher *publisher.TransactionEventPublisher //  NEW
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
	eventPublisher *publisher.TransactionEventPublisher, //  NEW PARAMETER
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
		eventPublisher:     eventPublisher,
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

		// Publish failure to both Kafka and Redis
		uc.publishTransactionEvent(ctx, receiptCode, "failed", err.Error())
		uc.publishRedisFailureEvent(receiptCode, req, err) // ✅ NEW

		uc.logTransactionError(receiptCode, err)
		return
	}

	// Transaction succeeded
	uc.statusTracker.Update(receiptCode, "completed", "")
	uc.receiptBatcher.UpdateStatus(receiptCode, receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED, "")

	// Queue notifications (batched)
	uc.queueNotifications(receiptCode, aggregate)

	// Publish success event to both Kafka and Redis
	uc.publishTransactionEvent(ctx, receiptCode, "completed", "")
	uc.publishRedisCompletionEvent(aggregate, req) // ✅ NEW

	// Log success with metrics
	uc.logTransactionSuccessWithMetrics(workerID, receiptCode, aggregate.Journal.ID, duration)

	// Invalidate caches
	uc.invalidateTransactionCaches(ctx, aggregate)
}

// ✅ NEW: Publish generic transaction completion to Redis
func (uc *TransactionUsecase) publishRedisCompletionEvent(aggregate *domain.LedgerAggregate, req *domain.TransactionRequest) {
	if uc.eventPublisher == nil || aggregate == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var totalAmount float64
	var currency string
	var balanceAfter float64

	for _, ledger := range aggregate.Ledgers {
		if ledger.DrCr == domain.DrCrDebit {
			totalAmount = ledger.Amount
			currency = ledger.Currency
			balanceAfter = ptrFloat64ToFloat64(ledger.BalanceAfter)
			break
		}
	}

	err := uc.eventPublisher.PublishTransactionCompleted(
		ctx,
		ptrStrToStr(req.CreatedByExternalID),
		ptrStrToStr(aggregate.Journal.ExternalRef),
		aggregate.Journal.ID,
		string(req.TransactionType),
		currency,
		totalAmount,
		balanceAfter,
		0, // fee - you can calculate from aggregate.Fees
	)

	if err != nil {
		fmt.Printf("[ERROR] Failed to publish transaction completion event: %v\n", err)
	}
}

// ✅ NEW: Publish transaction failure to Redis
func (uc *TransactionUsecase) publishRedisFailureEvent(receiptCode string, req *domain.TransactionRequest, txErr error) {
	if uc.eventPublisher == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var totalAmount float64
	var currency string

	for _, entry := range req.Entries {
		if entry.DrCr == domain.DrCrDebit {
			totalAmount = entry.Amount
			currency = entry.Currency
			break
		}
	}

	err := uc.eventPublisher.PublishTransactionFailed(
		ctx,
		ptrStrToStr(req.CreatedByExternalID),
		receiptCode,
		0, // transaction ID not available on failure
		string(req.TransactionType),
		currency,
		totalAmount,
		txErr.Error(),
	)

	if err != nil {
		fmt.Printf("[ERROR] Failed to publish transaction failure event: %v\n", err)
	}
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
		body := fmt. Sprintf("Your account was credited with %.2f %s", ledger. Amount, ledger.Currency)

		if ledger.DrCr == domain.DrCrDebit {
			eventType = "transaction.debit"
			title = "Debit Transaction"
			body = fmt.Sprintf("Your account was debited %.2f %s", ledger.Amount, ledger. Currency)
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
				return fmt.Errorf("insufficient balance in %s: available=%.2f, required=%.2f",
    				entry.AccountNumber, balance.AvailableBalance, entry.Amount)  // ✅ Use %.2f for float64
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
	Amount        float64   `json:"amount,omitempty"`
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
	fmt.Printf("[TRANSACTION START] Receipt: %s | Type: %s | Amount: %.2f %s | Accounts: %d\n",
		receiptCode, req.TransactionType, uc.getTotalAmount(req.Entries), req.GetCurrency(), len(req. Entries))
		// ✅ Use %.2f
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

func (uc *TransactionUsecase) getTotalAmount(entries []*domain.LedgerEntryRequest) float64 {
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

	// Fetch accounts
	systemAccount, userAccount, err := uc.fetchSystemAndUserAccounts(
		ctx, req.Currency, domain.PurposeLiquidity, req.AccountNumber,
	)
	if err != nil {
		return nil, err
	}
	if systemAccount.Currency != userAccount.Currency {
		return nil, xerrors.ErrCurrencyMismatch
	}

	// Build transaction request
	txReq := buildCreditDoubleEntry(req, systemAccount, userAccount)

	// Execute with common pattern
	return uc.executeWithReceipt(
		ctx,
		txReq,
		func(ctx context.Context) (*domain.LedgerAggregate, error) {
			req.ExternalRef = txReq.ExternalRef
			req.ReceiptCode = txReq.ReceiptCode
			return uc.transactionRepo.Credit(ctx, req)
		},
		func(agg *domain.LedgerAggregate) {
			uc.publishCreditEvent(agg, req)
		},
	)
}

func (uc *TransactionUsecase) publishCreditEvent(aggregate *domain.LedgerAggregate, req *domain.CreditRequest) {
	if uc.eventPublisher == nil || aggregate == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find the credited ledger
	var balanceAfter float64
	for _, ledger := range aggregate.Ledgers {
		if ledger.DrCr == domain.DrCrCredit {
			balanceAfter = ptrFloat64ToFloat64(ledger.BalanceAfter)
			break
		}
	}

	err := uc.eventPublisher.PublishDepositCompleted(
		ctx,
		req.CreatedByExternalID,
		ptrStrToStr(aggregate.Journal.ExternalRef),
		req.AccountNumber,
		req.Amount,
		req.Currency,
		balanceAfter,
	)

	if err != nil {
		fmt.Printf("[ERROR] Failed to publish credit event: %v\n", err)
	}
}

// Debit removes money from account (user → system, NO FEES)
func (uc *TransactionUsecase) Debit(
	ctx context.Context,
	req *domain.DebitRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid debit request: %w", err)
	}

	// Fetch accounts
	systemAccount, userAccount, err := uc.fetchSystemAndUserAccounts(
		ctx, req.Currency, domain.PurposeLiquidity, req.AccountNumber,
	)
	if err != nil {
		return nil, err
	}
	if systemAccount.Currency != userAccount.Currency {
		return nil, xerrors.ErrCurrencyMismatch
	}

	// Build transaction request
	txReq := buildDebitDoubleEntry(req, systemAccount, userAccount)

	// Execute with common pattern
	return uc.executeWithReceipt(
		ctx,
		txReq,
		func(ctx context.Context) (*domain.LedgerAggregate, error) {
			req.ExternalRef = txReq.ExternalRef
			req.ReceiptCode = txReq.ReceiptCode

			return uc.transactionRepo.Debit(ctx, req)
		},
		func(agg *domain.LedgerAggregate) {
			uc.publishDebitEvent(agg, req)
		},
	)
}

func (uc *TransactionUsecase) publishDebitEvent(aggregate *domain.LedgerAggregate, req *domain.DebitRequest) {
	if uc.eventPublisher == nil || aggregate == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find the debited ledger
	var balanceAfter float64
	for _, ledger := range aggregate.Ledgers {
		if ledger.DrCr == domain.DrCrDebit {
			balanceAfter = ptrFloat64ToFloat64(ledger.BalanceAfter)
			break
		}
	}

	err := uc.eventPublisher.PublishWithdrawalCompleted(
		ctx,
		req.CreatedByExternalID,
		ptrStrToStr(aggregate.Journal.ExternalRef),
		req.AccountNumber,
		req.Amount,
		req.Currency,
		balanceAfter,
	)

	if err != nil {
		fmt.Printf("[ERROR] Failed to publish debit event: %v\n", err)
	}
}

func (uc *TransactionUsecase) Transfer(
	ctx context.Context,
	req *domain.TransferRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transfer request: %w", err)
	}

	// Fetch accounts
	sourceAccount, destAccount, err := uc.fetchTransferAccounts(ctx, req.FromAccountNumber, req.ToAccountNumber)
	if err != nil {
		return nil, err
	}

	// Validate same currency
	if sourceAccount.Currency != destAccount.Currency {
		return nil, xerrors.ErrCurrencyMismatch
	}

	// Build transaction request
	txReq := buildTransferDoubleEntry(req, sourceAccount, destAccount)
	if txReq == nil {
		return nil, xerrors.ErrCurrencyMismatch
	}

	// Execute with common pattern
	return uc.executeWithReceipt(
		ctx,
		txReq,
		func(ctx context.Context) (*domain.LedgerAggregate, error) {
			req.ExternalRef = txReq.ExternalRef
			req.ReceiptCode = txReq.ReceiptCode
			return uc.transactionRepo.Transfer(ctx, req)
		},
		func(agg *domain.LedgerAggregate) {
			uc.publishTransferEvent(agg, req)
		},
	)
}

func (uc *TransactionUsecase) publishTransferEvent(aggregate *domain.LedgerAggregate, req *domain.TransferRequest) {
	if uc.eventPublisher == nil || aggregate == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calculate fee (if any)
	var feeAmount float64 = 0
	// You can fetch fee from aggregate. Fees if available

	err := uc.eventPublisher.PublishTransferCompleted(
		ctx,
		req.CreatedByExternalID,
		ptrStrToStr(aggregate.Journal.ExternalRef),
		req.FromAccountNumber,
		req.ToAccountNumber,
		req.Amount,
		"", // Currency - you may need to get this from accounts
		feeAmount,
	)

	if err != nil {
		fmt.Printf("[ERROR] Failed to publish transfer event: %v\n", err)
	}
}

// ConvertAndTransfer performs currency conversion (FEES APPLY)
func (uc *TransactionUsecase) ConvertAndTransfer(
	ctx context.Context,
	req *domain.ConversionRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid conversion request: %w", err)
	}

	// Fetch accounts
	sourceAccount, destAccount, err := uc.fetchTransferAccounts(ctx, req.FromAccountNumber, req.ToAccountNumber)
	if err != nil {
		return nil, err
	}

	// Currencies must be different
	if sourceAccount.Currency == destAccount.Currency {
		return nil, errors.New("use Transfer for same currency operations")
	}

	// Build transaction request
	txReq := buildConversionDoubleEntry(req, sourceAccount, destAccount)

	// Execute with common pattern
	return uc.executeWithReceipt(
		ctx,
		txReq,
		func(ctx context.Context) (*domain.LedgerAggregate, error) {
			req.ExternalRef = txReq.ExternalRef
			req.ReceiptCode = txReq.ReceiptCode
			return uc.transactionRepo.ConvertAndTransfer(ctx, req)
		},
		func(agg *domain.LedgerAggregate) {
			uc.publishConversionEvent(agg, req)
		},
	)
}

func (uc *TransactionUsecase) publishConversionEvent(aggregate *domain.LedgerAggregate, req *domain.ConversionRequest) {
	if uc.eventPublisher == nil || aggregate == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get currencies from ledgers
	var sourceCurrency, destCurrency string
	var sourceAmount, convertedAmount float64

	for _, ledger := range aggregate.Ledgers {
		if ledger.AccountData.AccountNumber == req.FromAccountNumber {
			sourceCurrency = ledger.Currency
			sourceAmount = ledger.Amount
		}
		if ledger.AccountData.AccountNumber == req.ToAccountNumber {
			destCurrency = ledger.Currency
			convertedAmount = ledger.Amount
		}
	}

	// Publish as a custom event
	event := &publisher.TransactionEvent{
		EventType:       "conversion.completed",
		UserID:          req.CreatedByExternalID,
		ReceiptCode:     ptrStrToStr(aggregate.Journal.ExternalRef),
		TransactionID:   aggregate.Journal.ID,
		TransactionType: "conversion",
		Status:          "completed",
		Amount:          sourceAmount,
		Currency:        sourceCurrency,
		FromAccount:     req.FromAccountNumber,
		ToAccount:       req.ToAccountNumber,
		Metadata: map[string]interface{}{
			"source_currency":  sourceCurrency,
			"dest_currency":    destCurrency,
			"source_amount":    sourceAmount,
			"converted_amount": convertedAmount,
		},
	}

	if err := uc.eventPublisher.PublishTransactionEvent(ctx, event); err != nil {
		fmt.Printf("[ERROR] Failed to publish conversion event: %v\n", err)
	}
}

// ProcessTradeWin credits account for trade win (NO FEES)
func (uc *TransactionUsecase) ProcessTradeWin(
	ctx context.Context,
	req *domain.TradeRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid trade request: %w", err)
	}

	// Fetch accounts
	systemAccount, userAccount, err := uc.fetchSystemAndUserAccounts(
		ctx, req.Currency, domain.PurposeLiquidity, req.AccountNumber,
	)
	if err != nil {
		return nil, err
	}

	// Build transaction request
	txReq := buildTradeDoubleEntry(req, systemAccount, userAccount, "win")

	// Execute with common pattern
	aggregate, err := uc.executeWithReceipt(
		ctx,
		txReq,
		func(ctx context.Context) (*domain.LedgerAggregate, error) {
			req.ReceiptCode = txReq.ReceiptCode
			return uc.transactionRepo.ProcessTradeWin(ctx, req)
		},
		func(agg *domain.LedgerAggregate) {
			uc.publishTradeEvent(agg, req, "win")
		},
	)

	if err == nil && aggregate != nil {
		// Override external_ref with receipt code
		aggregate.Journal.ExternalRef = txReq.ExternalRef
	}

	return aggregate, err
}

func (uc *TransactionUsecase) ProcessTradeLoss(
	ctx context.Context,
	req *domain.TradeRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid trade request: %w", err)
	}

	// Fetch accounts
	systemAccount, userAccount, err := uc.fetchSystemAndUserAccounts(
		ctx, req.Currency, domain.PurposeLiquidity, req.AccountNumber,
	)
	if err != nil {
		return nil, err
	}

	// Build transaction request
	txReq := buildTradeDoubleEntry(req, systemAccount, userAccount, "loss")

	// Execute with common pattern
	aggregate, err := uc.executeWithReceipt(
		ctx,
		txReq,
		func(ctx context.Context) (*domain.LedgerAggregate, error) {
			req.ReceiptCode = txReq.ReceiptCode
			return uc.transactionRepo.ProcessTradeLoss(ctx, req)
		},
		func(agg *domain.LedgerAggregate) {
			uc.publishTradeEvent(agg, req, "loss")
		},
	)

	if err == nil && aggregate != nil {
		// Override external_ref with receipt code
		aggregate.Journal.ExternalRef = txReq.ExternalRef
	}

	return aggregate, err
}

func (uc *TransactionUsecase) publishTradeEvent(aggregate *domain.LedgerAggregate, req *domain.TradeRequest, result string) {
	if uc.eventPublisher == nil || aggregate == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var balanceAfter float64
	for _, ledger := range aggregate.Ledgers {
		balanceAfter = ptrFloat64ToFloat64(ledger.BalanceAfter)
		break
	}

	event := &publisher.TransactionEvent{
		EventType:       fmt.Sprintf("trade.%s", result),
		UserID:          req.CreatedByExternalID,
		ReceiptCode:     ptrStrToStr(aggregate.Journal.ExternalRef),
		TransactionID:   aggregate.Journal.ID,
		TransactionType: "trade",
		Status:          "completed",
		Amount:          req.Amount,
		Currency:        req.Currency,
		AccountNumber:   req.AccountNumber,
		BalanceAfter:    balanceAfter,
		Metadata: map[string]interface{}{
			"trade_id":     req.TradeID,
			"trade_type":   req.TradeType,
			"trade_result": result,
		},
	}

	if err := uc.eventPublisher.PublishTransactionEvent(ctx, event); err != nil {
		fmt.Printf("[ERROR] Failed to publish trade event: %v\n", err)
	}
}

// ProcessAgentCommission pays commission to agent (NO FEES)
func (uc *TransactionUsecase) ProcessAgentCommission(
	ctx context.Context,
	req *domain.AgentCommissionRequest,
) (*domain.LedgerAggregate, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid commission request: %w", err)
	}

	// Get agent commission account
	agentAccount, err := uc.accountUC.GetAgentAccount(ctx, req.AgentExternalID, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent account: %w", err)
	}

	// Get system fee account
	systemFeeAccount, err := uc.accountUC.GetSystemAccount(ctx, req.Currency, domain.PurposeFees)
	if err != nil {
		return nil, fmt.Errorf("failed to get system fee account: %w", err)
	}

	// Build transaction request
	txReq := buildCommissionDoubleEntry(req, systemFeeAccount, agentAccount)

	// Execute with common pattern
	return uc.executeWithReceipt(
		ctx,
		txReq,
		func(ctx context.Context) (*domain.LedgerAggregate, error) {
			req.ReceiptCode = txReq.ReceiptCode
			return uc.transactionRepo.ProcessAgentCommission(ctx, req)
		},
		nil, // No specific event publisher for commission yet
	)
}

// ===============================
// HELPER: RECEIPT GENERATION
// ===============================

// generateReceiptCode generates a receipt code using the batch system
func (uc *TransactionUsecase) generateReceiptCode(
	ctx context.Context,
	req *domain.TransactionRequest,
) (string, error) {
	receiptCodeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	uc.receiptBatcher.Add(&ReceiptRequest{
		TxnReq:     req,
		ResultChan: receiptCodeChan,
		ErrorChan:  errChan,
	})

	select {
	case receiptCode := <-receiptCodeChan:
		return receiptCode, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(2 * time.Second):
		return "", errors.New("receipt generation timeout")
	}
}

package usecase

import (
	"accounting-service/internal/domain"
	"context"
	"fmt"
	"sync"
	"time"

	receiptclient "x/shared/common/receipt"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ===============================
// RECEIPT BATCHER
// ===============================

type ReceiptRequest struct {
	TxnReq     *domain.TransactionRequest
	ResultChan chan string
	ErrorChan  chan error
}

type ReceiptUpdateRequest struct {
	Code         string
	Status       receiptpb.TransactionStatus
	ErrorMessage string
}

type ReceiptBatcher struct {
	uc             *TransactionUsecase // Reference to usecase for account lookups
	client         *receiptclient.ReceiptClientV3
	createBatch    []*ReceiptRequest
	updateBatch    []*ReceiptUpdateRequest
	batchSize      int
	flushInterval  time.Duration
	mu             sync.Mutex
	stopChan       chan struct{}
}

func NewReceiptBatcher(uc *TransactionUsecase, client *receiptclient.ReceiptClientV3, batchSize int, interval time.Duration) *ReceiptBatcher {
	return &ReceiptBatcher{
		uc:            uc,
		client:        client,
		batchSize:     batchSize,
		flushInterval: interval,
		stopChan:      make(chan struct{}),
	}
}

func (rb *ReceiptBatcher) Start() {
	go rb.createWorker()
	go rb.updateWorker()
}

func (rb *ReceiptBatcher) Stop() {
	close(rb.stopChan)
}

func (rb *ReceiptBatcher) Add(req *ReceiptRequest) {
	rb.mu.Lock()
	rb.createBatch = append(rb.createBatch, req)
	shouldFlush := len(rb.createBatch) >= rb.batchSize
	rb.mu.Unlock()

	if shouldFlush {
		rb.flushCreate()
	}
}

func (rb *ReceiptBatcher) UpdateStatus(code string, status receiptpb.TransactionStatus, errorMsg string) {
	rb.mu.Lock()
	rb.updateBatch = append(rb.updateBatch, &ReceiptUpdateRequest{
		Code:         code,
		Status:       status,
		ErrorMessage: errorMsg,
	})
	shouldFlush := len(rb.updateBatch) >= rb.batchSize
	rb.mu.Unlock()

	if shouldFlush {
		rb.flushUpdate()
	}
}

func (rb *ReceiptBatcher) createWorker() {
	ticker := time.NewTicker(rb.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rb.flushCreate()
		case <-rb.stopChan:
			return
		}
	}
}

func (rb *ReceiptBatcher) updateWorker() {
	ticker := time.NewTicker(rb.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rb.flushUpdate()
		case <-rb.stopChan:
			return
		}
	}
}

func (rb *ReceiptBatcher) flushCreate() {
	rb.mu.Lock()
	if len(rb.createBatch) == 0 {
		rb.mu.Unlock()
		return
	}
	batch := rb.createBatch
	rb.createBatch = nil
	rb.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build batch request
	grpcReqs := make([]*receiptpb.CreateReceiptRequest, 0, len(batch))
	validRequests := make([]*ReceiptRequest, 0, len(batch))

	for _, req := range batch {
		grpcReq, err := rb.buildReceiptRequest(ctx, req.TxnReq)
		if err != nil {
			req.ErrorChan <- fmt.Errorf("failed to build receipt request: %w", err)
			continue
		}
		
		grpcReqs = append(grpcReqs, grpcReq)
		validRequests = append(validRequests, req)
	}

	if len(grpcReqs) == 0 {
		return
	}

	// Call batch API
	resp, err := rb.client.Client.CreateReceiptsBatch(ctx, &receiptpb.CreateReceiptsBatchRequest{
		Receipts:         grpcReqs,
		FailOnFirstError: false,
	})

	if err != nil {
		// All failed
		for _, req := range validRequests {
			req.ErrorChan <- err
		}
		fmt.Printf("[RECEIPT BATCHER] Failed to create batch: %v\n", err)
		return
	}

	// Match responses
	for i, receipt := range resp.Receipts {
		if i < len(validRequests) {
			validRequests[i].ResultChan <- receipt.Code
		}
	}

	// Handle errors
	for _, receiptErr := range resp.Errors {
		if int(receiptErr.Index) < len(validRequests) {
			validRequests[receiptErr.Index].ErrorChan <- fmt.Errorf("%s: %s", receiptErr.ErrorCode, receiptErr.ErrorMessage)
		}
	}

	fmt.Printf("[RECEIPT BATCHER] Created %d receipts\n", len(resp.Receipts))
}

func (rb *ReceiptBatcher) flushUpdate() {
	rb.mu.Lock()
	if len(rb.updateBatch) == 0 {
		rb.mu.Unlock()
		return
	}
	batch := rb.updateBatch
	rb.updateBatch = nil
	rb.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build update requests
	updates := make([]*receiptpb.UpdateReceiptRequest, len(batch))
	for i, req := range batch {
		updates[i] = &receiptpb.UpdateReceiptRequest{
			Code:         req.Code,
			Status:       req.Status,
			ErrorMessage: req.ErrorMessage,
		}
		if req.Status == receiptpb.TransactionStatus_TRANSACTION_STATUS_COMPLETED {
			now := time.Now()
			updates[i].CompletedAt = timestamppb.New(now)
		}
	}

	// Call batch update
	_, err := rb.client.Client.UpdateReceiptsBatch(ctx, &receiptpb.UpdateReceiptsBatchRequest{
		Updates: updates,
	})

	if err != nil {
		fmt.Printf("[RECEIPT BATCHER] Failed to update batch: %v\n", err)
	} else {
		fmt.Printf("[RECEIPT BATCHER] Updated %d receipts\n", len(batch))
	}
}

func (rb *ReceiptBatcher) buildReceiptRequest(ctx context.Context, req *domain.TransactionRequest) (*receiptpb.CreateReceiptRequest, error) {
	// Extract creditor/debitor from entries
	var creditor, debitor *receiptpb.PartyInfo
	var amount int64
	var currency string

	for _, entry := range req.Entries {
		// Fetch account info
		account, err := rb.uc.accountRepo.GetByAccountNumber(ctx, entry.AccountNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get account %s: %w", entry.AccountNumber, err)
		}

		party := &receiptpb.PartyInfo{
			AccountId:     account.ID,
			AccountNumber: account.AccountNumber,
			OwnerType:     rb.convertOwnerType(account.OwnerType),
			ExternalId:    account.OwnerID,
			Status:        receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING,
		}

		if entry.DrCr == domain.DrCrCredit {
			party.IsCreditor = true
			creditor = party
		} else {
			party.IsCreditor = false
			debitor = party
		}

		// Use debit amount as transaction amount
		if entry.DrCr == domain.DrCrDebit {
			amount = entry.Amount
			currency = entry.Currency
		}
	}

	// Validate we have both parties
	if creditor == nil || debitor == nil {
		return nil, fmt.Errorf("missing creditor or debitor in transaction")
	}

	return &receiptpb.CreateReceiptRequest{
		TransactionType: rb.convertTransactionType(req.TransactionType),
		Amount:          amount,
		Currency:        currency,
		AccountType:     rb.convertAccountType(req.AccountType),
		Creditor:        creditor,
		Debitor:         debitor,
		CreatedBy:       "system",
		ExternalRef:     ptrStrToStr(req.ExternalRef), // Can be used for additional reference
	}, nil
}

func (rb *ReceiptBatcher) convertTransactionType(t domain.TransactionType) receiptpb.TransactionType {
	switch t {
	case domain.TransactionTypeDeposit:
		return receiptpb.TransactionType_TRANSACTION_TYPE_DEPOSIT
	case domain.TransactionTypeWithdrawal:
		return receiptpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL
	case domain.TransactionTypeTransfer:
		return receiptpb.TransactionType_TRANSACTION_TYPE_TRANSFER
	case domain.TransactionTypeConversion:
		return receiptpb.TransactionType_TRANSACTION_TYPE_CONVERSION
	case domain.TransactionTypeFee:
		return receiptpb.TransactionType_TRANSACTION_TYPE_FEE
	case domain.TransactionTypeCommission:
		return receiptpb.TransactionType_TRANSACTION_TYPE_COMMISSION
	case domain.TransactionTypeTrade:
		return receiptpb.TransactionType_TRANSACTION_TYPE_TRADE
	case domain.TransactionTypeAdjustment:
		return receiptpb.TransactionType_TRANSACTION_TYPE_ADJUSTMENT
	case domain.TransactionTypeRefund:
		return receiptpb.TransactionType_TRANSACTION_TYPE_REFUND
	case domain.TransactionTypeReversal:
		return receiptpb.TransactionType_TRANSACTION_TYPE_REVERSAL
	default:
		return receiptpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func (rb *ReceiptBatcher) convertAccountType(t domain.AccountType) receiptpb.AccountType {
	if t == domain.AccountTypeReal {
		return receiptpb.AccountType_ACCOUNT_TYPE_REAL
	}
	return receiptpb.AccountType_ACCOUNT_TYPE_DEMO
}

func (rb *ReceiptBatcher) convertOwnerType(t domain.OwnerType) receiptpb.OwnerType {
	switch t {
	case domain.OwnerTypeSystem:
		return receiptpb.OwnerType_OWNER_TYPE_SYSTEM
	case domain.OwnerTypeUser:
		return receiptpb.OwnerType_OWNER_TYPE_USER
	case domain.OwnerTypeAgent:
		return receiptpb.OwnerType_OWNER_TYPE_AGENT
	case domain.OwnerTypePartner:
		return receiptpb.OwnerType_OWNER_TYPE_PARTNER
	default:
		return receiptpb.OwnerType_OWNER_TYPE_UNSPECIFIED
	}
}
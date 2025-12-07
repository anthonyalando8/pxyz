// usecase/partner_transactions.go
package usecase

import (
	"context"

	"errors"
	"fmt"
	"partner-service/internal/domain"
	"time"

	"go.uber.org/zap"
)

func (uc *PartnerUsecase) GetTransactionByRef(ctx context.Context, partnerID, transactionRef string) (*domain.PartnerTransaction, error) {

	if partnerID == "" {
		return nil, errors.New("partner_id is required")
	}
	if transactionRef == "" {
		return nil, errors.New("transaction_ref is required")
	}
	return uc.partnerRepo.GetTransactionByRef(ctx, partnerID, transactionRef)
}

func (uc *PartnerUsecase) GetTransactionByID(ctx context.Context, transactionID int64) (*domain.PartnerTransaction, error) {
	if transactionID <= 0 {
		return nil, errors.New("invalid transaction_id")
	}
	return uc.partnerRepo.GetTransactionByID(ctx, transactionID)
}

// InitiateDeposit creates a new deposit transaction
func (uc *PartnerUsecase) InitiateDeposit(ctx context.Context, req *domain.PartnerTransaction) error {
	// Validation
	if err := uc.validateDepositRequest(req); err != nil {
		return err
	}

	// Check if transaction ref already exists (idempotency)
	existing, _ := uc.partnerRepo.GetTransactionByRef(ctx, req.PartnerID, req.TransactionRef)
	if existing != nil {
		uc.logger.Warn("duplicate transaction reference",
			zap.String("partner_id", req.PartnerID),
			zap. String("transaction_ref", req. TransactionRef))
		return fmt.Errorf("transaction with ref %s already exists", req.TransactionRef)
	}

	// Verify partner exists and is active
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, req. PartnerID)
	if err != nil {
		return fmt. Errorf("partner not found: %w", err)
	}
	if partner.Status != domain.PartnerStatusActive {
		return errors.New("partner is not active")
	}

	// Set initial status
	req. Status = "pending"
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	// Create transaction record
	if err := uc. partnerRepo.CreateTransaction(ctx, req); err != nil {
		uc.logger.Error("failed to create transaction",
			zap.String("partner_id", req.PartnerID),
			zap.Error(err))
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	uc.logger.Info("deposit initiated",
		zap.String("partner_id", req. PartnerID),
		zap. String("transaction_ref", req. TransactionRef),
		zap.String("user_id", req.UserID),
		zap.Float64("amount", req. Amount),
		zap.String("currency", req.Currency))

	// ✅ Send webhook notification using existing SendWebhook method
	go uc.sendDepositInitiatedWebhook(req)

	return nil
}

// validateDepositRequest validates the deposit request
func (uc *PartnerUsecase) validateDepositRequest(req *domain.PartnerTransaction) error {
	if req.PartnerID == "" {
		return errors.New("partner_id is required")
	}
	if req.TransactionRef == "" {
		return errors.New("transaction_ref is required")
	}
	if req.UserID == "" {
		return errors.New("user_id is required")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	if req. Currency == "" {
		return errors.New("currency is required")
	}

	// Additional validations
	if len(req.Currency) < 3 || len(req.Currency) > 8 {
		return errors.New("invalid currency format")
	}

	return nil
}

// sendDepositInitiatedWebhook sends deposit. initiated webhook
func (uc *PartnerUsecase) sendDepositInitiatedWebhook(txn *domain.PartnerTransaction) {
	ctx := context.Background()

	// Build webhook payload
	payload := map[string]interface{}{
		"event":           "deposit.initiated",
		"transaction_ref": txn.TransactionRef,
		"partner_id":      txn.PartnerID,
		"user_id":         txn.UserID,
		"amount":          txn.Amount,
		"currency":        txn. Currency,
		"status":          txn.Status,
		"payment_method":  txn. PaymentMethod,
		"external_ref":    txn.ExternalRef,
		"metadata":        txn.Metadata,
		"created_at":      txn.CreatedAt.Unix(),
		"timestamp":       time. Now().Unix(),
	}

	// ✅ Use existing SendWebhook method
	if err := uc.SendWebhook(ctx, txn. PartnerID, "deposit.initiated", payload); err != nil {
		uc.logger.Error("failed to send deposit webhook",
			zap.String("partner_id", txn. PartnerID),
			zap.String("transaction_ref", txn. TransactionRef),
			zap.Error(err))
	}
}

// CreateTransaction creates a new partner transaction record
func (uc *PartnerUsecase) CreateTransaction(ctx context.Context, tx *domain.PartnerTransaction) error {
	return uc.partnerRepo.CreateTransaction(ctx, tx)
}

// UpdateTransactionStatus updates the status of a partner transaction
func (uc *PartnerUsecase) UpdateTransactionStatus(ctx context.Context, txID int64, status, errorMsg string) error {
	return uc.partnerRepo.UpdateTransactionStatus(ctx, txID, status, errorMsg)
}

func (uc *PartnerUsecase) UpdateTransactionWithReceipt(ctx context.Context, txID int64, receiptCode string, journalID int64, status string) error {
	return uc.partnerRepo.UpdateTransactionWithReceipt(ctx, txID, receiptCode, journalID, status)
}

// GetTransactionStatus retrieves transaction by reference
func (uc *PartnerUsecase) GetTransactionStatus(ctx context.Context, partnerID, transactionRef string) (*domain.PartnerTransaction, error) {
	if partnerID == "" || transactionRef == "" {
		return nil, errors.New("partner_id and transaction_ref are required")
	}

	return uc.partnerRepo.GetTransactionByRef(ctx, partnerID, transactionRef)
}

// ListTransactions returns paginated transactions for a partner
func (uc *PartnerUsecase) ListTransactions(ctx context.Context, partnerID string, limit, offset int, status *string, from, to *time.Time) ([]domain.PartnerTransaction, int64, error) {
	if partnerID == "" {
		return nil, 0, errors.New("partner_id is required")
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return uc.partnerRepo.ListTransactions(ctx, partnerID, limit, offset, status)
}

// ProcessDeposit handles the actual wallet funding (called by background worker)
func (uc *PartnerUsecase) ProcessDeposit(ctx context.Context, transactionID int64) error {
	// TODO: Implement
	// 1. Get transaction details
	// 2. Verify user exists
	// 3. Call accounting service to credit wallet
	// 4. Update transaction status to 'completed' or 'failed'
	// 5. Trigger webhook notification to partner
	return nil
}
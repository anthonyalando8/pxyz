// usecase/partner_transactions. go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"partner-service/internal/domain"
	"time"

	"go.uber.org/zap"
)

// Transaction types
const (
	TransactionTypeDeposit    = "deposit"
	TransactionTypeWithdrawal = "withdrawal"
)

// Transaction statuses
const (
	TransactionStatusPending    = "pending"
	TransactionStatusProcessing = "processing"
	TransactionStatusCompleted  = "completed"
	TransactionStatusFailed     = "failed"
	TransactionStatusCancelled  = "cancelled"
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

// ============================================================================
// DEPOSIT OPERATIONS
// ============================================================================

// InitiateDeposit creates a new deposit transaction
func (uc *PartnerUsecase) InitiateDeposit(ctx context.Context, req *domain.PartnerTransaction) error {
	// Validation
	if err := uc. validateTransactionRequest(req); err != nil {
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
		return fmt. Errorf("partner not found:  %w", err)
	}
	if partner.Status != domain.PartnerStatusActive {
		return errors.New("partner is not active")
	}

	// Set transaction details
	req.TransactionType = TransactionTypeDeposit
	req.Status = TransactionStatusPending
	req. CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	// Create transaction record
	if err := uc.partnerRepo.CreateTransaction(ctx, req); err != nil {
		uc.logger. Error("failed to create deposit transaction",
			zap.String("partner_id", req. PartnerID),
			zap.Error(err))
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	uc. logger.Info("deposit initiated",
		zap.String("partner_id", req.PartnerID),
		zap.String("transaction_ref", req.TransactionRef),
		zap.String("user_id", req.UserID),
		zap.Float64("amount", req.Amount),
		zap.String("currency", req.Currency))

	// Send webhook notification
	go uc.sendDepositInitiatedWebhook(req)

	return nil
}

// sendDepositInitiatedWebhook sends deposit. initiated webhook
func (uc *PartnerUsecase) sendDepositInitiatedWebhook(txn *domain. PartnerTransaction) {
	ctx := context.Background()

	// ✅ Extract metadata fields
	phoneNumber := extractStringFromMetadata(txn. Metadata, "phone_number")
	accountNumber := extractStringFromMetadata(txn.Metadata, "account_number")
	bankAccount := extractStringFromMetadata(txn.Metadata, "bank_account")

	// ✅ Log extracted values
	uc.logger.Info("preparing deposit webhook",
		zap.String("transaction_ref", txn.TransactionRef),
		zap.String("partner_id", txn.PartnerID),
		zap.String("phone_number", stringPtrToString(phoneNumber)),
		zap.String("account_number", stringPtrToString(accountNumber)),
		zap.String("bank_account", stringPtrToString(bankAccount)))

	// ✅ Build base payload
	payload := map[string]interface{}{
		"event":            "deposit.initiated",
		"transaction_ref":  txn.TransactionRef,
		"transaction_type": txn. TransactionType,
		"partner_id":       txn. PartnerID,
		"user_id":          txn.UserID,
		"amount":           txn.Amount,
		"currency":         txn.Currency,
		"status":           txn.Status,
		"payment_method":   txn.PaymentMethod,
		"provider":         txn.PaymentMethod,
		"external_ref":     txn.ExternalRef,
		"metadata":         txn. Metadata,
		"created_at":       txn.CreatedAt.Unix(),
		"timestamp":        time.Now().Unix(),
	}

	// ✅ Add optional fields if available
	if phoneNumber != nil {
		payload["phone_number"] = *phoneNumber
	}

	if accountNumber != nil {
		payload["account_number"] = *accountNumber
	} else if bankAccount != nil {
		// Use bank_account as account_number fallback
		payload["account_number"] = *bankAccount
	}

	if bankAccount != nil && accountNumber != nil && *bankAccount != *accountNumber {
		// Include bank_account separately if different from account_number
		payload["bank_account"] = *bankAccount
	}

	uc.logger.Info("sending deposit initiated webhook",
		zap. String("partner_id", txn.PartnerID),
		zap.String("transaction_ref", txn.TransactionRef))

	if err := uc. SendWebhook(ctx, txn.PartnerID, "deposit.initiated", payload); err != nil {
		uc.logger.Error("failed to send deposit webhook",
			zap.String("partner_id", txn. PartnerID),
			zap.String("transaction_ref", txn.TransactionRef),
			zap.Error(err))
	} else {
		uc.logger.Info("deposit webhook sent successfully",
			zap.String("partner_id", txn. PartnerID),
			zap.String("transaction_ref", txn.TransactionRef))
	}
}

// ✅ Helper function to extract string from metadata
func extractStringFromMetadata(metadata map[string]interface{}, key string) *string {
	if metadata == nil {
		return nil
	}

	if value, ok := metadata[key].(string); ok && value != "" {
		return &value
	}

	return nil
}

// ✅ Helper to convert string pointer to string for logging
func stringPtrToString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// ============================================================================
// WITHDRAWAL OPERATIONS
// ============================================================================

// InitiateWithdrawal creates a new withdrawal transaction
func (uc *PartnerUsecase) InitiateWithdrawal(ctx context.Context, req *domain.PartnerTransaction) error {
	// Validation
	if err := uc.validateTransactionRequest(req); err != nil {
		return err
	}

	// Additional validation for withdrawal
	if err := uc.validateWithdrawalRequest(req); err != nil {
		return err
	}

	// Check if transaction ref already exists (idempotency)
	existing, _ := uc.partnerRepo.GetTransactionByRef(ctx, req.PartnerID, req.TransactionRef)
	if existing != nil {
		uc.logger.Warn("duplicate transaction reference",
			zap.String("partner_id", req.PartnerID),
			zap.String("transaction_ref", req.TransactionRef))
		return fmt.Errorf("transaction with ref %s already exists", req.TransactionRef)
	}

	// Verify partner exists and is active
	partner, err := uc.partnerRepo. GetPartnerByID(ctx, req.PartnerID)
	if err != nil {
		return fmt.Errorf("partner not found: %w", err)
	}
	if partner. Status != domain.PartnerStatusActive {
		return errors.New("partner is not active")
	}

	// Check if partner has sufficient balance (if applicable)
	// TODO: Implement partner balance check if needed

	// Set transaction details
	req.TransactionType = TransactionTypeWithdrawal
	req.Status = TransactionStatusPending
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	// Create transaction record
	if err := uc.partnerRepo.CreateTransaction(ctx, req); err != nil {
		uc.logger.Error("failed to create withdrawal transaction",
			zap.String("partner_id", req.PartnerID),
			zap.Error(err))
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	uc.logger.Info("withdrawal initiated",
		zap.String("partner_id", req. PartnerID),
		zap.String("transaction_ref", req.TransactionRef),
		zap.String("user_id", req.UserID),
		zap.Float64("amount", req.Amount),
		zap.String("currency", req.Currency))

	// Send webhook notification
	go uc.sendWithdrawalInitiatedWebhook(req)

	return nil
}

// validateWithdrawalRequest validates withdrawal-specific requirements
func (uc *PartnerUsecase) validateWithdrawalRequest(req *domain. PartnerTransaction) error {
	// Payment method is required for withdrawal
	if req.PaymentMethod == nil || *req.PaymentMethod == "" {
		return errors.New("payment_method is required for withdrawal")
	}

	// Validate payment method format (e.g., phone number, bank account)
	// TODO: Add specific validation based on payment method type

	return nil
}

// sendWithdrawalInitiatedWebhook sends withdrawal.initiated webhook
func (uc *PartnerUsecase) sendWithdrawalInitiatedWebhook(txn *domain.PartnerTransaction) {
	ctx := context.Background()

	payload := map[string]interface{}{
		"event":            "withdrawal.initiated",
		"transaction_ref":   txn.TransactionRef,
		"transaction_type": txn.TransactionType,
		"partner_id":       txn.PartnerID,
		"user_id":          txn.UserID,
		"amount":           txn.Amount,
		"currency":         txn. Currency,
		"status":           txn.Status,
		"payment_method":   txn. PaymentMethod,
		"external_ref":     txn.ExternalRef,
		"metadata":          txn.Metadata,
		"created_at":       txn.CreatedAt.Unix(),
		"timestamp":        time. Now().Unix(),
	}

	if err := uc.SendWebhook(ctx, txn.PartnerID, "withdrawal.initiated", payload); err != nil {
		uc.logger. Error("failed to send withdrawal webhook",
			zap.String("partner_id", txn.PartnerID),
			zap.String("transaction_ref", txn.TransactionRef),
			zap.Error(err))
	}
}

// ============================================================================
// COMMON VALIDATION
// ============================================================================

// validateTransactionRequest validates common transaction requirements
func (uc *PartnerUsecase) validateTransactionRequest(req *domain. PartnerTransaction) error {
	if req.PartnerID == "" {
		return errors.New("partner_id is required")
	}
	if req.TransactionRef == "" {
		return errors. New("transaction_ref is required")
	}
	if req. UserID == "" {
		return errors.New("user_id is required")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	if req.Currency == "" {
		return errors. New("currency is required")
	}

	// Validate currency format
	if len(req.Currency) < 3 || len(req.Currency) > 8 {
		return errors.New("invalid currency format")
	}

	// Validate amount precision (max 2 decimal places)
	if req.Amount*100 != float64(int64(req.Amount*100)) {
		return errors.New("amount can have maximum 2 decimal places")
	}

	return nil
}

// ============================================================================
// TRANSACTION MANAGEMENT
// ============================================================================

// CreateTransaction creates a new partner transaction record
func (uc *PartnerUsecase) CreateTransaction(ctx context.Context, tx *domain. PartnerTransaction) error {
	return uc.partnerRepo.CreateTransaction(ctx, tx)
}

// UpdateTransactionStatus updates the status of a partner transaction
func (uc *PartnerUsecase) UpdateTransactionStatus(ctx context.Context, txID int64, status, errorMsg string) error {
	return uc.partnerRepo.UpdateTransactionStatus(ctx, txID, status, errorMsg)
}

// UpdateTransactionWithReceipt updates transaction with accounting receipt
func (uc *PartnerUsecase) UpdateTransactionWithReceipt(ctx context.Context, txID int64, receiptCode string, journalID int64, status string) error {
	return uc.partnerRepo.UpdateTransactionWithReceipt(ctx, txID, receiptCode, journalID, status)
}

// GetTransactionStatus retrieves transaction by reference
func (uc *PartnerUsecase) GetTransactionStatus(ctx context.Context, partnerID, transactionRef string) (*domain.PartnerTransaction, error) {
	if partnerID == "" || transactionRef == "" {
		return nil, errors.New("partner_id and transaction_ref are required")
	}

	return uc.partnerRepo. GetTransactionByRef(ctx, partnerID, transactionRef)
}

// ListTransactions returns paginated transactions for a partner
func (uc *PartnerUsecase) ListTransactions(ctx context.Context, partnerID string, limit, offset int, status *string, from, to *time. Time) ([]domain.PartnerTransaction, int64, error) {
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

// ListTransactionsByType returns transactions filtered by type
func (uc *PartnerUsecase) ListTransactionsByType(ctx context.Context, partnerID, transactionType string, limit, offset int) ([]domain.PartnerTransaction, int64, error) {
	if partnerID == "" {
		return nil, 0, errors.New("partner_id is required")
	}

	// Validate transaction type
	if transactionType != TransactionTypeDeposit && transactionType != TransactionTypeWithdrawal {
		return nil, 0, fmt.Errorf("invalid transaction_type: must be '%s' or '%s'", TransactionTypeDeposit, TransactionTypeWithdrawal)
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return uc.partnerRepo.GetTransactionsByType(ctx, partnerID, transactionType, limit, offset)
}

// ============================================================================
// TRANSACTION PROCESSING
// ============================================================================

// ProcessDeposit handles the actual wallet funding (called by background worker)
func (uc *PartnerUsecase) ProcessDeposit(ctx context.Context, transactionID int64) error {
	// Get transaction details
	txn, err := uc.partnerRepo.GetTransactionByID(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	// Validate transaction type
	if txn. TransactionType != TransactionTypeDeposit {
		return errors.New("transaction is not a deposit")
	}

	// Check if already processed
	if txn.Status == TransactionStatusCompleted {
		uc.logger. Warn("transaction already completed",
			zap.Int64("transaction_id", transactionID))
		return nil
	}

	// Update status to processing
	if err := uc.partnerRepo.UpdateTransactionStatus(ctx, transactionID, TransactionStatusProcessing, ""); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// TODO: Call accounting service to credit wallet
	// Example: 
	// receipt, err := uc.accountingClient.CreditAccount(ctx, txn.UserID, txn.Amount, txn.Currency)
	// if err != nil {
	//     uc.partnerRepo.UpdateTransactionStatus(ctx, transactionID, TransactionStatusFailed, err.Error())
	//     return err
	// }
	
	// Update transaction with receipt
	// uc.partnerRepo.UpdateTransactionWithReceipt(ctx, transactionID, receipt. Code, receipt.JournalID, TransactionStatusCompleted)

	return nil
}

// ProcessWithdrawal handles the actual wallet debit (called by background worker)
func (uc *PartnerUsecase) ProcessWithdrawal(ctx context.Context, transactionID int64) error {
	// Get transaction details
	txn, err := uc.partnerRepo.GetTransactionByID(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	// Validate transaction type
	if txn.TransactionType != TransactionTypeWithdrawal {
		return errors. New("transaction is not a withdrawal")
	}

	// Check if already processed
	if txn. Status == TransactionStatusCompleted {
		uc.logger. Warn("transaction already completed",
			zap.Int64("transaction_id", transactionID))
		return nil
	}

	// Update status to processing
	if err := uc.partnerRepo. UpdateTransactionStatus(ctx, transactionID, TransactionStatusProcessing, ""); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// TODO: Call accounting service to debit wallet
	// Example:
	// receipt, err := uc. accountingClient.DebitAccount(ctx, txn.UserID, txn.Amount, txn. Currency)
	// if err != nil {
	//     uc.partnerRepo.UpdateTransactionStatus(ctx, transactionID, TransactionStatusFailed, err. Error())
	//     return err
	// }
	
	// Update transaction with receipt
	// uc.partnerRepo.UpdateTransactionWithReceipt(ctx, transactionID, receipt.Code, receipt.JournalID, TransactionStatusCompleted)

	// TODO: Initiate external payment (M-Pesa, bank transfer, etc.)
	// This depends on the payment_method specified

	return nil
}

// CancelTransaction cancels a pending transaction
func (uc *PartnerUsecase) CancelTransaction(ctx context.Context, partnerID, transactionRef string) error {
	// Get transaction
	txn, err := uc.partnerRepo.GetTransactionByRef(ctx, partnerID, transactionRef)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	// Can only cancel pending transactions
	if txn.Status != TransactionStatusPending {
		return fmt.Errorf("cannot cancel transaction with status: %s", txn. Status)
	}

	// Update status to cancelled
	if err := uc.partnerRepo. UpdateTransactionStatus(ctx, txn.ID, TransactionStatusCancelled, "cancelled by partner"); err != nil {
		return fmt.Errorf("failed to cancel transaction: %w", err)
	}

	uc.logger.Info("transaction cancelled",
		zap.String("partner_id", partnerID),
		zap.String("transaction_ref", transactionRef),
		zap.String("transaction_type", txn.TransactionType))

	// Send webhook notification
	go uc. sendTransactionCancelledWebhook(txn)

	return nil
}

// sendTransactionCancelledWebhook sends transaction. cancelled webhook
func (uc *PartnerUsecase) sendTransactionCancelledWebhook(txn *domain.PartnerTransaction) {
	ctx := context.Background()

	eventType := fmt.Sprintf("%s.cancelled", txn.TransactionType)
	
	payload := map[string]interface{}{
		"event":            eventType,
		"transaction_ref":  txn.TransactionRef,
		"transaction_type":  txn.TransactionType,
		"partner_id":       txn.PartnerID,
		"user_id":          txn.UserID,
		"amount":           txn.Amount,
		"currency":         txn.Currency,
		"status":            TransactionStatusCancelled,
		"cancelled_at":     time.Now().Unix(),
		"timestamp":        time.Now().Unix(),
	}

	if err := uc.SendWebhook(ctx, txn.PartnerID, eventType, payload); err != nil {
		uc. logger.Error("failed to send cancellation webhook",
			zap. String("partner_id", txn.PartnerID),
			zap.String("transaction_ref", txn.TransactionRef),
			zap.Error(err))
	}
}

func (uc *PartnerUsecase) GetTransactionStats(ctx context.Context, partnerID string, from, to time.Time) (map[string]interface{}, error) {
	return uc.partnerRepo.GetTransactionStats(ctx, partnerID, from, to)
}
	
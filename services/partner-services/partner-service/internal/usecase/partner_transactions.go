// usecase/partner_transactions.go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"partner-service/internal/domain"
	"time"
)

// InitiateDeposit creates a new deposit transaction
func (uc *PartnerUsecase) InitiateDeposit(ctx context.Context, req *domain.PartnerTransaction) error {
	// Validation
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
	if req.Currency == "" {
		return errors.New("currency is required")
	}

	// Check if transaction ref already exists
	existing, _ := uc.partnerRepo.GetTransactionByRef(ctx, req.PartnerID, req.TransactionRef)
	if existing != nil {
		return fmt.Errorf("transaction with ref %s already exists", req.TransactionRef)
	}

	// Verify partner exists and is active
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, req.PartnerID)
	if err != nil {
		return fmt.Errorf("partner not found: %w", err)
	}
	if partner.Status != domain.PartnerStatusActive {
		return errors.New("partner is not active")
	}

	// Set initial status
	req.Status = "pending"
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	// Create transaction
	if err := uc.partnerRepo.CreateTransaction(ctx, req); err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// TODO: Async processing
	// 1. Validate user exists in auth service
	// 2. Call accounting service to credit wallet
	// 3. Update transaction status
	// 4. Send webhook notification to partner

	return nil
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
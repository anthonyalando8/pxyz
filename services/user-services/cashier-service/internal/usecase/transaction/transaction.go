package usecase

import (
    "context"
    "errors"
    "fmt"
    "time"
    
    "cashier-service/internal/domain"
    "cashier-service/internal/repository"
    
    "github.com/google/uuid"
)

var (
    ErrDepositNotFound    = errors.New("deposit request not found")
    ErrWithdrawalNotFound = errors.New("withdrawal request not found")
    ErrInvalidStatus      = errors.New("invalid status transition")
    ErrDepositExpired     = errors.New("deposit request has expired")
    ErrUnauthorized       = errors.New("unauthorized")
)

type UserUsecase struct {
    repo *repository.UserRepo
}

func NewUserUsecase(repo *repository.UserRepo) *UserUsecase {
    return &UserUsecase{repo: repo}
}

// ============================================================================
// DEPOSIT USE CASES
// ============================================================================

// InitiateDeposit - User initiates a deposit request
func (uc *UserUsecase) InitiateDeposit(ctx context.Context, userID int64, partnerID string, amount float64, currency string, service string, paymentMethod *string, expirationMinutes int) (*domain.DepositRequest, error) {
    // Validate
    if amount <= 0 {
        return nil, errors.New("amount must be positive")
    }
    if currency == "" {
        return nil, errors.New("currency is required")
    }
    if service == "" {
        return nil, errors.New("service is required")
    }
    
    // Generate unique reference
    requestRef := fmt.Sprintf("DEP-%d-%s", userID, uuid.New().String()[:8])
    
    req := &domain.DepositRequest{
        UserID:        userID,
        PartnerID:     partnerID,
        RequestRef:    requestRef,
        Amount:        amount,
        Currency:      currency,
        Service:       service,
        PaymentMethod: paymentMethod,
        Status:        domain.DepositStatusPending,
        ExpiresAt:     time.Now().Add(time.Duration(expirationMinutes) * time.Minute),
        Metadata:      make(map[string]interface{}),
    }
    
    if err := uc.repo.CreateDepositRequest(ctx, req); err != nil {
        return nil, fmt.Errorf("failed to create deposit request: %w", err)
    }
    
    return req, nil
}

// GetDepositDetails - Get deposit request details (with ownership check)
func (uc *UserUsecase) GetDepositDetails(ctx context.Context, requestRef string, userID int64) (*domain. DepositRequest, error) {
    deposit, err := uc.repo. GetDepositByRef(ctx, requestRef)
    if err != nil {
        return nil, err
    }
    if deposit == nil {
        return nil, ErrDepositNotFound
    }
    
    // Verify ownership
    if deposit.UserID != userID {
        return nil, ErrUnauthorized
    }
    
    return deposit, nil
}

// GetUserDepositHistory - Get user's deposit history with pagination
func (uc *UserUsecase) GetUserDepositHistory(ctx context.Context, userID int64, limit, offset int) ([]domain. DepositRequest, int64, error) {
    if limit <= 0 || limit > 100 {
        limit = 20 // default
    }
    if offset < 0 {
        offset = 0
    }
    
    return uc.repo.ListDeposits(ctx, userID, limit, offset)
}

// MarkDepositSentToPartner - Called when deposit request is sent to payment partner
func (uc *UserUsecase) MarkDepositSentToPartner(ctx context.Context, requestRef string, partnerRef string) error {
    deposit, err := uc.repo.GetDepositByRef(ctx, requestRef)
    if err != nil || deposit == nil {
        return ErrDepositNotFound
    }
    
    // Check if already processed
    if deposit.Status != domain.DepositStatusPending {
        return ErrInvalidStatus
    }
    
    return uc.repo.UpdateDepositWithPartnerRef(ctx, deposit. ID, partnerRef, domain.DepositStatusSentToPartner)
}

// MarkDepositProcessing - Called by partner webhook when payment is being processed
func (uc *UserUsecase) MarkDepositProcessing(ctx context.Context, partnerRef string) error {
    deposit, err := uc.repo.GetDepositByPartnerRef(ctx, partnerRef)
    if err != nil || deposit == nil {
        return ErrDepositNotFound
    }
    
    return uc.repo.UpdateDepositStatus(ctx, deposit.ID, domain.DepositStatusProcessing, nil)
}

// CompleteDeposit - Called by accounting service after successful credit
func (uc *UserUsecase) CompleteDeposit(ctx context.Context, requestRef string, receiptCode string, journalID int64) error {
    deposit, err := uc. repo.GetDepositByRef(ctx, requestRef)
    if err != nil || deposit == nil {
        return ErrDepositNotFound
    }
    
    // Check expiration
    if time.Now(). After(deposit.ExpiresAt) {
        return ErrDepositExpired
    }
    
    return uc.repo. MarkDepositCompleted(ctx, requestRef, receiptCode, journalID)
}

// FailDeposit - Called when deposit fails (from partner or accounting service)
func (uc *UserUsecase) FailDeposit(ctx context.Context, requestRef string, errorMsg string) error {
    deposit, err := uc.repo.GetDepositByRef(ctx, requestRef)
    if err != nil || deposit == nil {
        return ErrDepositNotFound
    }
    
    return uc.repo. MarkDepositFailed(ctx, requestRef, errorMsg)
}

// CancelDeposit - User cancels pending deposit
func (uc *UserUsecase) CancelDeposit(ctx context.Context, requestRef string, userID int64) error {
    deposit, err := uc. repo.GetDepositByRef(ctx, requestRef)
    if err != nil || deposit == nil {
        return ErrDepositNotFound
    }
    
    // Verify ownership
    if deposit.UserID != userID {
        return ErrUnauthorized
    }
    
    // Can only cancel pending or sent_to_partner deposits
    if deposit.Status != domain.DepositStatusPending && deposit.Status != domain.DepositStatusSentToPartner {
        return fmt.Errorf("cannot cancel deposit in status: %s", deposit.Status)
    }
    
    errMsg := "cancelled by user"
    return uc. repo.UpdateDepositStatus(ctx, deposit. ID, domain.DepositStatusCancelled, &errMsg)
}

// ============================================================================
// WITHDRAWAL USE CASES
// ============================================================================

// InitiateWithdrawal - User initiates a withdrawal request
func (uc *UserUsecase) InitiateWithdrawal(ctx context.Context, userID int64, amount float64, currency string, destination string, service *string, agentID *string) (*domain.WithdrawalRequest, error) {
    // Validate
    if amount <= 0 {
        return nil, errors.New("amount must be positive")
    }
    if currency == "" {
        return nil, errors.New("currency is required")
    }
    if destination == "" {
        return nil, errors. New("destination is required")
    }
    
    // Generate unique reference
    requestRef := fmt.Sprintf("WTH-%d-%s", userID, uuid.New().String()[:8])
    
    req := &domain.WithdrawalRequest{
        UserID:          userID,
        RequestRef:      requestRef,
        Amount:          amount,
        Currency:        currency,
        Destination:     destination,
        Service:         service,
        AgentExternalID: agentID,
        Status:          domain.WithdrawalStatusPending,
        Metadata:        make(map[string]interface{}),
    }
    
    if err := uc.repo.CreateWithdrawalRequest(ctx, req); err != nil {
        return nil, fmt.Errorf("failed to create withdrawal request: %w", err)
    }
    
    return req, nil
}

// GetWithdrawalDetails - Get withdrawal request details (with ownership check)
func (uc *UserUsecase) GetWithdrawalDetails(ctx context.Context, requestRef string, userID int64) (*domain.WithdrawalRequest, error) {
    withdrawal, err := uc.repo.GetWithdrawalByRef(ctx, requestRef)
    if err != nil {
        return nil, err
    }
    if withdrawal == nil {
        return nil, ErrWithdrawalNotFound
    }
    
    // Verify ownership
    if withdrawal.UserID != userID {
        return nil, ErrUnauthorized
    }
    
    return withdrawal, nil
}

// GetUserWithdrawalHistory - Get user's withdrawal history with pagination
func (uc *UserUsecase) GetUserWithdrawalHistory(ctx context.Context, userID int64, limit, offset int) ([]domain.WithdrawalRequest, int64, error) {
    if limit <= 0 || limit > 100 {
        limit = 20 // default
    }
    if offset < 0 {
        offset = 0
    }
    
    return uc.repo.ListWithdrawals(ctx, userID, limit, offset)
}

// MarkWithdrawalProcessing - Called when withdrawal is being processed
func (uc *UserUsecase) MarkWithdrawalProcessing(ctx context.Context, requestRef string) error {
    withdrawal, err := uc.repo.GetWithdrawalByRef(ctx, requestRef)
    if err != nil || withdrawal == nil {
        return ErrWithdrawalNotFound
    }
    
    if withdrawal.Status != domain.WithdrawalStatusPending {
        return ErrInvalidStatus
    }
    
    return uc.repo.UpdateWithdrawalStatus(ctx, withdrawal.ID, domain.WithdrawalStatusProcessing, nil)
}

// CompleteWithdrawal - Called by accounting service after successful debit
func (uc *UserUsecase) CompleteWithdrawal(ctx context.Context, requestRef string, receiptCode string, journalID int64) error {
    withdrawal, err := uc.repo.GetWithdrawalByRef(ctx, requestRef)
    if err != nil || withdrawal == nil {
        return ErrWithdrawalNotFound
    }
    
    return uc.repo.MarkWithdrawalCompleted(ctx, requestRef, receiptCode, journalID)
}

// FailWithdrawal - Called when withdrawal fails (from accounting service)
func (uc *UserUsecase) FailWithdrawal(ctx context.Context, requestRef string, errorMsg string) error {
    withdrawal, err := uc.repo.GetWithdrawalByRef(ctx, requestRef)
    if err != nil || withdrawal == nil {
        return ErrWithdrawalNotFound
    }
    
    return uc.repo.MarkWithdrawalFailed(ctx, requestRef, errorMsg)
}

// CancelWithdrawal - User or admin cancels pending withdrawal
func (uc *UserUsecase) CancelWithdrawal(ctx context.Context, requestRef string, userID int64) error {
    withdrawal, err := uc.repo.GetWithdrawalByRef(ctx, requestRef)
    if err != nil || withdrawal == nil {
        return ErrWithdrawalNotFound
    }
    
    // Verify ownership
    if withdrawal.UserID != userID {
        return ErrUnauthorized
    }
    
    // Can only cancel pending withdrawals
    if withdrawal.Status != domain.WithdrawalStatusPending {
        return fmt. Errorf("cannot cancel withdrawal in status: %s", withdrawal.Status)
    }
    
    errMsg := "cancelled by user"
    return uc.repo.UpdateWithdrawalStatus(ctx, withdrawal.ID, domain.WithdrawalStatusCancelled, &errMsg)
}

// ============================================================================
// ADMIN/SYSTEM METHODS (No ownership checks)
// ============================================================================

// GetDepositByRef - Admin/system get deposit without ownership check
func (uc *UserUsecase) GetDepositByRef(ctx context.Context, requestRef string) (*domain. DepositRequest, error) {
    return uc.repo.GetDepositByRef(ctx, requestRef)
}

// GetWithdrawalByRef - Admin/system get withdrawal without ownership check
func (uc *UserUsecase) GetWithdrawalByRef(ctx context.Context, requestRef string) (*domain.WithdrawalRequest, error) {
    return uc.repo.GetWithdrawalByRef(ctx, requestRef)
}
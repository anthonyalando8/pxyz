// usecase/transaction_approval_usecase. go
package usecase

import (
    "context"
    "fmt"

    "accounting-service/internal/domain"
    "accounting-service/internal/repository"
    xerrors "x/shared/utils/errors"
)

type TransactionApprovalUsecase struct {
    approvalRepo repository.TransactionApprovalRepository
    txUC         *TransactionUsecase
}

func NewTransactionApprovalUsecase(
    approvalRepo repository.TransactionApprovalRepository,
    txUC *TransactionUsecase,
) *TransactionApprovalUsecase {
    return &TransactionApprovalUsecase{
        approvalRepo:  approvalRepo,
        txUC:         txUC,
    }
}

func (uc *TransactionApprovalUsecase) CreateApproval(
    ctx context.Context,
    req *domain.CreateApprovalRequest,
) (*domain.TransactionApproval, error) {
    approval := &domain.TransactionApproval{
        RequestedBy:       req.RequestedBy,
        TransactionType:  req.TransactionType,
        AccountNumber:    req.AccountNumber,
        Amount:           req.Amount,
        Currency:         req. Currency,
        Description:      req.Description,
        ToAccountNumber:  req.ToAccountNumber,
        Status:           domain.ApprovalStatusPending,
    }

    // Store metadata as JSON
    if req.RequestMetadata != nil {
        // Already a map, will be marshaled by repo
    }

    if err := approval.Validate(); err != nil {
        return nil, err
    }

    if err := uc.approvalRepo.Create(ctx, approval); err != nil {
        return nil, fmt.Errorf("failed to create approval: %w", err)
    }

    return approval, nil
}

func (uc *TransactionApprovalUsecase) GetPendingApprovals(
    ctx context.Context,
    limit, offset int,
) ([]*domain.TransactionApproval, int64, error) {
    status := domain.ApprovalStatusPending
    filter := &domain.ApprovalFilter{
        Status:  &status,
        Limit:   limit,
        Offset:  offset,
    }

    return uc.approvalRepo.List(ctx, filter)
}

// usecase/transaction_approval_usecase. go

func (uc *TransactionApprovalUsecase) ApproveOrReject(
    ctx context.Context,
    req *domain.ApproveApprovalRequest,
) (*domain.TransactionApproval, string, error) {
    // Get approval
    approval, err := uc. approvalRepo.GetByID(ctx, req.RequestID)
    if err != nil {
        return nil, "", xerrors.ErrNotFound
    }

    // Check if already processed
    if approval.Status != domain. ApprovalStatusPending {
        return nil, "", fmt. Errorf("approval already processed with status: %s", approval.Status)
    }

    // Prevent self-approval
    if approval. RequestedBy == req.ApprovedBy {
        return nil, "", xerrors.ErrSelfApprovalNotAllowed
    }

    // Update status
    if req. Approved {
        // Approve
        if err := uc.approvalRepo.UpdateStatus(ctx, req.RequestID, domain.ApprovalStatusApproved, &req.ApprovedBy, nil); err != nil {
            return nil, "", err
        }

        // Execute transaction asynchronously
        go uc.executeApprovedTransaction(approval)

        // Refresh approval
        approval, _ = uc.approvalRepo. GetByID(ctx, req. RequestID)
        return approval, "", nil
    } else {
        // Reject
        if err := uc.approvalRepo. UpdateStatus(ctx, req.RequestID, domain.ApprovalStatusRejected, &req.ApprovedBy, req.Reason); err != nil {
            return nil, "", err
        }

        // Refresh approval
        approval, _ = uc.approvalRepo.GetByID(ctx, req.RequestID)
        return approval, "", nil
    }
}

func (uc *TransactionApprovalUsecase) executeApprovedTransaction(approval *domain.TransactionApproval) {
    ctx := context.Background()

    var aggregate *domain.LedgerAggregate
    var err error

    // Execute based on type
    switch approval.TransactionType {
    case domain.TransactionTypeDeposit:
        aggregate, err = uc.txUC. Credit(ctx, &domain.CreditRequest{
            AccountNumber:        approval.AccountNumber,
            Amount:              approval.Amount,
            Description:         ptrStrToStr(approval.Description),
            TransactionType:     domain.TransactionTypeDeposit,
			AccountType: domain.AccountTypeReal,
			CreatedByExternalID: fmt.Sprintf("%d",approval.RequestedBy),
			CreatedByType: domain.OwnerTypeAdmin,
        })

    case domain.TransactionTypeWithdrawal:
        aggregate, err = uc.txUC. Debit(ctx, &domain. DebitRequest{
            AccountNumber:       approval.AccountNumber,
            Amount:              approval.Amount,
            Description:         ptrStrToStr(approval.Description),
            TransactionType:     domain.TransactionTypeWithdrawal,
			AccountType: domain.AccountTypeReal,
			CreatedByExternalID: fmt.Sprintf("%d", approval.RequestedBy),
			CreatedByType: domain.OwnerTypeAdmin,
			//TransactionType: approval.TransactionTypeWithdrawal,
        })

    case domain.TransactionTypeTransfer:
        aggregate, err = uc.txUC.Transfer(ctx, &domain.TransferRequest{
            FromAccountNumber:    approval.AccountNumber,
            ToAccountNumber:     *approval.ToAccountNumber,
            Amount:              approval.Amount,
            Description:         ptrStrToStr(approval.Description),
            TransactionType:     domain.TransactionTypeTransfer,
			AccountType: domain.AccountTypeReal,
			CreatedByExternalID: fmt.Sprintf("%d", approval.RequestedBy),
			CreatedByType: domain.OwnerTypeAdmin,
        })

    case domain.TransactionTypeConversion:
        aggregate, err = uc.txUC.ConvertAndTransfer(ctx, &domain.ConversionRequest{
            FromAccountNumber:   approval.AccountNumber,
            ToAccountNumber:     *approval.ToAccountNumber,
            Amount:              approval.Amount,
			AccountType: domain.AccountTypeReal,
			CreatedByExternalID: fmt.Sprintf("%d", approval.RequestedBy),
			CreatedByType: domain.OwnerTypeAdmin,
        })
    }

    // Update approval with result
    if err != nil {
        _ = uc.approvalRepo. MarkFailed(ctx, approval.ID, err.Error())
    } else if aggregate != nil {
        //  Extract receipt code from ledgers (primary source)
        receiptCode := extractReceiptCodeFromAggregate(aggregate)
        _ = uc.approvalRepo. MarkExecuted(ctx, approval.ID, receiptCode)
    }
}

func extractReceiptCodeFromAggregate(aggregate *domain.LedgerAggregate) string {
    if aggregate == nil {
        return ""
    }

    // Try ledgers first (primary source)
    if len(aggregate.Ledgers) > 0 {
        for _, ledger := range aggregate. Ledgers {
            if ledger. ReceiptCode != nil && *ledger.ReceiptCode != "" {
                return *ledger.ReceiptCode
            }
        }
    }

    //  Fallback to journal external_ref (backward compatibility)
    if aggregate.Journal. ExternalRef != nil && *aggregate.Journal.ExternalRef != "" {
        return *aggregate.Journal.ExternalRef
    }

    return ""
}

func (uc *TransactionApprovalUsecase) GetApprovalHistory(
    ctx context.Context,
    filter *domain.ApprovalFilter,
) ([]*domain.TransactionApproval, int64, error) {
    return uc.approvalRepo.List(ctx, filter)
}
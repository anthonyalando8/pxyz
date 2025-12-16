// handler/grpc_transaction_approval. go
package hgrpc

import (
    "context"
    "time"

    "accounting-service/internal/domain"
    accountingpb "x/shared/genproto/shared/accounting/v1"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/timestamppb"
)

// ===============================
// CREATE TRANSACTION APPROVAL
// ===============================

func (h *AccountingHandler) CreateTransactionApproval(
    ctx context.Context,
    req *accountingpb.CreateTransactionApprovalRequest,
) (*accountingpb.CreateTransactionApprovalResponse, error) {
    // Validate
    if err := validateApprovalRequest(req); err != nil {
        return nil, err
    }

    // Convert to domain
    domainReq := &domain.CreateApprovalRequest{
        RequestedBy:      req.RequestedBy,
        TransactionType:  convertTransactionTypeToDomain(req.TransactionType),
        AccountNumber:    req.AccountNumber,
        Amount:          req.Amount,
        Currency:        req.Currency,
        Description:     ptrString(req.Description),
        ToAccountNumber: toStringPtr(req.ToAccountNumber),
        RequestMetadata: map[string]interface{}{
            "transaction_type": req.TransactionType. String(),
            "requested_at":     time.Now().Unix(),
        },
    }

    // Create approval
    approval, err := h.approvalUC.CreateApproval(ctx, domainReq)
    if err != nil {
        return nil, handleUsecaseError(err)
    }

    return &accountingpb.CreateTransactionApprovalResponse{
        Approval: convertApprovalToProto(approval),
        Message:  "Transaction approval request created successfully.  Awaiting super admin approval.",
    }, nil
}

// ===============================
// GET PENDING APPROVALS
// ===============================

func (h *AccountingHandler) GetPendingApprovals(
    ctx context. Context,
    req *accountingpb.GetPendingApprovalsRequest,
) (*accountingpb.GetPendingApprovalsResponse, error) {
    // Set defaults
    limit := int(req.GetLimit())
    if limit <= 0 || limit > 100 {
        limit = 50
    }

    offset := int(req.GetOffset())
    if offset < 0 {
        offset = 0
    }

    // Get pending approvals
    approvals, total, err := h.approvalUC.GetPendingApprovals(ctx, limit, offset)
    if err != nil {
        return nil, handleUsecaseError(err)
    }

    // Convert to proto
    protoApprovals := make([]*accountingpb.TransactionApproval, 0, len(approvals))
    for _, approval := range approvals {
        protoApprovals = append(protoApprovals, convertApprovalToProto(approval))
    }

    return &accountingpb.GetPendingApprovalsResponse{
        Approvals: protoApprovals,
        Total:      int32(total),
    }, nil
}

// ===============================
// APPROVE/REJECT TRANSACTION
// ===============================

func (h *AccountingHandler) ApproveTransaction(
    ctx context.Context,
    req *accountingpb.ApproveTransactionRequest,
) (*accountingpb.ApproveTransactionResponse, error) {
    // Validate
    if req.RequestId <= 0 {
        return nil, status.Error(codes.InvalidArgument, "request_id is required")
    }
    if req.ApprovedBy <= 0 {
        return nil, status.Error(codes.InvalidArgument, "approved_by is required")
    }
    if ! req.Approved && req. Reason == nil {
        return nil, status. Error(codes.InvalidArgument, "rejection reason is required when rejecting")
    }

    // Convert to domain
    domainReq := &domain.ApproveApprovalRequest{
        RequestID:  req.RequestId,
        ApprovedBy: req.ApprovedBy,
        Approved:    req.Approved,
        Reason:     toStringPtr(req.Reason),
    }

    // Approve or reject
    approval, receiptCode, err := h.approvalUC.ApproveOrReject(ctx, domainReq)
    if err != nil {
        return nil, handleUsecaseError(err)
    }

    // Build response message
    message := "Transaction rejected"
    if req.Approved {
        message = "Transaction approved and queued for execution"
        if receiptCode != "" {
            message = "Transaction approved and executed successfully"
        }
    }

    return &accountingpb.ApproveTransactionResponse{
        Approval:     convertApprovalToProto(approval),
        ReceiptCode: ptrString(receiptCode),
        Message:     message,
    }, nil
}

// ===============================
// GET APPROVAL HISTORY
// ===============================

func (h *AccountingHandler) GetApprovalHistory(
    ctx context.Context,
    req *accountingpb. GetApprovalHistoryRequest,
) (*accountingpb.GetApprovalHistoryResponse, error) {
    // Set defaults
    limit := int(req. Limit)
    if limit <= 0 || limit > 100 {
        limit = 50
    }

    offset := int(req.Offset)
    if offset < 0 {
        offset = 0
    }

    // Build filter
    filter := &domain.ApprovalFilter{
        Limit:  limit,
        Offset: offset,
    }

    // Add optional filters
    if req.RequestedBy != nil {
        filter.RequestedBy = req.RequestedBy
    }

    if req.Status != nil && *req.Status != accountingpb.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED {
        status := convertApprovalStatusToDomain(*req.Status)
        filter.Status = &status
    }

    if req.From != nil {
        fromTime := req.From.AsTime()
        filter.FromDate = &fromTime
    }

    if req.To != nil {
        toTime := req.To.AsTime()
        filter.ToDate = &toTime
    }

    // Get history
    approvals, total, err := h. approvalUC.GetApprovalHistory(ctx, filter)
    if err != nil {
        return nil, handleUsecaseError(err)
    }

    // Convert to proto
    protoApprovals := make([]*accountingpb.TransactionApproval, 0, len(approvals))
    for _, approval := range approvals {
        protoApprovals = append(protoApprovals, convertApprovalToProto(approval))
    }

    return &accountingpb.GetApprovalHistoryResponse{
        Approvals:  protoApprovals,
        Total:     int32(total),
    }, nil
}

// ===============================
// VALIDATION HELPERS
// ===============================

func validateApprovalRequest(req *accountingpb.CreateTransactionApprovalRequest) error {
    if req.RequestedBy <= 0 {
        return status.Error(codes.InvalidArgument, "requested_by is required")
    }

    if req.TransactionType == accountingpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED {
        return status.Error(codes.InvalidArgument, "transaction_type is required")
    }

    if req.AccountNumber == "" {
        return status. Error(codes.InvalidArgument, "account_number is required")
    }

    if req.Amount <= 0 {
        return status.Error(codes.InvalidArgument, "amount must be positive")
    }

    if req.Currency == "" {
        return status.Error(codes. InvalidArgument, "currency is required")
    }

    // Validate transfer/conversion specific fields
    if req.TransactionType == accountingpb.TransactionType_TRANSACTION_TYPE_TRANSFER ||
        req.TransactionType == accountingpb.TransactionType_TRANSACTION_TYPE_CONVERSION {
        if req.ToAccountNumber == nil || *req.ToAccountNumber == "" {
            return status.Error(codes.InvalidArgument, "to_account_number is required for transfers and conversions")
        }
    }

    return nil
}

// ===============================
// CONVERSION HELPERS
// ===============================

// convertApprovalToProto converts domain approval to protobuf
func convertApprovalToProto(approval *domain. TransactionApproval) *accountingpb.TransactionApproval {
    if approval == nil {
        return nil
    }

    proto := &accountingpb.TransactionApproval{
        Id:              approval.ID,
        RequestedBy:     approval.RequestedBy,
        TransactionType: convertTransactionTypeToProto(approval.TransactionType),
        AccountNumber:   approval.AccountNumber,
        Amount:          approval.Amount,
        Currency:        approval.Currency,
        Status:          convertApprovalStatusToProto(approval.Status),
        CreatedAt:       timestamppb.New(approval.CreatedAt),
        UpdatedAt:       timestamppb.New(approval.UpdatedAt),
    }

    // Optional fields
    if approval.Description != nil {
        proto.Description = *approval.Description
    }

    if approval.ToAccountNumber != nil {
        proto.ToAccountNumber = approval.ToAccountNumber
    }

    if approval.ApprovedBy != nil {
        proto.ApprovedBy = approval.ApprovedBy
    }

    if approval.RejectionReason != nil {
        proto.RejectionReason = approval.RejectionReason
    }

    if approval.ReceiptCode != nil {
        proto.ReceiptCode = approval.ReceiptCode
    }

    return proto
}

// convertApprovalStatusToProto converts domain status to protobuf
func convertApprovalStatusToProto(status domain.ApprovalStatus) accountingpb.ApprovalStatus {
    switch status {
    case domain.ApprovalStatusPending:
        return accountingpb.ApprovalStatus_APPROVAL_STATUS_PENDING
    case domain.ApprovalStatusApproved:
        return accountingpb.ApprovalStatus_APPROVAL_STATUS_APPROVED
    case domain.ApprovalStatusRejected:
        return accountingpb.ApprovalStatus_APPROVAL_STATUS_REJECTED
    case domain.ApprovalStatusExecuted:
        return accountingpb.ApprovalStatus_APPROVAL_STATUS_EXECUTED
    case domain.ApprovalStatusFailed:
        return accountingpb.ApprovalStatus_APPROVAL_STATUS_FAILED
    default:
        return accountingpb.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED
    }
}

// convertApprovalStatusToDomain converts protobuf status to domain
func convertApprovalStatusToDomain(status accountingpb.ApprovalStatus) domain.ApprovalStatus {
    switch status {
    case accountingpb.ApprovalStatus_APPROVAL_STATUS_PENDING:
        return domain.ApprovalStatusPending
    case accountingpb.ApprovalStatus_APPROVAL_STATUS_APPROVED:
        return domain.ApprovalStatusApproved
    case accountingpb.ApprovalStatus_APPROVAL_STATUS_REJECTED:
        return domain. ApprovalStatusRejected
    case accountingpb.ApprovalStatus_APPROVAL_STATUS_EXECUTED:
        return domain.ApprovalStatusExecuted
    case accountingpb.ApprovalStatus_APPROVAL_STATUS_FAILED:
        return domain.ApprovalStatusFailed
    default:
        return domain. ApprovalStatusPending
    }
}

// toStringPtr converts optional string to pointer
func toStringPtr(opt *string) *string {
    if opt == nil {
        return nil
    }
    return opt
}
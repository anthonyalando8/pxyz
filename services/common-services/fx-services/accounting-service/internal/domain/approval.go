// domain/transaction_approval.go
package domain

import (
    "encoding/json"
    "time"
	"x/shared/utils/errors"
)

type ApprovalStatus string

const (
    ApprovalStatusPending  ApprovalStatus = "pending"
    ApprovalStatusApproved ApprovalStatus = "approved"
    ApprovalStatusRejected ApprovalStatus = "rejected"
    ApprovalStatusExecuted ApprovalStatus = "executed"
    ApprovalStatusFailed   ApprovalStatus = "failed"
)

type TransactionApproval struct {
    ID                int64                  `json:"id" db:"id"`
    RequestedBy       int64                  `json:"requested_by" db:"requested_by"`
    TransactionType   TransactionType        `json:"transaction_type" db:"transaction_type"`
    AccountNumber     string                 `json:"account_number" db:"account_number"`
    Amount            float64                `json:"amount" db:"amount"`
    Currency          string                 `json:"currency" db:"currency"`
    Description       *string                `json:"description,omitempty" db:"description"`
    ToAccountNumber   *string                `json:"to_account_number,omitempty" db:"to_account_number"`
    
    Status            ApprovalStatus         `json:"status" db:"status"`
    ApprovedBy        *int64                 `json:"approved_by,omitempty" db:"approved_by"`
    RejectionReason   *string                `json:"rejection_reason,omitempty" db:"rejection_reason"`
    
    ReceiptCode       *string                `json:"receipt_code,omitempty" db:"receipt_code"`
    ErrorMessage      *string                `json:"error_message,omitempty" db:"error_message"`
    
    RequestMetadata   json.RawMessage        `json:"request_metadata,omitempty" db:"request_metadata"`
    
    CreatedAt         time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt         time.Time              `json:"updated_at" db:"updated_at"`
    ApprovedAt        *time.Time             `json:"approved_at,omitempty" db:"approved_at"`
    ExecutedAt        *time.Time             `json:"executed_at,omitempty" db:"executed_at"`
}

type CreateApprovalRequest struct {
    RequestedBy      int64
    TransactionType  TransactionType
    AccountNumber    string
    Amount           float64
    Currency         string
    Description      *string
    ToAccountNumber  *string
    RequestMetadata  map[string]interface{}
}

type ApproveApprovalRequest struct {
    RequestID  int64
    ApprovedBy int64
    Approved   bool
    Reason     *string
}

type ApprovalFilter struct {
    Status       *ApprovalStatus
    RequestedBy  *int64
    ApprovedBy   *int64
    FromDate     *time.Time
    ToDate       *time.Time
    Limit        int
    Offset       int
}

func (a *TransactionApproval) Validate() error {
    if a.RequestedBy <= 0 {
        return xerrors.ErrInvalidInput
    }
    if a.AccountNumber == "" {
        return xerrors.ErrInvalidAccountNumber
    }
    if a.Amount <= 0 {
        return xerrors.ErrInvalidAmount
    }
    if a.Currency == "" {
        return xerrors.ErrInvalidCurrency
    }
    if a.TransactionType == TransactionTypeTransfer || a.TransactionType == TransactionTypeConversion {
        if a.ToAccountNumber == nil || *a.ToAccountNumber == "" {
            return xerrors.ErrRequiredFieldMissing
        }
    }
    return nil
}
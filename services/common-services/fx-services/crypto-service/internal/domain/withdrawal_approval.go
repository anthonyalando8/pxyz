// internal/domain/withdrawal_approval.go
package domain

import (
    //"crypto-service/internal/risk"
    "math/big"
    "time"
)

type ApprovalStatus string

type RiskFactor struct {
    Factor      string
    Description string
    Score       int
}

const (
    ApprovalStatusPendingReview ApprovalStatus = "pending_review"
    ApprovalStatusApproved      ApprovalStatus = "approved"
    ApprovalStatusRejected      ApprovalStatus = "rejected"
    ApprovalStatusAutoApproved  ApprovalStatus = "auto_approved"
)

type WithdrawalApproval struct {
    ID                  int64
    TransactionID       int64
    UserID              string
    Amount              *big.Int
    Asset               string
    ToAddress           string
    RiskScore           int
    RiskFactors         []RiskFactor
    RequiresApproval    bool
    Status              ApprovalStatus
    ApprovedBy          *string
    ApprovedAt          *time.Time
    RejectionReason     *string
    AutoApprovedReason  *string
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
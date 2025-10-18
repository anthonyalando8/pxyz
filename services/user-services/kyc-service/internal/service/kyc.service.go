package service

import (
	"context"
	"errors"
	"time"
	"fmt"

	"kyc-service/internal/domain"
	"kyc-service/internal/repository"
	"x/shared/utils/id"
	"x/shared/utils/errors"
)

type KYCService struct {
	repo *repository.KYCRepo
	sf   *id.Snowflake
}

type ReviewKYCRequest struct {
	KYCID        string            `json:"kyc_id"`         // set in handler from URL param
	ReviewerID   string            `json:"reviewer_id"`    // admin/system user performing review
	Decision     domain.KYCStatus  `json:"decision"`       // "approved" or "rejected"
	RejectionNote string           `json:"rejection_note"` // optional reason if rejected
}

func NewKYCService(repo *repository.KYCRepo, sf *id.Snowflake) *KYCService {
	return &KYCService{
		repo: repo,
		sf:   sf,
	}
}

func (s *KYCService) Review(ctx context.Context, req *ReviewKYCRequest) error {
	// validate decision
	if req.Decision != domain.KYCStatusApproved && req.Decision != domain.KYCStatusRejected {
		return errors.New("invalid decision: must be 'approved' or 'rejected'")
	}

	// prepare rejection reason pointer
	var rejectionReason *string
	if req.Decision == domain.KYCStatusRejected {
		if req.RejectionNote == "" {
			return errors.New("rejection note is required when rejecting KYC")
		}
		rejectionReason = &req.RejectionNote
	}

	// update status in repo
	if err := s.repo.UpdateStatus(ctx, req.KYCID, req.Decision, rejectionReason); err != nil {
		return err
	}

	// insert audit log
	log := &domain.KYCAuditLog{
		KYCID:  req.KYCID,
		Action: string(req.Decision), // store status as action
		Actor:  req.ReviewerID,
		Notes:  req.RejectionNote,
	}
	return s.repo.InsertAuditLog(ctx, log)
}


// Submit creates a new KYC submission for a user.
func (s *KYCService) Submit(ctx context.Context, userID string, idNumber, docType, frontURL, backURL, faceURL, DOB string) error {
	
	sub := &domain.KYCSubmission{
		ID:               s.sf.Generate(),
		UserID:           userID,
		IDNumber:         idNumber,
		DocumentType:     docType,
		DocumentFrontURL: frontURL,
		DocumentBackURL:  backURL,
		FacePhotoURL:    faceURL,
		Status:           domain.KYCStatusPending,
		SubmittedAt:      time.Now(),
		UpdatedAt:        time.Now(),
	}
	if dob, err := time.Parse("2006-01-02", DOB); err == nil {
		sub.DateOfBirth = dob
	}

	if err := s.repo.Create(ctx, sub); err != nil {
		return err
	}

	// audit log
	log := &domain.KYCAuditLog{
		KYCID:  sub.ID,
		Action: "submitted",
		Actor:  "system",
		Notes:  "User submitted new KYC",
	}
	return s.repo.InsertAuditLog(ctx, log)
}

// Approve sets status to "approved".
func (s *KYCService) Approve(ctx context.Context, kycID string, reviewer string) error {
	if err := s.repo.UpdateStatus(ctx, kycID, domain.KYCStatusApproved, nil); err != nil {
		return err
	}

	log := &domain.KYCAuditLog{
		KYCID:  kycID,
		Action: "approved",
		Actor:  reviewer,
		Notes:  "KYC approved",
	}
	return s.repo.InsertAuditLog(ctx, log)
}

// Reject sets status to "rejected" with a reason.
func (s *KYCService) Reject(ctx context.Context, kycID string, reviewer, reason string) error {
	if err := s.repo.UpdateStatus(ctx, kycID, domain.KYCStatusRejected, &reason); err != nil {
		return err
	}

	log := &domain.KYCAuditLog{
		KYCID:  kycID,
		Action: "rejected",
		Actor:  reviewer,
		Notes:  reason,
	}
	return s.repo.InsertAuditLog(ctx, log)
}

// GetSubmission retrieves a KYC submission by ID.
func (s *KYCService) GetSubmission(ctx context.Context, id string) (*domain.KYCSubmission, error) {
	return s.repo.GetByID(ctx, id)
}

// GetUserSubmission retrieves the latest KYC submission for a user.
func (s *KYCService) GetUserSubmission(ctx context.Context, userID string) (*domain.KYCSubmission, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// GetAuditLogs retrieves audit logs for a submission.
func (s *KYCService) GetAuditLogs(ctx context.Context, kycID string) ([]domain.KYCAuditLog, error) {
	return s.repo.GetAuditLogs(ctx, kycID)
}


// GetStatus returns the current KYC status for a given user.
func (s *KYCService) GetStatus(ctx context.Context, userID string) (*domain.KYCSubmission, error) {
	kyc, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if kyc == nil {
		return nil, fmt.Errorf("no KYC submission found for user: %w", xerrors.ErrNotFound)
	}

	return kyc, nil
}
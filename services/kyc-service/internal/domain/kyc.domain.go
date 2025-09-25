package domain

import "time"

// KYCStatus represents possible states of a KYC submission.
type KYCStatus string

const (
	KYCStatusPending     KYCStatus = "pending"
	KYCStatusUnderReview KYCStatus = "under_review"
	KYCStatusApproved    KYCStatus = "approved"
	KYCStatusRejected    KYCStatus = "rejected"
)

const (
	// Exceptional states
	KYCStatusResubmissionRequired KYCStatus = "resubmission_required"
	KYCStatusExpired              KYCStatus = "expired"
	KYCStatusCancelled            KYCStatus = "cancelled"
)
// KYCSubmission is the main entity for user KYC details.
type KYCSubmission struct {
	ID               string      `json:"id"`
	UserID           string      `json:"user_id"`
	IDNumber         string     `json:"id_number"`
	DocumentType     string     `json:"document_type"`
	DocumentFrontURL string     `json:"document_front_url"`
	DocumentBackURL  string     `json:"document_back_url"`
	FacePhotoURL    string     `json:"face_photo_url"`
	DateOfBirth      time.Time `json:"date_of_birth,omitempty"`
	Status           KYCStatus  `json:"status"`
	RejectionReason  *string    `json:"rejection_reason,omitempty"`
	SubmittedAt      time.Time  `json:"submitted_at"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	UpdatedAt        time.Time  `json:"-"`
}

type KYCSubmissionResponse struct {
	ID               string     `json:"id,omitempty"`
	UserID           string     `json:"user_id,omitempty"`
	IDNumber         string     `json:"id_number,omitempty"`
	DocumentType     string     `json:"document_type,omitempty"`
	DocumentFrontURL string     `json:"document_front_url,omitempty"`
	DocumentBackURL  string     `json:"document_back_url,omitempty"`
	FacePhotoURL     string     `json:"face_photo_url,omitempty"`
	DateOfBirth      *time.Time `json:"date_of_birth,omitempty"`
	Status           KYCStatus  `json:"status,omitempty"`
	RejectionReason  *string    `json:"rejection_reason,omitempty"`
	SubmittedAt      *time.Time `json:"submitted_at,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
}

// KYCAuditLog captures changes in submission state or actions taken.
type KYCAuditLog struct {
	ID        string     `json:"id"`
	KYCID     string     `json:"kyc_id"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

package repository

import (
	"context"
	"errors"

	"kyc-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"x/shared/utils/errors"
)

type KYCRepo struct {
	db *pgxpool.Pool
}

func NewKYCRepo(db *pgxpool.Pool) *KYCRepo {
	return &KYCRepo{db: db}
}

// Create inserts a new KYC submission.
func (r *KYCRepo) Create(ctx context.Context, k *domain.KYCSubmission) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO kyc_submissions 
			(user_id, id_number, selfie_image_url, document_type, document_front_url, document_back_url, status, date_of_birth, submitted_at, updated_at) 
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		RETURNING id
	`, k.UserID, k.IDNumber, k.FacePhotoURL, k.DocumentType, k.DocumentFrontURL, k.DocumentBackURL, k.Status, k.DateOfBirth,
	).Scan(&k.ID)

	return err
}

// GetByID fetches a submission by its ID.
func (r *KYCRepo) GetByID(ctx context.Context, id string) (*domain.KYCSubmission, error) {
	var k domain.KYCSubmission
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, id_number, selfie_image_url, date_of_birth, document_type, document_front_url, document_back_url, status,
		       rejection_reason, submitted_at, reviewed_at, updated_at
		FROM kyc_submissions
		WHERE id=$1
	`, id).Scan(
		&k.ID, &k.UserID, &k.IDNumber, &k.FacePhotoURL, &k.DateOfBirth, &k.DocumentType, &k.DocumentFrontURL, &k.DocumentBackURL,
		&k.Status, &k.RejectionReason, &k.SubmittedAt, &k.ReviewedAt, &k.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}

	return &k, nil
}


// GetByUserID fetches the latest KYC submission for a user.
func (r *KYCRepo) GetByUserID(ctx context.Context, userID string) (*domain.KYCSubmission, error) {
	var k domain.KYCSubmission
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, id_number, selfie_image_url, date_of_birth, document_type,
		       document_front_url, document_back_url, status,
		       rejection_reason, submitted_at, reviewed_at, updated_at
		FROM kyc_submissions
		WHERE user_id=$1
		ORDER BY submitted_at DESC
		LIMIT 1
	`, userID).Scan(
		&k.ID,
		&k.UserID,
		&k.IDNumber,
		&k.FacePhotoURL,
		&k.DateOfBirth,
		&k.DocumentType,
		&k.DocumentFrontURL,
		&k.DocumentBackURL,
		&k.Status,
		&k.RejectionReason,
		&k.SubmittedAt,
		&k.ReviewedAt,
		&k.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}


// UpdateStatus updates the status and optional rejection reason.
func (r *KYCRepo) UpdateStatus(ctx context.Context, id string, status domain.KYCStatus, rejectionReason *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE kyc_submissions
		SET status=$1, rejection_reason=$2, reviewed_at=NOW(), updated_at=NOW()
		WHERE id=$3
	`, status, rejectionReason, id)
	return err
}

// InsertAuditLog records an action in the audit logs.
func (r *KYCRepo) InsertAuditLog(ctx context.Context, log *domain.KYCAuditLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO kyc_audit_logs (kyc_id, action, actor, notes, created_at)
		VALUES ($1,$2,$3,$4,NOW())
	`, log.KYCID, log.Action, log.Actor, log.Notes)
	return err
}

// GetAuditLogs fetches audit logs for a given submission.
func (r *KYCRepo) GetAuditLogs(ctx context.Context, kycID string) ([]domain.KYCAuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, kyc_id, action, actor, notes, created_at
		FROM kyc_audit_logs
		WHERE kyc_id=$1
		ORDER BY created_at ASC
	`, kycID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domain.KYCAuditLog
	for rows.Next() {
		var l domain.KYCAuditLog
		if err := rows.Scan(&l.ID, &l.KYCID, &l.Action, &l.Actor, &l.Notes, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

package repository

import (
	"context"
	"accounting-service/internal/domain"
	"time"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	xerrors "x/shared/utils/errors"
)

type JournalRepository interface {
	Create(ctx context.Context, j *domain.Journal, tx pgx.Tx) error
	GetByID(ctx context.Context, id int64) (*domain.Journal, error)
	ListByAccount(ctx context.Context, accountID int64) ([]*domain.Journal, error)
}

type journalRepo struct {
	db *pgxpool.Pool
}

func NewJournalRepo(db *pgxpool.Pool) JournalRepository {
	return &journalRepo{db: db}
}

// Create inserts a new journal entry inside a transaction
func (r *journalRepo) Create(ctx context.Context, j *domain.Journal, tx pgx.Tx) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}

	err := tx.QueryRow(ctx, `
		INSERT INTO journals (external_ref, idempotency_key, description, created_by_user, created_by_type, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id
	`, j.ExternalRef, j.IdempotencyKey, j.Description, j.CreatedBy, j.CreatedByType, time.Now()).Scan(&j.ID)

	return err
}

// GetByID fetches a journal by its ID
func (r *journalRepo) GetByID(ctx context.Context, id int64) (*domain.Journal, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, external_ref, idempotency_key, description, created_by_user, created_by_type, created_at
		FROM journals
		WHERE id=$1
	`, id)

	var j domain.Journal
	err := row.Scan(
		&j.ID, &j.ExternalRef, &j.IdempotencyKey, &j.Description,
		&j.CreatedBy, &j.CreatedByType, &j.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

// ListByAccount fetches all journals linked to postings of a specific account
func (r *journalRepo) ListByAccount(ctx context.Context, accountID int64) ([]*domain.Journal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT j.id, j.external_ref, j.idempotency_key, j.description, j.created_by_user, j.created_by_type, j.created_at
		FROM journals j
		JOIN postings p ON p.journal_id = j.id
		WHERE p.account_id=$1
		ORDER BY j.created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var journals []*domain.Journal
	for rows.Next() {
		var j domain.Journal
		if err := rows.Scan(
			&j.ID, &j.ExternalRef, &j.IdempotencyKey, &j.Description,
			&j.CreatedBy, &j.CreatedByType, &j.CreatedAt,
		); err != nil {
			return nil, err
		}
		journals = append(journals, &j)
	}

	if len(journals) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return journals, nil
}

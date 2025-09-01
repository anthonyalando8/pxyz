package repository

import (
	"context"
	"partner-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
	"x/shared/utils/id"
)

type PartnerUserRepo struct {
	db *pgxpool.Pool
}

func NewPartnerUserRepo(db *pgxpool.Pool) *PartnerUserRepo {
	return &PartnerUserRepo{db: db}
}

// CreatePartnerUser links a user to a partner
func (r *PartnerUserRepo) CreatePartnerUser(ctx context.Context, u *domain.PartnerUser) error {
	// 1. Generate unique partner_user ID if not set
	if u.ID == "" {
		u.ID = id.GenerateID("PTNU")
	}

	query := `
		INSERT INTO partner_users (id, partner_id, role, user_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		u.ID,
		u.PartnerID,
		u.Role,
		u.UserID,
		u.IsActive,
	).Scan(&u.CreatedAt, &u.UpdatedAt)
}


// GetPartnerUserByID fetches a partner_user by id
func (r *PartnerUserRepo) GetPartnerUserByID(ctx context.Context, id string) (*domain.PartnerUser, error) {
	query := `
		SELECT id, partner_id, role, user_id, is_active, created_at, updated_at
		FROM partner_users
		WHERE id=$1
	`
	row := r.db.QueryRow(ctx, query, id)
	var pu domain.PartnerUser
	err := row.Scan(&pu.ID, &pu.PartnerID, &pu.Role, &pu.UserID, &pu.IsActive, &pu.CreatedAt, &pu.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &pu, nil
}

// UpdatePartnerUser updates role and status
func (r *PartnerUserRepo) UpdatePartnerUser(ctx context.Context, u *domain.PartnerUser) error {
	query := `
		UPDATE partner_users
		SET role=$1, is_active=$2, updated_at=NOW()
		WHERE id=$3
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		u.Role,
		u.IsActive,
		u.ID,
	).Scan(&u.UpdatedAt)
}

// DeletePartnerUser removes the user from a partner
func (r *PartnerUserRepo) DeletePartnerUser(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM partner_users WHERE id=$1`, id)
	return err
}

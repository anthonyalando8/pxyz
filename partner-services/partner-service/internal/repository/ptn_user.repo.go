package repository

import (
	"context"
	"partner-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PartnerUserRepo struct {
	db *pgxpool.Pool
}

func NewPartnerUserRepo(db *pgxpool.Pool) *PartnerUserRepo {
	return &PartnerUserRepo{db: db}
}

// UpdatePartnerUser updates role and status
func (r *PartnerUserRepo) UpdatePartnerUser(ctx context.Context, u *domain.PartnerUser) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update partner_users
	queryPU := `
		UPDATE partner_users
		SET role = $1, is_active = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at
	`
	if err := tx.QueryRow(ctx, queryPU,
		u.Role,
		u.IsActive,
		u.ID,
	).Scan(&u.UpdatedAt); err != nil {
		return err
	}

	// Map IsActive -> account_status
	accountStatus := "active"
	if !u.IsActive {
		accountStatus = "suspended"
	}

	// Update users table
	queryU := `
		UPDATE users
		SET role = $1,
		    account_status = $2,
		    updated_at = NOW()
		WHERE id = $3
	`
	if _, err := tx.Exec(ctx, queryU,
		u.Role,          // sync role too
		accountStatus,   // active/suspended
		u.UserID,        // user_id from PartnerUser struct
	); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
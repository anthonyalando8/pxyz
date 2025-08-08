package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func getString(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

func (r *UserRepository) UpdateEmail(ctx context.Context, userID, newEmail string) error {
	query := `UPDATE users SET email=$1, updated_at=NOW() WHERE id=$2`
	_, err := r.db.Exec(ctx, query, newEmail, userID)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, newPassword string) error {
	query := `UPDATE users SET password=$1, updated_at=NOW() WHERE id=$2`
	_, err := r.db.Exec(ctx, query, newPassword, userID)
	return err
}

package repository

import (
	"context"
	"database/sql"
	"fmt"

	"time"

	"u-rbac-service/internal/domain"
	"x/shared/utils/errors"

	"github.com/jackc/pgx/v5/pgconn"
)



func (r *rbacRepo) CreatePermissionTypes(ctx context.Context, perms []*domain.PermissionType) ([]*domain.PermissionType, []*xerrors.RepoError, error) {
	query := `
		INSERT INTO rbac_permission_types (code, description, is_active, created_at, created_by)
		VALUES ($1, $2, COALESCE($3, TRUE), NOW(), $4)
		RETURNING id, created_at
	`

	var created []*domain.PermissionType
	var errs []*xerrors.RepoError

	for _, p := range perms {
		row := r.db.QueryRow(ctx, query, p.Code, p.Description, p.IsActive, p.CreatedBy)

		var id int64
		var createdAt time.Time
		if err := row.Scan(&id, &createdAt); err != nil {
			// handle conflict (duplicate code)
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
				errs = append(errs, &xerrors.RepoError{
					Entity: "PermissionType",
					Code:   "DUPLICATE",
					Msg:    fmt.Sprintf("permission type with code '%s' already exists", p.Code),
					Ref:    p.Code,
				})
				continue
			}

			// general failure
			return nil, nil, fmt.Errorf("failed to insert permission type (code=%s): %w", p.Code, err)
		}

		p.ID = id
		p.CreatedAt = createdAt
		created = append(created, p)
	}

	return created, errs, nil
}

func (r *rbacRepo) ListPermissionTypes(ctx context.Context) ([]*domain.PermissionType, error) {
	query := `
		SELECT id, code, description, is_active, created_at, created_by, updated_at, updated_by
		FROM rbac_permission_types
		ORDER BY id ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list permission types: %w", err)
	}
	defer rows.Close()

	var results []*domain.PermissionType
	for rows.Next() {
		var p domain.PermissionType
		var updatedAt sql.NullTime
		var updatedBy sql.NullInt64

		if err := rows.Scan(
			&p.ID,
			&p.Code,
			&p.Description,
			&p.IsActive,
			&p.CreatedAt,
			&p.CreatedBy,
			&updatedAt,
			&updatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan permission type row: %w", err)
		}

		if updatedAt.Valid {
			p.UpdatedAt = &updatedAt.Time
		}
		if updatedBy.Valid {
			p.UpdatedBy = &updatedBy.Int64
		}

		results = append(results, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error in ListPermissionTypes: %w", err)
	}

	return results, nil
}

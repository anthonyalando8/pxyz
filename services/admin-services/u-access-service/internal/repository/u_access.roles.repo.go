package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"time"

	"admin-rbac-service/internal/domain"
	"x/shared/utils/errors"

	"github.com/jackc/pgx/v5"
	//"github.com/jackc/pgx/v5/pgconn"
)

func (r *rbacRepo) CreateRoles(ctx context.Context, roles []*domain.Role) ([]*domain.Role, []*xerrors.RepoError, error) {
	query := `
		INSERT INTO rbac_roles (name, description, is_active, created_by)
		VALUES ($1, $2, COALESCE($3, true), $4)
		ON CONFLICT (name) DO UPDATE
		SET description = EXCLUDED.description,
		    is_active = EXCLUDED.is_active,
		    updated_at = NOW(),
		    updated_by = EXCLUDED.created_by
		RETURNING id, created_at, updated_at
	`

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var created []*domain.Role
	var perrs []*xerrors.RepoError

	for _, role := range roles {
		var id int64
		var createdAt time.Time
		var updatedAt *time.Time

		err := tx.QueryRow(ctx, query,
			role.Name,
			role.Description,
			role.IsActive,
			role.CreatedBy,
		).Scan(&id, &createdAt, &updatedAt)

		if err != nil {
			perrs = append(perrs, &xerrors.RepoError{
				Entity: "Role",
				Code:   "DB_INSERT_ERROR",
				Msg:    fmt.Sprintf("failed to insert/update role %q: %v", role.Name, err),
				Ref:    role.Name,
			})
			continue
		}

		role.ID = id
		role.CreatedAt = createdAt
		role.UpdatedAt = updatedAt
		created = append(created, role)
	}

	if len(perrs) > 0 && len(created) == 0 {
		// all inserts failed → rollback
		return nil, perrs, tx.Rollback(ctx)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return created, perrs, nil
}

func (r *rbacRepo) UpdateRole(ctx context.Context, role *domain.Role) error {
	now := time.Now()
	role.UpdatedAt = &now

	query := `
		UPDATE rbac_roles
		SET name = $1,
		    description = $2,
		    is_active = $3,
		    updated_at = $4,
		    updated_by = $5
		WHERE id = $6
	`

	tag, err := r.db.Exec(ctx, query,
		role.Name,
		role.Description,
		role.IsActive,
		role.UpdatedAt,
		role.UpdatedBy,
		role.ID,
	)
	if err != nil {
		return fmt.Errorf("update role %d: %w", role.ID, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("role %d not found", role.ID)
	}

	return nil
}
func (r *rbacRepo) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	query := `
		SELECT id, name, description, is_active, created_at, created_by, updated_at, updated_by
		FROM rbac_roles
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query roles: %w", err)
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		var role domain.Role
		var updatedAt sql.NullTime
		var updatedBy sql.NullInt64

		if err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.IsActive,
			&role.CreatedAt,
			&role.CreatedBy,
			&updatedAt,
			&updatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan role row: %w", err)
		}

		if updatedAt.Valid {
			role.UpdatedAt = &updatedAt.Time
		}
		if updatedBy.Valid {
			role.UpdatedBy = &updatedBy.Int64
		}

		roles = append(roles, &role)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return roles, nil
}

func (r *rbacRepo) AssignRolePermissions(
	ctx context.Context,
	perms []*domain.RolePermission,
) ([]*domain.RolePermission, []*xerrors.RepoError, error) {

	query := `
		INSERT INTO rbac_role_permissions (
			role_id, module_id, submodule_id, permission_type_id, allow, created_by
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING
		RETURNING id, role_id, module_id, submodule_id, permission_type_id,
		          allow, created_at, created_by, updated_at, updated_by
	`

	var created []*domain.RolePermission
	var errs []*xerrors.RepoError

	for _, p := range perms {
		var rp domain.RolePermission
		var updatedAt sql.NullTime
		var updatedBy sql.NullInt64

		row := r.db.QueryRow(
			ctx,
			query,
			p.RoleID,
			p.ModuleID,
			p.SubmoduleID,
			p.PermissionTypeID,
			p.Allow,
			p.CreatedBy,
		)

		err := row.Scan(
			&rp.ID,
			&rp.RoleID,
			&rp.ModuleID,
			&rp.SubmoduleID,
			&rp.PermissionTypeID,
			&rp.Allow,
			&rp.CreatedAt,
			&rp.CreatedBy,
			&updatedAt,
			&updatedBy,
		)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// duplicate case (conflict -> DO NOTHING -> no row returned)
				// errs = append(errs, &xerrors.RepoError{
				// 	Entity: "RolePermission",
				// 	Code:   "duplicate",
				// 	Msg:    fmt.Sprintf("permission already assigned: role=%d, module=%d, submodule=%v, perm_type=%d",
				// 		p.RoleID, p.ModuleID, p.SubmoduleID, p.PermissionTypeID),
				// 	Ref: fmt.Sprintf("role:%d,module:%d,sub:%v,perm:%d",
				// 		p.RoleID, p.ModuleID, p.SubmoduleID, p.PermissionTypeID),
				// })
				continue
			}
			return nil, nil, fmt.Errorf("failed to assign role permission: %w", err)
		}

		if updatedAt.Valid {
			rp.UpdatedAt = &updatedAt.Time
		}
		if updatedBy.Valid {
			rp.UpdatedBy = &updatedBy.Int64
		}

		created = append(created, &rp)
	}

	return created, errs, nil
}

func (r *rbacRepo) ListRolePermissions(ctx context.Context, roleID int64) ([]*domain.RolePermission, error) {
	query := `
		SELECT 
			id, role_id, module_id, submodule_id, permission_type_id, allow,
			created_at, created_by, updated_at, updated_by
		FROM rbac_role_permissions
		WHERE role_id = $1
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list role permissions: %w", err)
	}
	defer rows.Close()

	var result []*domain.RolePermission

	for rows.Next() {
		var rp domain.RolePermission
		var updatedAt sql.NullTime
		var updatedBy sql.NullInt64

		if err := rows.Scan(
			&rp.ID,
			&rp.RoleID,
			&rp.ModuleID,
			&rp.SubmoduleID,
			&rp.PermissionTypeID,
			&rp.Allow,
			&rp.CreatedAt,
			&rp.CreatedBy,
			&updatedAt,
			&updatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan role permission: %w", err)
		}

		if updatedAt.Valid {
			rp.UpdatedAt = &updatedAt.Time
		}
		if updatedBy.Valid {
			rp.UpdatedBy = &updatedBy.Int64
		}

		result = append(result, &rp)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iteration error in list role permissions: %w", rows.Err())
	}

	return result, nil
}

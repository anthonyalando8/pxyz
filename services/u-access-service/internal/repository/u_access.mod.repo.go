package repository

import (
	"context"
	"database/sql"
	"fmt"

	"time"

	"u-rbac-service/internal/domain"
	"x/shared/utils/errors"

	"github.com/jackc/pgx/v5"

)

func (r *rbacRepo) CreateModules(ctx context.Context, modules []*domain.Module) ([]*domain.Module, []*xerrors.RepoError, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var created []*domain.Module
	var errs []*xerrors.RepoError

	for _, m := range modules {
		query := `
			INSERT INTO modules (parent_id, code, name, meta, is_active, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, created_at
		`
		err := tx.QueryRow(ctx, query,
			m.ParentID, m.Code, m.Name, m.Meta, m.IsActive, m.CreatedBy,
		).Scan(&m.ID, &m.CreatedAt)

		if err != nil {
			repoErr := &xerrors.RepoError{
				Entity: "module",
				Code:   xerrors.ParsePGErrorCode(err),
				Msg:    err.Error(),
				Ref:    m.Code,
			}
			errs = append(errs, repoErr)
			continue
		}
		created = append(created, m)
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return nil, nil, commitErr
	}

	return created, errs, nil
}

func (r *rbacRepo) UpdateModule(ctx context.Context, module *domain.Module) error {
	query := `
		UPDATE rbac_modules
		SET code = $1,
		    name = $2,
		    parent_id = $3,
		    meta = $4,
		    is_active = $5,
		    updated_at = now(),
		    updated_by = $6
		WHERE id = $7
		RETURNING id
	`

	var parentID interface{}
	if module.ParentID != nil {
		parentID = *module.ParentID
	}

	var updatedBy interface{}
	if module.UpdatedBy != nil {
		updatedBy = *module.UpdatedBy
	}

	var idReturned int64
	err := r.db.QueryRow(ctx, query,
		module.Code,
		module.Name,
		parentID,
		module.Meta,
		module.IsActive,
		updatedBy,
		module.ID,
	).Scan(&idReturned)

	if err != nil {
		if err == pgx.ErrNoRows {
			return xerrors.ErrNotFound
		}
		return fmt.Errorf("update module id=%d: %w", module.ID, err)
	}

	return nil
}
func (r *rbacRepo) GetModuleByCode(ctx context.Context, code string) (*domain.Module, error) {
	query := `
		SELECT id, parent_id, code, name, meta, is_active,
		       created_at, created_by, updated_at, updated_by
		FROM rbac_modules
		WHERE code = $1
	`

	m := &domain.Module{}
	var metaBytes []byte
	var parentID sql.NullInt64
	var updatedAt sql.NullTime
	var updatedBy sql.NullInt64

	err := r.db.QueryRow(ctx, query, code).Scan(
		&m.ID,
		&parentID,
		&m.Code,
		&m.Name,
		&metaBytes,
		&m.IsActive,
		&m.CreatedAt,
		&m.CreatedBy,
		&updatedAt,
		&updatedBy,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, fmt.Errorf("get module by code=%s: %w", code, err)
	}

	// handle nullable fields
	if parentID.Valid {
		m.ParentID = &parentID.Int64
	}
	if updatedAt.Valid {
		m.UpdatedAt = &updatedAt.Time
	}
	if updatedBy.Valid {
		m.UpdatedBy = &updatedBy.Int64
	}
	m.Meta = metaBytes

	return m, nil
}

func (r *rbacRepo) ListModules(ctx context.Context) ([]*domain.Module, error) {
	query := `
		SELECT 
			id,
			parent_id,
			code,
			name,
			meta,
			is_active,
			created_at,
			created_by,
			updated_at,
			updated_by
		FROM modules
		ORDER BY id ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query modules: %w", err)
	}
	defer rows.Close()

	var modules []*domain.Module
	for rows.Next() {
		var m domain.Module
		err := rows.Scan(
			&m.ID,
			&m.ParentID,
			&m.Code,
			&m.Name,
			&m.Meta,
			&m.IsActive,
			&m.CreatedAt,
			&m.CreatedBy,
			&m.UpdatedAt,
			&m.UpdatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan module: %w", err)
		}
		modules = append(modules, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return modules, nil
}

func (r *rbacRepo) CreateSubmodules(ctx context.Context, subs []*domain.Submodule) ([]*domain.Submodule, []*xerrors.RepoError, error) {
	query := `
		INSERT INTO rbac_submodules (
			module_id, code, name, meta, is_active, created_at, created_by
		)
		VALUES ($1, $2, $3, $4, COALESCE($5, true), $6, $7)
		RETURNING id, created_at
	`

	var createdSubs []*domain.Submodule
	var repoErrs []*xerrors.RepoError

	for _, s := range subs {
		var id int64
		var createdAt time.Time

		err := r.db.QueryRow(
			ctx,
			query,
			s.ModuleID,
			s.Code,
			s.Name,
			s.Meta,
			s.IsActive,
			time.Now(),
			s.CreatedBy,
		).Scan(&id, &createdAt)

		if err != nil {
			repoErrs = append(repoErrs, &xerrors.RepoError{
				Entity: "submodule",
				Code:   "INSERT_FAILED",
				Msg:    fmt.Sprintf("failed to insert submodule with code=%s: %v", s.Code, err),
				Ref:    s.Code,
			})
			continue
		}

		s.ID = id
		s.CreatedAt = createdAt
		createdSubs = append(createdSubs, s)
	}

	if len(createdSubs) == 0 && len(repoErrs) > 0 {
		return nil, repoErrs, fmt.Errorf("all submodule inserts failed")
	}

	return createdSubs, repoErrs, nil
}

func (r *rbacRepo) UpdateSubmodule(ctx context.Context, sub *domain.Submodule) error {
	query := `
		UPDATE rbac_submodules
		SET name = $1,
		    meta = $2,
		    is_active = $3,
		    updated_at = NOW(),
		    updated_by = $4
		WHERE id = $5
		RETURNING id
	`

	var id int64
	err := r.db.QueryRow(ctx, query,
		sub.Name,
		sub.Meta,
		sub.IsActive,
		sub.UpdatedBy,
		sub.ID,
	).Scan(&id)

	if err != nil {
		if err == pgx.ErrNoRows {
			return xerrors.ErrNotFound
		}
		return fmt.Errorf("update submodule id=%d: %w", sub.ID, err)
	}

	return nil
}
func (r *rbacRepo) GetSubmoduleByCode(ctx context.Context, moduleID int64, code string) (*domain.Submodule, error) {
	query := `
		SELECT id, module_id, code, name, meta, is_active, 
		       created_at, created_by, updated_at, updated_by
		FROM rbac_submodules
		WHERE module_id = $1 AND code = $2
	`

	row := r.db.QueryRow(ctx, query, moduleID, code)

	var sub domain.Submodule
	err := row.Scan(
		&sub.ID,
		&sub.ModuleID,
		&sub.Code,
		&sub.Name,
		&sub.Meta,
		&sub.IsActive,
		&sub.CreatedAt,
		&sub.CreatedBy,
		&sub.UpdatedAt,
		&sub.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound // not found
		}
		return nil, fmt.Errorf("get submodule by code (module=%d, code=%s): %w", moduleID, code, err)
	}

	return &sub, nil
}

func (r *rbacRepo) ListSubmodules(ctx context.Context, moduleID int64) ([]*domain.Submodule, error) {
	query := `
		SELECT 
			id, module_id, code, name, meta, is_active, 
			created_at, created_by, updated_at, updated_by
		FROM rbac_submodules
		WHERE module_id = $1 AND is_active = true
		ORDER BY id ASC
	`

	rows, err := r.db.Query(ctx, query, moduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query submodules for module_id=%d: %w", moduleID, err)
	}
	defer rows.Close()

	var subs []*domain.Submodule
	for rows.Next() {
		var s domain.Submodule
		var meta []byte
		var updatedAt *time.Time
		var updatedBy *int64

		if err := rows.Scan(
			&s.ID,
			&s.ModuleID,
			&s.Code,
			&s.Name,
			&meta,
			&s.IsActive,
			&s.CreatedAt,
			&s.CreatedBy,
			&updatedAt,
			&updatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan submodule row: %w", err)
		}

		s.Meta = meta
		s.UpdatedAt = updatedAt
		s.UpdatedBy = updatedBy

		subs = append(subs, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error while listing submodules: %w", err)
	}

	return subs, nil
}
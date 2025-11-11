package repository

import (
	"context"
	"fmt"
	"log"
	"strconv"
		"x/shared/utils/errors"
	"admin-rbac-service/internal/domain"
)

func (r *rbacRepo) AssignUserRoles(ctx context.Context, roles []*domain.UserRole) ([]*domain.UserRole, []*xerrors.RepoError, error) {
	query := `
		INSERT INTO rbac_user_roles (user_id, role_id, assigned_by, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, role_id) DO UPDATE 
			SET assigned_by = EXCLUDED.assigned_by,
			    updated_at = NOW()
		RETURNING id, user_id, role_id, assigned_by, created_at, updated_at, updated_by
	`

	var created []*domain.UserRole
	var perr []*xerrors.RepoError

	log.Printf("🔄 Starting assignment of %d user roles", len(roles))

	for _, ur := range roles {
		var newUR domain.UserRole
		err := r.db.QueryRow(ctx, query, ur.UserID, ur.RoleID, ur.AssignedBy).Scan(
			&newUR.ID,
			&newUR.UserID,
			&newUR.RoleID,
			&newUR.AssignedBy,
			&newUR.CreatedAt,
			&newUR.UpdatedAt,
			&newUR.UpdatedBy,
		)

		if err != nil {
			log.Printf("❌ Failed to assign role %d to user %s: %v", ur.RoleID, ur.UserID, err)

			re := &xerrors.RepoError{
				Entity: "UserRole",
				Code:   "INSERT_FAILED",
				Msg:    err.Error(),
				Ref:    fmt.Sprintf("user:%s-role:%d", ur.UserID, ur.RoleID),
			}
			perr = append(perr, re)
			continue
		}

		log.Printf("✅ Assigned role %d to user %s (id: %d)", newUR.RoleID, newUR.UserID, newUR.ID)
		created = append(created, &newUR)
	}

	log.Printf("📦 Finished user-role assignment: Success: %d | Failed: %d", len(created), len(perr))

	if len(perr) > 0 && len(created) == 0 {
		return nil, perr, nil // all failed
	}

	return created, perr, nil
}

func (r *rbacRepo) BatchAssignRolesToUsers(
	ctx context.Context,
	systemUserID int64,
	roleIDResolver func(ctx context.Context, roleName string) (int64, error),
) error {
	// Step 1: Fetch and classify users without roles
	assignments, err := r.GetUsersWithoutRolesAndClassify(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unassigned users: %w", err)
	}

	total := len(assignments)
	if total == 0 {
		fmt.Println("✅ No users without roles found. Nothing to assign.")
		return nil
	}

	fmt.Printf("📝 Found %d users without roles to classify and assign\n", total)

	const batchSize = 1000
	var rolesBatch []*domain.UserRole
	var totalAssigned int
	var totalFailed int

	for i, ua := range assignments {
		roleID, err := roleIDResolver(ctx, ua.RoleName)
		if err != nil {
			fmt.Printf("❌ Failed to resolve role ID for user %s (role: %s): %v\n", ua.UserID, ua.RoleName, err)
			totalFailed++
			continue
		}

		rolesBatch = append(rolesBatch, &domain.UserRole{
			UserID:     ua.UserID,
			RoleID:     roleID,
			AssignedBy: systemUserID,
		})

		// When batch full or final item — insert
		if len(rolesBatch) == batchSize || i == len(assignments)-1 {
			fmt.Printf("📦 Assigning batch of %d users...\n", len(rolesBatch))

			created, perr, err := r.AssignUserRoles(ctx, rolesBatch)
			if err != nil {
				fmt.Printf("🚨 Batch insert failed: %v\n", err)
				totalFailed += len(rolesBatch)
				rolesBatch = rolesBatch[:0]
				continue
			}

			totalAssigned += len(created)

			if len(perr) > 0 {
				totalFailed += len(perr)
				for _, e := range perr {
					fmt.Printf("⚠️ Partial failure: user-role %s → %s\n", e.Ref, e.Msg)
				}
			}

			fmt.Printf("✅ Batch completed. Assigned: %d, Partial failures: %d\n", len(created), len(perr))
			rolesBatch = rolesBatch[:0] // reset batch
		}
	}

	fmt.Printf("🎯 Done. Total users processed: %d | Assigned: %d | Failed: %d\n", total, totalAssigned, totalFailed)

	return nil
}

func (r *rbacRepo) GetUsersWithoutRolesAndClassify(ctx context.Context) ([]*UserRoleAssignment, error) {
	query := `
		SELECT 
		u.id::TEXT AS user_id,
		CASE
			WHEN 
				(u.account_type = 'hybrid' AND uc.password_hash IS NULL)
				OR (u.account_type = 'password' AND uc.password_hash IS NULL)
				OR NOT uc.is_email_verified
				OR up.nationality IS NULL
			THEN 'any'

			WHEN ks.status IS DISTINCT FROM 'approved'
			THEN 'kyc_unverified'

			ELSE 'trader'
		END AS role_name
	FROM users u
	LEFT JOIN user_credentials uc ON uc.user_id = u.id
	LEFT JOIN rbac_user_roles rur ON rur.user_id = u.id
	LEFT JOIN (
		SELECT DISTINCT ON (user_id) *
		FROM kyc_submissions
		ORDER BY user_id, submitted_at DESC
	) ks ON ks.user_id = u.id
	LEFT JOIN user_profiles up ON up.user_id = u.id
	WHERE rur.user_id IS NULL
	AND u.account_status != 'deleted';
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to classify users: %w", err)
	}
	defer rows.Close()

	var result []*UserRoleAssignment
	for rows.Next() {
		var ua UserRoleAssignment
		if err := rows.Scan(&ua.UserID, &ua.RoleName); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		result = append(result, &ua)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

// UpgradeUserRole replaces a user's current role with a new role
func (r *rbacRepo) UpgradeUserRole(ctx context.Context, userID string, newRoleID, assignedBy int64) (*domain.UserRole, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Check if user already has the new role
	var exists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM rbac_user_roles
			WHERE user_id = $1 AND role_id = $2
		)
	`, userID, newRoleID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing role: %w", err)
	}
	if exists {
		// already has role, nothing to do
		return nil, nil
	}

	// Delete current role (assumes only 1 role per user)
	_, err = tx.Exec(ctx, `
		DELETE FROM rbac_user_roles
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to remove old role: %w", err)
	}

	// Insert new role
	var ur domain.UserRole
	err = tx.QueryRow(ctx, `
		INSERT INTO rbac_user_roles (user_id, role_id, assigned_by, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, user_id, role_id, assigned_by, created_at, updated_at, updated_by
	`, userID, newRoleID, assignedBy).Scan(
		&ur.ID, &ur.UserID, &ur.RoleID, &ur.AssignedBy, &ur.CreatedAt, &ur.UpdatedAt, &ur.UpdatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to assign new role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &ur, nil
}

func (r *rbacRepo) ListUserRoles(ctx context.Context, userID string) ([]*domain.UserRole, error) {
	const query = `
		SELECT 
			rur.id,
			rur.user_id,
			rur.role_id,
			rr.name AS role_name,
			rur.assigned_by,
			rur.created_at,
			rur.updated_at,
			rur.updated_by
		FROM rbac_user_roles rur
		JOIN rbac_roles rr ON rur.role_id = rr.id
		WHERE rur.user_id = $1
		ORDER BY rur.created_at DESC
	`

	// convert userID string → int64
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid userID: %w", err)
	}

	rows, err := r.db.Query(ctx, query, uid)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var roles []*domain.UserRole
	for rows.Next() {
		var ur domain.UserRole
		if err := rows.Scan(
			&ur.ID,
			&ur.UserID,
			&ur.RoleID,
			&ur.RoleName, // Add this field in your struct
			&ur.AssignedBy,
			&ur.CreatedAt,
			&ur.UpdatedAt,
			&ur.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		roles = append(roles, &ur)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return roles, nil
}

func (r *rbacRepo) AssignUserPermissionOverrides(
	ctx context.Context,
	overrides []*domain.UserPermissionOverride,
) ([]*domain.UserPermissionOverride, []*xerrors.RepoError, error) {

	var saved []*domain.UserPermissionOverride
	var errs []*xerrors.RepoError

	for _, o := range overrides {
		var q string
		var args []interface{}

		if o.SubmoduleID != nil { // Submodule-level permission
			q = `
				INSERT INTO rbac_user_permissions_override
					(user_id, module_id, submodule_id, permission_type_id, allow, created_by)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (user_id, module_id, submodule_id, permission_type_id)
				DO UPDATE SET
					allow = EXCLUDED.allow,
					updated_at = now(),
					updated_by = EXCLUDED.created_by
				RETURNING id, user_id, module_id, submodule_id, permission_type_id,
						allow, created_at, created_by, updated_at, updated_by
			`
			args = []interface{}{o.UserID, o.ModuleID, o.SubmoduleID, o.PermissionTypeID, o.Allow, o.CreatedBy}
		} else { // Module-level permission
			q = `
				INSERT INTO rbac_user_permissions_override
					(user_id, module_id, permission_type_id, allow, created_by)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (user_id, module_id, submodule_id, permission_type_id)
				DO UPDATE SET
					allow = EXCLUDED.allow,
					updated_at = now(),
					updated_by = EXCLUDED.created_by
				RETURNING id, user_id, module_id, submodule_id, permission_type_id,
						allow, created_at, created_by, updated_at, updated_by
			`
			args = []interface{}{o.UserID, o.ModuleID, o.PermissionTypeID, o.Allow, o.CreatedBy}
		}

		row := r.db.QueryRow(ctx, q, args...)

		var savedO domain.UserPermissionOverride
		err := row.Scan(
			&savedO.ID,
			&savedO.UserID,
			&savedO.ModuleID,
			&savedO.SubmoduleID,
			&savedO.PermissionTypeID,
			&savedO.Allow,
			&savedO.CreatedAt,
			&savedO.CreatedBy,
			&savedO.UpdatedAt,
			&savedO.UpdatedBy,
		)
		if err != nil {
			errs = append(errs, &xerrors.RepoError{
				Entity: "UserPermissionOverride",
				Code:   "DB_ERROR",
				Msg:    err.Error(),
				Ref: fmt.Sprintf("user_id=%s,module_id=%d,submodule_id=%v,perm_type_id=%d",
					o.UserID, o.ModuleID, o.SubmoduleID, o.PermissionTypeID),
			})
			continue
		}

		saved = append(saved, &savedO)
	}

	if len(errs) > 0 && len(saved) == 0 {
		return nil, errs, fmt.Errorf("failed to assign any user permission overrides")
	}

	return saved, errs, nil
}

func (r *rbacRepo) ListUserPermissionOverrides(
	ctx context.Context,
	userID string,
) ([]*domain.UserPermissionOverride, error) {
	const q = `
		SELECT id, user_id, module_id, submodule_id, permission_type_id,
		       allow, created_at, created_by, updated_at, updated_by
		FROM rbac_user_permissions_override
		WHERE user_id = $1
	`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query user permission overrides: %w", err)
	}
	defer rows.Close()

	var overrides []*domain.UserPermissionOverride
	for rows.Next() {
		var o domain.UserPermissionOverride
		err := rows.Scan(
			&o.ID,
			&o.UserID,
			&o.ModuleID,
			&o.SubmoduleID,
			&o.PermissionTypeID,
			&o.Allow,
			&o.CreatedAt,
			&o.CreatedBy,
			&o.UpdatedAt,
			&o.UpdatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user permission override: %w", err)
		}
		overrides = append(overrides, &o)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate user permission overrides: %w", rows.Err())
	}

	return overrides, nil
}

func (r *rbacRepo) GetEffectivePermissions(
	ctx context.Context,
	userID string,
	moduleCode *string,
	submoduleCode *string,
) ([]*domain.EffectivePermission, error) {
	query := `
SELECT 
    m.id AS module_id,
    m.code AS module_code,
    m.name AS module_name,
    m.is_active AS module_active,
    s.id AS submodule_id,
    s.code AS submodule_code,
    s.name AS submodule_name,
    s.is_active AS submodule_active,
    pt.id AS permission_type_id,
    pt.code AS permission_code,
    pt.description AS permission_name,
    COALESCE(uo.allow, rp.allow) AS allow,
    ur.user_id,
    rp.role_id,
    rp.created_at,
    rp.updated_at
FROM rbac_user_roles ur
JOIN rbac_roles r ON r.id = ur.role_id
JOIN rbac_role_permissions rp ON rp.role_id = r.id
JOIN rbac_modules m ON m.id = rp.module_id
LEFT JOIN rbac_submodules s ON s.id = rp.submodule_id
JOIN rbac_permission_types pt ON pt.id = rp.permission_type_id
LEFT JOIN rbac_user_permissions_override uo 
    ON uo.user_id = ur.user_id 
    AND uo.module_id = rp.module_id 
    AND (uo.submodule_id = rp.submodule_id OR (uo.submodule_id IS NULL AND rp.submodule_id IS NULL))
WHERE ur.user_id = $1
`
	args := []interface{}{userID}

	if moduleCode != nil {
		query += " AND m.code = $2"
		args = append(args, *moduleCode)
	}

	if submoduleCode != nil {
		if moduleCode != nil {
			query += " AND s.code = $3"
		} else {
			query += " AND s.code = $2"
		}
		args = append(args, *submoduleCode)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.EffectivePermission
	for rows.Next() {
		var ep domain.EffectivePermission
		err := rows.Scan(
			&ep.ModuleID,
			&ep.ModuleCode,
			&ep.ModuleName,
			&ep.ModuleActive,
			&ep.SubmoduleID,
			&ep.SubmoduleCode,
			&ep.SubmoduleName,
			&ep.SubmoduleActive,
			&ep.PermissionTypeID,
			&ep.PermissionCode,
			&ep.PermissionName,
			&ep.Allow,
			&ep.UserID,
			&ep.RoleID,
			&ep.CreatedAt,
			&ep.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, &ep)
	}

	return result, nil
}

func GroupEffectivePermissions(eps []*domain.EffectivePermission) []*domain.ModuleWithPermissions {
	modMap := make(map[int64]*domain.ModuleWithPermissions)

	for _, ep := range eps {
		// Module
		mod, exists := modMap[ep.ModuleID]
		if !exists {
			mod = &domain.ModuleWithPermissions{
				ID:          ep.ModuleID,
				Code:        ep.ModuleCode,
				Name:        ep.ModuleName,
				IsActive:    ep.ModuleActive,
				Permissions: []*domain.PermissionInfo{},
				Submodules:  []*domain.SubmoduleWithPermissions{},
			}
			modMap[ep.ModuleID] = mod
		}

		// Decide if this is a module-level or submodule-level permission
		if ep.SubmoduleID == nil {
			// ✅ module-level permission
			mod.Permissions = append(mod.Permissions, &domain.PermissionInfo{
				ID:      ep.PermissionTypeID,
				Code:    ep.PermissionCode,
				Name:    ep.PermissionName,
				Allowed: ep.Allow,
				RoleID:  ep.RoleID,
				UserID:  ep.UserID,
			})
		} else {
			// ✅ submodule-level permission
			var sub *domain.SubmoduleWithPermissions
			for _, sm := range mod.Submodules {
				if sm.ID != nil && *sm.ID == *ep.SubmoduleID {
					sub = sm
					break
				}
			}
			if sub == nil {
				sub = &domain.SubmoduleWithPermissions{
					ID:          ep.SubmoduleID,
					Code:        ep.SubmoduleCode,
					Name:        ep.SubmoduleName,
					IsActive:    ep.SubmoduleActive,
					Permissions: []*domain.PermissionInfo{},
				}
				mod.Submodules = append(mod.Submodules, sub)
			}

			sub.Permissions = append(sub.Permissions, &domain.PermissionInfo{
				ID:      ep.PermissionTypeID,
				Code:    ep.PermissionCode,
				Name:    ep.PermissionName,
				Allowed: ep.Allow,
				RoleID:  ep.RoleID,
				UserID:  ep.UserID,
			})
		}
	}

	// Convert map to slice
	result := make([]*domain.ModuleWithPermissions, 0, len(modMap))
	for _, mod := range modMap {
		result = append(result, mod)
	}

	return result
}

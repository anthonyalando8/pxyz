package repository

import (
    "auth-service/internal/domain"
    "context"
    "fmt"

    "github.com/jackc/pgx/v5"
)

func (r *UserRepository) GetOrCreateRole(ctx context.Context, role domain.Role) (*domain.Role, error) {
	query := `
		INSERT INTO roles (name, description)
		VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE 
		SET description = EXCLUDED.description
		RETURNING id, name, description
	`

	var dbRole domain.Role
	err := r.db.QueryRow(ctx, query, role.Name, role.Description).
		Scan(&dbRole.ID, &dbRole.Name, &dbRole.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create role: %w", err)
	}

	return &dbRole, nil
}



func (r *UserRepository) CreateRoles(ctx context.Context, roles []domain.Role) error {
    if len(roles) == 0 {
        return nil
    }

    batch := &pgx.Batch{}

    for _, role := range roles {
        query := `
            INSERT INTO roles (name, description)
            VALUES ($1, $2)
            ON CONFLICT (name) DO NOTHING
        `
        batch.Queue(query, role.Name, role.Description)
    }

    br := r.db.SendBatch(ctx, batch)
    defer br.Close()

    for range roles {
        if _, err := br.Exec(); err != nil {
            return fmt.Errorf("failed to insert role: %w", err)
        }
    }

    return nil
}


func (r *UserRepository) CreatePermissions(ctx context.Context, perms []domain.Permission) error {
    if len(perms) == 0 {
        return nil
    }

    batch := &pgx.Batch{}

    for _, p := range perms {
        query := `
            INSERT INTO permissions (name, description)
            VALUES ($1, $2)
            ON CONFLICT (name) DO NOTHING
        `
        batch.Queue(query, p.Name, p.Description)
    }

    br := r.db.SendBatch(ctx, batch)
    defer br.Close()

    for range perms {
        if _, err := br.Exec(); err != nil {
            return fmt.Errorf("failed to insert permission: %w", err)
        }
    }

    return nil
}



func (r *UserRepository) AssignPermissionToRole(ctx context.Context, roleID, permissionID int) error {
	query := `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to assign permission to role: %w", err)
	}
	return nil
}

func (r *UserRepository) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error {
	query := `
		DELETE FROM role_permissions
		WHERE role_id = $1 AND permission_id = $2
	`
	_, err := r.db.Exec(ctx, query, roleID, permissionID)
	return err
}

func (r *UserRepository) AssignRoleToUser(ctx context.Context, userID string, roleID int) error {
	query := `
		INSERT INTO user_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to assign role to user: %w", err)
	}
	return nil
}

func (r *UserRepository) RemoveRoleFromUser(ctx context.Context, userID int64, roleID int) error {
	query := `
		DELETE FROM user_roles
		WHERE user_id = $1 AND role_id = $2
	`
	_, err := r.db.Exec(ctx, query, userID, roleID)
	return err
}


func (r *UserRepository) SetUserPermission(ctx context.Context, userID int64, permissionID int, isAllowed bool) error {
	query := `
		INSERT INTO user_permissions (user_id, permission_id, is_allowed)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, permission_id)
		DO UPDATE SET is_allowed = EXCLUDED.is_allowed, assigned_at = NOW()
	`
	_, err := r.db.Exec(ctx, query, userID, permissionID, isAllowed)
	if err != nil {
		return fmt.Errorf("failed to set user permission: %w", err)
	}
	return nil
}

func (r *UserRepository) RemoveUserPermission(ctx context.Context, userID int64, permissionID int) error {
	query := `
		DELETE FROM user_permissions
		WHERE user_id = $1 AND permission_id = $2
	`
	_, err := r.db.Exec(ctx, query, userID, permissionID)
	return err
}



// roles := []domain.Role{
//     {Name: "system_admin", Description: "Full access to system"},
//     {Name: "partner_admin", Description: "Admin for a partner"},
//     {Name: "partner_user", Description: "User under a partner"},
//     {Name: "trader", Description: "Regular trading user"},
// }

// if err := userRepo.CreateRoles(ctx, roles); err != nil {
//     log.Fatalf("failed to seed roles: %v", err)
// }

package usecase

import (
	"context"
	"fmt"
	"auth-service/internal/domain"
)

func (uc *UserUsecase) AssignRoleToUserHelper(ctx context.Context, userID string, role domain.Role) error {
	// Ensure role exists in DB (insert if missing, update description if changed)
	dbRole, err := uc.userRepo.GetOrCreateRole(ctx, role)
	if err != nil {
		return fmt.Errorf("could not resolve role %s: %w", role.Name, err)
	}

	// Assign role to user
	if err := uc.userRepo.AssignRoleToUser(ctx, userID, dbRole.ID); err != nil {
		return fmt.Errorf("could not assign role to user: %w", err)
	}

	return nil
}



func (uc *UserUsecase) CreateRoles(ctx context.Context, roles []domain.Role) error {
	if len(roles) == 0 {
		return nil
	}

	if err := uc.userRepo.CreateRoles(ctx, roles); err != nil {
		return fmt.Errorf("usecase: failed to create roles: %w", err)
	}

	return nil
}

func (uc *UserUsecase) CreatePermissions(ctx context.Context, perms []domain.Permission) error {
	if len(perms) == 0 {
		return nil
	}

	if err := uc.userRepo.CreatePermissions(ctx, perms); err != nil {
		return fmt.Errorf("usecase: failed to create permissions: %w", err)
	}

	return nil
}

func (uc *UserUsecase) AssignPermissionToRole(ctx context.Context, roleID, permissionID int) error {
	if err := uc.userRepo.AssignPermissionToRole(ctx, roleID, permissionID); err != nil {
		return fmt.Errorf("usecase: failed to assign permission to role: %w", err)
	}
	return nil
}

func (uc *UserUsecase) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error {
	if err := uc.userRepo.RemovePermissionFromRole(ctx, roleID, permissionID); err != nil {
		return fmt.Errorf("usecase: failed to remove permission from role: %w", err)
	}
	return nil
}


func (uc *UserUsecase) AssignRoleToUser(ctx context.Context, userID string, roleID int) error {
	if err := uc.userRepo.AssignRoleToUser(ctx, userID, roleID); err != nil {
		return fmt.Errorf("usecase: failed to assign role to user: %w", err)
	}
	return nil
}

func (uc *UserUsecase) RemoveRoleFromUser(ctx context.Context, userID int64, roleID int) error {
	if err := uc.userRepo.RemoveRoleFromUser(ctx, userID, roleID); err != nil {
		return fmt.Errorf("usecase: failed to remove role from user: %w", err)
	}
	return nil
}

func (uc *UserUsecase) SetUserPermission(ctx context.Context, userID int64, permissionID int, isAllowed bool) error {
	if err := uc.userRepo.SetUserPermission(ctx, userID, permissionID, isAllowed); err != nil {
		return fmt.Errorf("usecase: failed to set user permission: %w", err)
	}
	return nil
}

func (uc *UserUsecase) RemoveUserPermission(ctx context.Context, userID int64, permissionID int) error {
	if err := uc.userRepo.RemoveUserPermission(ctx, userID, permissionID); err != nil {
		return fmt.Errorf("usecase: failed to remove user permission: %w", err)
	}
	return nil
}


// GetUserEffectiveRolesPermissions retrieves a user's role(s) along with their permissions,
// considering role permissions overridden by user-specific permissions (is_allowed).
func (uc *UserUsecase) GetUserRoleWithPermissions(ctx context.Context, userID string) (*domain.RoleWithPermissions, error) {
	// Step 1: Get the user's role (or default "trader" role)
	role, err := uc.userRepo.GetOrCreateUserRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}

	// Step 2: Get all permissions for that role
	rolePerms, err := uc.userRepo.GetRolePermissions(ctx, role.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}

	// Step 3: Get user-specific permissions (overrides)
	userPerms, err := uc.userRepo.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	// Step 4: Merge permissions
	permMap := make(map[string]domain.Permission)

	// Role permissions default to IsAllowed=true
	for _, rp := range rolePerms {
		rp.IsAllowed = true
		permMap[rp.Name] = rp
	}

	// User-specific permissions override role permissions
	for _, up := range userPerms {
		permMap[up.Name] = up
	}

	// Step 5: Collect final permissions
	var effectivePerms []domain.Permission
	for _, p := range permMap {
		effectivePerms = append(effectivePerms, p)
	}

	return &domain.RoleWithPermissions{
		Role:        *role,
		Permissions: effectivePerms,
	}, nil
}




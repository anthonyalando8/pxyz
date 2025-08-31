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

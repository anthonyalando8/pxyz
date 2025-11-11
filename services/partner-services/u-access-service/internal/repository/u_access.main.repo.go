package repository

import (
	"context"

	"ptn-rbac-service/internal/domain"
	"x/shared/utils/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RBACRepository interface {
	// Modules
	CreateModules(ctx context.Context, modules []*domain.Module) ([]*domain.Module, []*xerrors.RepoError, error)
	UpdateModule(ctx context.Context, module *domain.Module) error
	GetModuleByCode(ctx context.Context, code string) (*domain.Module, error)
	ListModules(ctx context.Context) ([]*domain.Module, error)

	// Submodules
	CreateSubmodules(ctx context.Context, subs []*domain.Submodule) ([]*domain.Submodule, []*xerrors.RepoError, error)
	UpdateSubmodule(ctx context.Context, sub *domain.Submodule) error
	GetSubmoduleByCode(ctx context.Context, moduleID int64, code string) (*domain.Submodule, error)
	ListSubmodules(ctx context.Context, moduleID int64) ([]*domain.Submodule, error)

	// Roles
	CreateRoles(ctx context.Context, roles []*domain.Role) ([]*domain.Role, []*xerrors.RepoError, error)
	UpdateRole(ctx context.Context, role *domain.Role) error
	ListRoles(ctx context.Context) ([]*domain.Role, error)

	// Permission Types
	CreatePermissionTypes(ctx context.Context, perms []*domain.PermissionType) ([]*domain.PermissionType, []*xerrors.RepoError, error)
	ListPermissionTypes(ctx context.Context) ([]*domain.PermissionType, error)

	// Role Permissions
	AssignRolePermissions(ctx context.Context, perms []*domain.RolePermission) ([]*domain.RolePermission, []*xerrors.RepoError, error)
	ListRolePermissions(ctx context.Context, roleID int64) ([]*domain.RolePermission, error)

	// User Roles
	AssignUserRoles(ctx context.Context, roles []*domain.UserRole) ([]*domain.UserRole, []*xerrors.RepoError, error)
	ListUserRoles(ctx context.Context, userID string) ([]*domain.UserRole, error)
	UpgradeUserRole(ctx context.Context, userID string, newRoleID, assignedBy int64) (*domain.UserRole, error) 

	// User Permission Overrides
	AssignUserPermissionOverrides(ctx context.Context, overrides []*domain.UserPermissionOverride) ([]*domain.UserPermissionOverride, []*xerrors.RepoError, error)
	ListUserPermissionOverrides(ctx context.Context, userID string) ([]*domain.UserPermissionOverride, error)

	// Audit
	LogPermissionEvent(ctx context.Context, audit *domain.PermissionsAudit) error
	ListAuditEvents(ctx context.Context, filter map[string]interface{}) ([]*domain.PermissionsAudit, error)

	// GetEffectivePermissions fetches all permissions for a user across modules and submodules
	GetEffectivePermissions(ctx context.Context, userID string, moduleCode *string, submoduleCode *string) ([]*domain.EffectivePermission, error)
	GetModulesMap(ctx context.Context) (map[string]int64, error)
	GetSubmodulesMap(ctx context.Context) (map[string]int64, error)
	GetUsersWithoutRolesAndClassify(ctx context.Context) ([]*UserRoleAssignment, error)
	BatchAssignRolesToUsers(ctx context.Context, systemUserID int64, roleIDResolver func(ctx context.Context, roleName string) (int64, error)) error
}

type UserRoleAssignment struct {
	UserID   string
	RoleName string
}
// Implementation struct
type rbacRepo struct {
	db *pgxpool.Pool
}

// Factory
func NewRBACRepo(db *pgxpool.Pool) RBACRepository {
	return &rbacRepo{db: db}
}


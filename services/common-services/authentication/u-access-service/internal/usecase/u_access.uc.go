package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"u-rbac-service/internal/domain"
	"u-rbac-service/internal/repository"
	"x/shared/utils/errors"
	"x/shared/utils/id"

		"x/shared/utils/cache"

)

type RBACUsecase struct {
	rbacRepo repository.RBACRepository
	sf       *id.Snowflake
	cache *cache.Cache
}

// NewRBACUsecase initializes the RBACUsecase
func NewRBACUsecase(rbacRepo repository.RBACRepository, sf *id.Snowflake, cache *cache.Cache,) *RBACUsecase {
	return &RBACUsecase{
		rbacRepo: rbacRepo,
		sf:       sf,
		cache: cache,
	}
}

// ------------------------ Modules ------------------------
func (uc *RBACUsecase) CreateModules(ctx context.Context, modules []*domain.Module) ([]*domain.Module, []*xerrors.RepoError, error) {
	now := time.Now().UTC()
	for _, m := range modules {
		idStr := uc.sf.Generate()
		idInt, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("usecase: failed to parse generated ID: %w", err)
		}
		m.ID = idInt
		m.CreatedAt = now
		m.UpdatedAt = &now
	}

	modules, repoErrs, err := uc.rbacRepo.CreateModules(ctx, modules)

	// Invalidate caches
	uc.cache.Delete(ctx,"urbac", "rbac:modules:list")

	return modules, repoErrs, err
}
func (uc *RBACUsecase) UpdateModule(ctx context.Context, module *domain.Module) error {
	module.UpdatedAt = ptrTime(time.Now().UTC())
	err := uc.rbacRepo.UpdateModule(ctx, module)

	// Invalidate caches for this module code and list
	uc.cache.Delete(ctx,"urbac", fmt.Sprintf("rbac:module:code:%s", module.Code))
	uc.cache.Delete(ctx, "urbac", "rbac:modules:list")

	return err
}

func (uc *RBACUsecase) GetModuleByCode(ctx context.Context, code string) (*domain.Module, error) {
	key := fmt.Sprintf("rbac:module:code:%s", code)
	return getOrSetCache(ctx, uc.cache, key, 5*time.Minute, func() (*domain.Module, error) {
		return uc.rbacRepo.GetModuleByCode(ctx, code)
	})
}

func (uc *RBACUsecase) ListModules(ctx context.Context) ([]*domain.Module, error) {
	return getOrSetCache(ctx, uc.cache, "rbac:modules:list", 5*time.Minute, func() ([]*domain.Module, error) {
		return uc.rbacRepo.ListModules(ctx)
	})
}


// ------------------------ Submodules ------------------------
func (uc *RBACUsecase) CreateSubmodules(ctx context.Context, subs []*domain.Submodule) ([]*domain.Submodule, []*xerrors.RepoError, error) {
	now := time.Now().UTC()
	for _, s := range subs {
		idStr := uc.sf.Generate()
		idInt, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("usecase: failed to parse generated ID: %w", err)
		}
		s.ID = idInt
		s.CreatedAt = now
		s.UpdatedAt = &now
	}
	return uc.rbacRepo.CreateSubmodules(ctx, subs)
}

func (uc *RBACUsecase) UpdateSubmodule(ctx context.Context, sub *domain.Submodule) error {
	sub.UpdatedAt = ptrTime(time.Now().UTC())
	return uc.rbacRepo.UpdateSubmodule(ctx, sub)
}

func (uc *RBACUsecase) GetSubmoduleByCode(ctx context.Context, moduleID int64, code string) (*domain.Submodule, error) {
	key := fmt.Sprintf("rbac:submodule:code:%d:%s", moduleID, code)
	return getOrSetCache(ctx, uc.cache, key, 5*time.Minute, func() (*domain.Submodule, error) {
		return uc.rbacRepo.GetSubmoduleByCode(ctx, moduleID, code)
	})
}

func (uc *RBACUsecase) ListSubmodules(ctx context.Context, moduleID int64) ([]*domain.Submodule, error) {
	key := fmt.Sprintf("rbac:submodules:list:%d", moduleID)
	return getOrSetCache(ctx, uc.cache, key, 5*time.Minute, func() ([]*domain.Submodule, error) {
		return uc.rbacRepo.ListSubmodules(ctx, moduleID)
	})
}

// ------------------------ Roles ------------------------
func (uc *RBACUsecase) CreateRoles(ctx context.Context, roles []*domain.Role) ([]*domain.Role, []*xerrors.RepoError, error) {
	now := time.Now().UTC()
	for _, r := range roles {
		idStr := uc.sf.Generate()
		idInt, _ := strconv.ParseInt(idStr, 10, 64)
		r.ID = idInt
		r.CreatedAt = now
		r.UpdatedAt = &now
	}
	return uc.rbacRepo.CreateRoles(ctx, roles)
}

func (uc *RBACUsecase) UpdateRole(ctx context.Context, role *domain.Role) error {
	role.UpdatedAt = ptrTime(time.Now().UTC())
	return uc.rbacRepo.UpdateRole(ctx, role)
}

func (uc *RBACUsecase) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	return getOrSetCache(ctx, uc.cache, "rbac:roles:list", 5*time.Minute, func() ([]*domain.Role, error) {
		return uc.rbacRepo.ListRoles(ctx)
	})
}

// ------------------------ Permission Types ------------------------
func (uc *RBACUsecase) CreatePermissionTypes(ctx context.Context, perms []*domain.PermissionType) ([]*domain.PermissionType, []*xerrors.RepoError, error) {
	now := time.Now().UTC()
	for _, p := range perms {
		idStr := uc.sf.Generate()
		idInt, _ := strconv.ParseInt(idStr, 10, 64)
		p.ID = idInt
		p.CreatedAt = now
		p.UpdatedAt = &now
	}
	return uc.rbacRepo.CreatePermissionTypes(ctx, perms)
}

func (uc *RBACUsecase) ListPermissionTypes(ctx context.Context) ([]*domain.PermissionType, error) {
	return getOrSetCache(ctx, uc.cache, "rbac:permission_types:list", 5*time.Minute, func() ([]*domain.PermissionType, error) {
		return uc.rbacRepo.ListPermissionTypes(ctx)
	})
}

// ------------------------ Role Permissions ------------------------
func (uc *RBACUsecase) AssignRolePermissions(ctx context.Context, perms []*domain.RolePermission) ([]*domain.RolePermission, []*xerrors.RepoError, error) {
	now := time.Now().UTC()
	for _, rp := range perms {
		rp.CreatedAt = now
		rp.UpdatedAt = &now
	}
	return uc.rbacRepo.AssignRolePermissions(ctx, perms)
}

func (uc *RBACUsecase) ListRolePermissions(ctx context.Context, roleID int64) ([]*domain.RolePermission, error) {
	key := fmt.Sprintf("rbac:role_permissions:list:%d", roleID)
	return getOrSetCache(ctx, uc.cache, key, 5*time.Minute, func() ([]*domain.RolePermission, error) {
		return uc.rbacRepo.ListRolePermissions(ctx, roleID)
	})
}


// ------------------------ User Roles ------------------------
func (uc *RBACUsecase) AssignUserRoles(ctx context.Context, roles []*domain.UserRole) ([]*domain.UserRole, []*xerrors.RepoError, error) {
	now := time.Now().UTC()
	for _, r := range roles {
		r.CreatedAt = now
		r.UpdatedAt = &now
	}
	return uc.rbacRepo.AssignUserRoles(ctx, roles)
}

func (uc *RBACUsecase) ListUserRoles(ctx context.Context, userID string) ([]*domain.UserRole, error) {
	key := fmt.Sprintf("rbac:user_roles:list:%s", userID)
	return getOrSetCache(ctx, uc.cache, key, 5*time.Minute, func() ([]*domain.UserRole, error) {
		return uc.rbacRepo.ListUserRoles(ctx, userID)
	})
}

func (uc *RBACUsecase) UpgradeUserRole(ctx context.Context, userID string, newRoleID, assignedBy int64) (*domain.UserRole, error) {
	return uc.rbacRepo.UpgradeUserRole(ctx, userID, newRoleID, assignedBy)
}

// ------------------------ User Permission Overrides ------------------------
func (uc *RBACUsecase) AssignUserPermissionOverrides(ctx context.Context, overrides []*domain.UserPermissionOverride) ([]*domain.UserPermissionOverride, []*xerrors.RepoError, error) {
	now := time.Now().UTC()
	for _, o := range overrides {
		o.CreatedAt = now
		o.UpdatedAt = &now
	}
	return uc.rbacRepo.AssignUserPermissionOverrides(ctx, overrides)
}

func (uc *RBACUsecase) ListUserPermissionOverrides(ctx context.Context, userID string) ([]*domain.UserPermissionOverride, error) {
	key := fmt.Sprintf("rbac:user_permission_overrides:list:%s", userID)
	return getOrSetCache(ctx, uc.cache, key, 5*time.Minute, func() ([]*domain.UserPermissionOverride, error) {
		return uc.rbacRepo.ListUserPermissionOverrides(ctx, userID)
	})
}


// ------------------------ Audit ------------------------
func (uc *RBACUsecase) LogPermissionEvent(ctx context.Context, audit *domain.PermissionsAudit) error {
	audit.CreatedAt = time.Now().UTC()
	return uc.rbacRepo.LogPermissionEvent(ctx, audit)
}

func (uc *RBACUsecase) ListAuditEvents(ctx context.Context, filter map[string]interface{}) ([]*domain.PermissionsAudit, error) {
	return uc.rbacRepo.ListAuditEvents(ctx, filter)
}

// ------------------------ Effective Permissions ------------------------
func (uc *RBACUsecase) GetEffectivePermissions(ctx context.Context, userID string, moduleCode, submoduleCode *string) ([]*domain.EffectivePermission, error) {
	// Compose cache key depending on moduleCode and submoduleCode presence
	key := fmt.Sprintf("rbac:effective_permissions:%s", userID)
	if moduleCode != nil {
		key += fmt.Sprintf(":module:%s", *moduleCode)
	}
	if submoduleCode != nil {
		key += fmt.Sprintf(":submodule:%s", *submoduleCode)
	}

	return getOrSetCache(ctx, uc.cache, key, 15*time.Second, func() ([]*domain.EffectivePermission, error) {
		return uc.rbacRepo.GetEffectivePermissions(ctx, userID, moduleCode, submoduleCode)
	})
}

func (uc *RBACUsecase) CheckUserPermission(ctx context.Context, userID string, moduleID int64, submoduleID int64, permissionTypeID int64) (bool, error) {
	// fetch effective permissions for the user and check if the specific permission is allowed
	perms, err := uc.GetEffectivePermissions(ctx, userID, nil, nil)
	if err != nil {
		return false, err
	}

	for _, p := range perms {
		if p.ModuleID == moduleID && 
		   (p.SubmoduleID == nil || *p.SubmoduleID == submoduleID) &&
		   p.PermissionTypeID == permissionTypeID {
			return p.Allow, nil
		}
	}

	return false, nil
}

func (uc *RBACUsecase) BatchAssignRolesToUnassignedUsers(ctx context.Context, systemUserID int64) error {
	start := time.Now()

	// Call the repo method using the cached role ID resolver
	err := uc.rbacRepo.BatchAssignRolesToUsers(ctx, systemUserID, uc.resolveRoleIDFromCachedList)
	if err != nil {
		return fmt.Errorf("failed to assign default roles: %w", err)
	}

	fmt.Printf("✅ Batch role assignment completed in %s\n", time.Since(start))
	return nil
}

func (uc *RBACUsecase) resolveRoleIDFromCachedList(ctx context.Context, roleName string) (int64, error) {
	roles, err := uc.ListRoles(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list roles: %w", err)
	}

	for _, role := range roles {
		if role.Name == roleName {
			return role.ID, nil
		}
	}
	return 0, fmt.Errorf("role %q not found", roleName)
}

// ------------------------ Helpers ------------------------
func ptrTime(t time.Time) *time.Time {
	return &t
}


func getOrSetCache[T any](ctx context.Context, cache *cache.Cache, key string, ttl time.Duration, fetchFunc func() (T, error)) (T, error) {
	var result T

	// Try to get from Redis
	cached, err := cache.Get(ctx,"urbac", key)
	if err == nil {
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return result, nil
		}
	}

	// Fetch from source (DB)
	result, err = fetchFunc()
	if err != nil {
		return result, err
	}

	// Cache the result
	if data, err := json.Marshal(result); err == nil {
		cache.Set(ctx,"urbac", key, data, ttl)
	}

	return result, nil
}
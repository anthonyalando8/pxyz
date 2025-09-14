package urbacservice

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	rbacpb "x/shared/genproto/urbacpb"

	"github.com/redis/go-redis/v9"
)

type Service struct {
	Client      rbacpb.RBACServiceClient
	RedisClient *redis.Client
}

// NewService creates a new wrapper around RBAC client.
func NewService(client rbacpb.RBACServiceClient, redisClient *redis.Client) *Service {
	return &Service{
		Client:      client,
		RedisClient: redisClient,
	}
}

// AssignRoleByName finds the role ID from name and assigns to user.
func (s *Service) AssignRoleByName(ctx context.Context, userID string, roleName string, assignedBy int64) error {
	cacheKey := "urbac:roles:all"
	const ttl = 5 * time.Minute

	var roles []*rbacpb.Role

	// Step 1: Try Redis cache
	if cached, err := s.RedisClient.Get(ctx, cacheKey).Result(); err == nil {
		if err := json.Unmarshal([]byte(cached), &roles); err == nil && len(roles) > 0 {
			// Cache hit and parse success
		}
	} else if err != redis.Nil {
		// Real Redis error
		return fmt.Errorf("redis error: %w", err)
	}

	// Step 2: If cache missed or invalid, call gRPC
	if len(roles) == 0 {
		resp, err := s.Client.ListRoles(ctx, &emptypb.Empty{})
		if err != nil {
			return fmt.Errorf("failed to list roles: %w", err)
		}
		roles = resp.GetRoles()

		// Store in Redis
		if data, err := json.Marshal(roles); err == nil {
			_ = s.RedisClient.Set(ctx, cacheKey, data, ttl).Err()
		}
	}

	// Step 3: Find role ID by name
	var roleID int64
	for _, r := range roles {
		if r.GetName() == roleName {
			roleID = r.GetId()
			break
		}
	}
	if roleID == 0 {
		return fmt.Errorf("role %s not found", roleName)
	}

	// Step 4: Assign role
	// _, err := s.Client.AssignUserRole(ctx, &rbacpb.AssignUserRoleRequest{
	// 	UserId:     userID,
	// 	RoleId:     roleID,
	// 	AssignedBy: assignedBy,
	// })
	// if err == nil {
	// 	s.invalidateUserRolesCache(ctx, userID)
	// }
	resp, err := s.Client.UpgradeUserRole(ctx, &rbacpb.UpgradeUserRoleRequest{
		UserId:     userID,
		NewRoleId:  roleID,
		AssignedBy: assignedBy,
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade user role: %w", err)
	}

	// ✅ If successful, clear user roles cache
	s.invalidateUserRolesCache(ctx, userID)

	// You can also use resp.Role if needed
	log.Printf("✅ User %s upgraded to role %d", resp.Role.UserId, resp.Role.RoleId)

	return err
}

func (s *Service) GetEffectiveUserPermissions(
	ctx context.Context,
	userID string,
	moduleCode, submoduleCode *string,
) ([]*rbacpb.ModuleWithPermissions, error) {
	// Build cache key
	cacheKey := "urbac:user:effective_perms:" + userID
	if moduleCode != nil {
		cacheKey += ":mod=" + *moduleCode
	}
	if submoduleCode != nil {
		cacheKey += ":subm=" + *submoduleCode
	}
	//const ttl = 5 * time.Minute
	const ttl = 15 * time.Second

	// Try cache first
	cached, err := s.RedisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var modules []*rbacpb.ModuleWithPermissions
		if err := json.Unmarshal([]byte(cached), &modules); err == nil {
			return modules, nil
		}
		// If bad cache, ignore and fetch fresh
	}
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("redis error: %w", err)
	}

	// Fetch fresh from gRPC
	resp, err := s.Client.GetEffectiveUserPermissions(ctx, &rbacpb.GetEffectiveUserPermissionsRequest{
		UserId:        userID,
		ModuleCode:    stringPtrOrEmpty(moduleCode),
		SubmoduleCode: stringPtrOrEmpty(submoduleCode),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch effective permissions for user %s: %w", userID, err)
	}

	modules := resp.GetModules()

	// Cache the result (non-blocking, safe failure)
	if data, err := json.Marshal(modules); err == nil {
		_ = s.RedisClient.Set(ctx, cacheKey, data, ttl).Err()
	}

	return modules, nil
}

func stringPtrOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func (s *Service) CheckUserPermission(
	ctx context.Context,
	userID, moduleCode, permissionCode, submoduleCode string,
) (bool, error) {
	var subPtr *string
	if submoduleCode != "" {
		subPtr = &submoduleCode
	}
	return s.checkUserPermissionInternal(ctx, userID, moduleCode, permissionCode, subPtr)
}


func (s *Service) checkUserPermissionInternal(
	ctx context.Context,
	userID, moduleCode, permissionCode string,
	submoduleCode *string,
) (bool, error) {
	// Fetch effective permissions (cached gRPC call)
	modules, err := s.GetEffectiveUserPermissions(ctx, userID, &moduleCode, submoduleCode)
	if err != nil {
		return false, fmt.Errorf("failed to get effective permissions: %w", err)
	}

	// Find the module
	var targetModule *rbacpb.ModuleWithPermissions
	for _, m := range modules {
		if m.Code == moduleCode {
			targetModule = m
			break
		}
	}
	if targetModule == nil {
		return false, nil // module not found
	}

	// ---- Case 1: No submodule provided → check permission at module-level
	if submoduleCode == nil {
		for _, perm := range targetModule.Permissions {
			if perm.Code == permissionCode {
				return perm.Allowed, nil
			}
		}
		return false, nil
	}

	// ---- Case 2: Submodule provided
	// First, check module-level "can_access"
	moduleAllowed := false
	for _, perm := range targetModule.Permissions {
		if perm.Code == "can_access" && perm.Allowed {
			moduleAllowed = true
			break
		}
	}
	if !moduleAllowed {
		return false, nil // deny if module has no can_access
	}

	// Then check submodule for the requested permissionCode
	for _, sm := range targetModule.Submodules {
		if sm.Code == *submoduleCode {
			for _, perm := range sm.Permissions {
				if perm.Code == permissionCode {
					return perm.Allowed, nil
				}
			}
		}
	}

	// Default: not found
	return false, nil
}



func (s *Service) AssignPermissionByCode(ctx context.Context, userID string, moduleCode, submoduleCode, permissionTypeCode string, allow bool, createdBy int64) error {
	const ttl = 5 * time.Minute

	// Step 1: Module ID (cached)
	moduleKey := "urbac:module:code:" + moduleCode
	moduleID, err := s.getOrSetInt64Cache(ctx, moduleKey, ttl, func() (int64, error) {
		resp, err := s.Client.ListModules(ctx, &rbacpb.ListModulesRequest{})
		if err != nil {
			return 0, err
		}
		for _, m := range resp.GetModules() {
			if m.GetCode() == moduleCode {
				return m.GetId(), nil
			}
		}
		return 0, fmt.Errorf("module %s not found", moduleCode)
	})
	if err != nil {
		return err
	}

	// Step 2: Submodule ID (optional, cached)
	var submoduleID int64
	if submoduleCode != "" {
		submodKey := fmt.Sprintf("urbac:submodule:%d:code:%s", moduleID, submoduleCode)
		submoduleID, err = s.getOrSetInt64Cache(ctx, submodKey, ttl, func() (int64, error) {
			resp, err := s.Client.ListSubmodules(ctx, &rbacpb.ListSubmodulesRequest{
				ModuleId: moduleID,
			})
			if err != nil {
				return 0, err
			}
			for _, sm := range resp.GetSubmodules() {
				if sm.GetCode() == submoduleCode {
					return sm.GetId(), nil
				}
			}
			return 0, fmt.Errorf("submodule %s not found in module %s", submoduleCode, moduleCode)
		})
		if err != nil {
			return err
		}
	}

	// Step 3: Permission Type ID (cached)
	permKey := "urbac:permtype:code:" + permissionTypeCode
	permTypeID, err := s.getOrSetInt64Cache(ctx, permKey, ttl, func() (int64, error) {
		resp, err := s.Client.ListPermissionTypes(ctx, &emptypb.Empty{})
		if err != nil {
			return 0, err
		}
		for _, pt := range resp.GetPermissionTypes() {
			if pt.GetCode() == permissionTypeCode {
				return pt.GetId(), nil
			}
		}
		return 0, fmt.Errorf("permission type %s not found", permissionTypeCode)
	})
	if err != nil {
		return err
	}

	// Step 4: Assign permission override
	_, err = s.Client.AssignUserPermissionOverride(ctx, &rbacpb.AssignUserPermissionOverrideRequest{
		UserId:           userID,
		ModuleId:         moduleID,
		SubmoduleId:      submoduleID,
		PermissionTypeId: permTypeID,
		Allow:            allow,
		CreatedBy:        createdBy,
	})
	if err != nil {
		return fmt.Errorf("failed to assign permission override: %w", err)
	}

	return nil
}

func (s *Service) GetUserRoles(ctx context.Context, userID string) ([]*rbacpb.UserRole, error) {
	cacheKey := "urbac:user:roles:" + userID
	const ttl = 5 * time.Minute

	// Try cache first
	cached, err := s.RedisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var roles []*rbacpb.UserRole
		if err := json.Unmarshal([]byte(cached), &roles); err == nil {
			return roles, nil
		}
		// Log or ignore bad cache value; will fallback to fresh fetch
	}
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("redis error: %w", err)
	}

	// Fetch from gRPC
	resp, err := s.Client.ListUserRoles(ctx, &rbacpb.ListUserRolesRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list user roles for user %s: %w", userID, err)
	}

	roles := resp.GetRoles()

	// Cache the result (non-blocking)
	if data, err := json.Marshal(roles); err == nil {
		_ = s.RedisClient.Set(ctx, cacheKey, data, ttl).Err()
	}

	return roles, nil
}

// AssignPermissionByCode assigns a user permission override using module, submodule, and permission type codes.
func (s *Service) getOrSetInt64Cache(ctx context.Context, key string, ttl time.Duration, fetch func() (int64, error)) (int64, error) {
	val, err := s.RedisClient.Get(ctx, key).Result()
	if err == nil {
		id, convErr := strconv.ParseInt(val, 10, 64)
		if convErr != nil {
			return 0, fmt.Errorf("invalid cached value for %s: %w", key, convErr)
		}
		return id, nil
	}
	if err != redis.Nil {
		return 0, fmt.Errorf("redis error for key %s: %w", key, err)
	}

	// Cache miss — fetch and store
	id, err := fetch()
	if err != nil {
		return 0, err
	}

	err = s.RedisClient.Set(ctx, key, id, ttl).Err()
	if err != nil {
		return 0, fmt.Errorf("failed to set cache for key %s: %w", key, err)
	}

	return id, nil
}

func (s *Service) invalidateUserRolesCache(ctx context.Context, userID string) {
	cacheKey := "urbac:user:roles:" + userID
	_ = s.RedisClient.Del(ctx, cacheKey).Err() // Ignore error
}


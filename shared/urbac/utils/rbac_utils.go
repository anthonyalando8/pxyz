package urbacservice

import (
	"context"
	"encoding/json"
	"fmt"
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
	_, err := s.Client.AssignUserRole(ctx, &rbacpb.AssignUserRoleRequest{
		UserId:     userID,
		RoleId:     roleID,
		AssignedBy: assignedBy,
	})
	if err == nil {
		s.invalidateUserRolesCache(ctx, userID)
	}
	return err
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


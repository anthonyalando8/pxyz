package service

import (
	"context"
	"log"
	"time"
	xerrors "x/shared/utils/errors"

	"ptn-rbac-service/internal/domain"
	"ptn-rbac-service/internal/repository"
)

// RBACSeedService seeds default RBAC modules/submodules
type RBACSeedService struct {
	repo repository.RBACRepository
}

// NewRBACSeedService creates a new instance
func NewRBACSeedService(repo repository.RBACRepository) *RBACSeedService {
	return &RBACSeedService{repo: repo}
}

// SeedDefaults ensures default modules/submodules exist in DB
func (s *RBACSeedService) SeedDefaults(ctx context.Context) error {
	now := time.Now()
	_ = now

	modules, submodules, err := domain.GetDefaultModules()
	if err != nil {
		logError("Failed to get default modules", err)
		return err
	}

	createdModules := []*domain.Module{}
	if len(modules) > 0 {
		createdModules, moduleErrs, err := s.repo.CreateModules(ctx, modules)
		if err != nil {
			logError("Error creating modules", err)
			return err
		}
		logWarnings("Module creation warning", moduleErrs)
		logCreatedModules(createdModules)
	}

	modMap, err := s.mapModules(ctx, createdModules)
	if err != nil {
		log.Printf("⚠️ Warning: proceeding with empty module map due to error: %v", err)
		modMap = make(map[string]int64) // ensure it's not nil to avoid panic
	}

	mapSubmodulesToModules(submodules, modMap)

	validSubmodules := filterValidSubmodules(submodules)

	if len(validSubmodules) > 0 {
		createdSubs, subErrs, err := s.repo.CreateSubmodules(ctx, validSubmodules)
		if err != nil {
			logError("Error creating submodules", err)
			return err
		}
		logWarnings("Submodule creation warning", subErrs)
		if len(createdSubs) == 0 {
			log.Println("⚠️ No submodules were created")
		}
	}

	log.Println("✅ Default RBAC modules and submodules seeded")

	permissions, roles, rolePerms, err := domain.GetDefaultRolesAndPermissions()
	if err != nil {
		logError("Failed to get default roles and permissions", err)
		return err
	}

	createdPerms, permErrs, err := s.repo.CreatePermissionTypes(ctx, permissions)
	if err != nil {
		logError("Error creating permission types", err)
		return err
	}
	logWarnings("Permission creation warning", permErrs)
	permMap := mapPermissions(createdPerms)

	createdRoles, roleErrs, err := s.repo.CreateRoles(ctx, roles)
	if err != nil {
		logError("Error creating roles", err)
		return err
	}
	logWarnings("Role creation warning", roleErrs)
	roleMap := mapRoles(createdRoles)

	subMap, err := s.mapSubmodules(ctx)
	if err != nil {
		log.Printf("⚠️ Warning: proceeding with empty submodule map due to error: %v", err)
		subMap = make(map[string]int64)
	}

	mapRolePermissions(rolePerms, roleMap, permMap, modMap, subMap)

	_, rpErrs, err := s.repo.AssignRolePermissions(ctx, rolePerms)
	if err != nil {
		logError("Error assigning role permissions", err)
		//return err
	}
	logRolePermissionWarnings(rpErrs)

	log.Println("✅ Default RBAC roles, permissions, and role-permissions seeded successfully")
	return nil
}
func logError(message string, err error) {
	log.Printf("❌ %s: %v", message, err)
}

func logWarnings(prefix string, warnings []*xerrors.RepoError) {
	for _, e := range warnings {
		log.Printf("⚠️ %s: %v", prefix, e)
	}
}


func logCreatedModules(modules []*domain.Module) {
	if len(modules) == 0 {
		log.Println("⚠️ No modules were created")
		return
	}
	log.Println("Created modules:")
	for _, m := range modules {
		log.Printf("- %s: ID = %d", m.Code, m.ID)
	}
}

func (s *RBACSeedService) mapModules(ctx context.Context, modules []*domain.Module) (map[string]int64, error) {
	modMap := make(map[string]int64)

	// Build from provided modules
	for _, m := range modules {
		if m.ID != 0 {
			modMap[m.Code] = m.ID
		}
	}

	// Fallback: fetch from DB if empty
	if len(modMap) == 0 {
		log.Println("⚠️ Module ID map is empty, falling back to DB query")
		var err error
		modMap, err = s.repo.GetModulesMap(ctx)
		if err != nil {
			log.Printf("❌ Failed to fetch module ID map from DB: %v", err)
			return nil, err
		}
	}

	return modMap, nil
}

func (s *RBACSeedService) mapSubmodules(ctx context.Context) (map[string]int64, error) {
	subMap := make(map[string]int64)

	subs, err := s.repo.GetSubmodulesMap(ctx)
	if err != nil {
		return nil, err
	}

	for code, id := range subs {
		subMap[code] = id
	}
	return subMap, nil
}


func mapPermissions(perms []*domain.PermissionType) map[string]int64 {
	permMap := make(map[string]int64)
	for _, p := range perms {
		permMap[p.Code] = p.ID
	}
	return permMap
}

func mapRoles(roles []*domain.Role) map[string]int64 {
	roleMap := make(map[string]int64)
	for _, r := range roles {
		roleMap[r.Name] = r.ID
	}
	return roleMap
}

func mapSubmodulesToModules(submodules []*domain.Submodule, modMap map[string]int64) {
	for _, sm := range submodules {
		if sm.ModuleID == 0 {
			if id, ok := modMap[sm.ModuleCode]; ok {
				sm.ModuleID = id
			} else {
				log.Printf("⚠️ Cannot map submodule %s to module code %s", sm.Code, sm.ModuleCode)
			}
		}
	}
}

func filterValidSubmodules(submodules []*domain.Submodule) []*domain.Submodule {
	valid := []*domain.Submodule{}
	for _, sm := range submodules {
		if sm.ModuleID != 0 {
			valid = append(valid, sm)
		} else {
			log.Printf("⚠️ Skipping submodule %s: no ModuleID found for module code %s", sm.Code, sm.ModuleCode)
		}
	}
	return valid
}

func mapRolePermissions(rolePerms []*domain.RolePermission, roleMap, permMap, modMap, subMap map[string]int64) {
	for _, rp := range rolePerms {
		if roleID, ok := roleMap[rp.RoleName]; ok {
			rp.RoleID = roleID
		} else {
			log.Printf("⚠️ Cannot find RoleID for role: %s", rp.RoleName)
		}

		if permID, ok := permMap[rp.PermissionCode]; ok {
			rp.PermissionTypeID = permID
		} else {
			log.Printf("❌ Missing PermissionTypeID for permission code: %s (role: %s, module: %s)",
				rp.PermissionCode, rp.RoleName, rp.ModuleCode)
		}

		if modID, ok := modMap[rp.ModuleCode]; ok {
			rp.ModuleID = modID
		} else {
			log.Printf("⚠️ Cannot map role-permission for role %s to module %s", rp.RoleName, rp.ModuleCode)
		}

		if rp.SubmoduleCode != nil {
			if subID, ok := subMap[*rp.SubmoduleCode]; ok {
				rp.SubmoduleID = &subID
			} else {
				log.Printf("⚠️ Cannot map role-permission for role %s to submodule %s",
					rp.RoleName, *rp.SubmoduleCode)
			}
		}
	}
}



func logRolePermissionWarnings(warnings []*xerrors.RepoError) {
	for _, e := range warnings {
		log.Printf("⚠️ RolePermission assignment warning: role=%s, perm=%s", e.Ref, e.Msg)
	}
}

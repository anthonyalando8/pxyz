package service

import (
	"context"
	"log"
	"time"

	"u-rbac-service/internal/domain"
	"u-rbac-service/internal/repository"
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

	// --- Modules/Submodules ---
	modules, submodules, err := domain.GetDefaultModules()
	if err != nil {
		log.Printf("❌ Failed to get default modules: %v", err)
		return err
	}

	createdModules := []*domain.Module{}
	if len(modules) > 0 {
		createdModules, moduleErrs, err := s.repo.CreateModules(ctx, modules)
		if err != nil {
			log.Printf("❌ Error creating modules: %v", err)
			return err
		}
		for _, e := range moduleErrs {
			log.Printf("⚠️ Module creation warning: %s - %s", e.Code, e.Msg)
		}
		if len(createdModules) == 0 {
			log.Println("⚠️ No modules were created")
		}
	}

	// Map module code → DB ID
	modMap := make(map[string]int64)
	for _, m := range createdModules {
		modMap[m.Code] = m.ID
	}

	// Update submodules' ModuleID
	for _, sm := range submodules {
		if sm.ModuleID == 0 {
			if id, ok := modMap[sm.ModuleCode]; ok {
				sm.ModuleID = id
			} else {
				log.Printf("⚠️ Cannot map submodule %s to module code %s", sm.Code, sm.ModuleCode)
			}
		}
	}

	if len(submodules) > 0 {
		createdSubs, subErrs, err := s.repo.CreateSubmodules(ctx, submodules)
		if err != nil {
			log.Printf("❌ Error creating submodules: %v", err)
			return err
		}
		for _, e := range subErrs {
			log.Printf("⚠️ Submodule creation warning: %s - %s", e.Code, e.Msg)
		}
		if len(createdSubs) == 0 {
			log.Println("⚠️ No submodules were created")
		}
	}

	log.Println("✅ Default RBAC modules and submodules seeded")

	// --- Permissions/Roles ---
	permissions, roles, rolePerms, err := domain.GetDefaultRolesAndPermissions()
	if err != nil {
		log.Printf("❌ Failed to get default roles and permissions: %v", err)
		return err
	}

	// Insert PermissionTypes
	createdPerms, permErrs, err := s.repo.CreatePermissionTypes(ctx, permissions)
	if err != nil {
		log.Printf("❌ Error creating permission types: %v", err)
		return err
	}
	for _, e := range permErrs {
		log.Printf("⚠️ Permission creation warning: %s - %s", e.Code, e.Msg)
	}
	permMap := make(map[string]int64)
	for _, p := range createdPerms {
		permMap[p.Code] = p.ID
	}

	// Insert Roles
	createdRoles, roleErrs, err := s.repo.CreateRoles(ctx, roles)
	if err != nil {
		log.Printf("❌ Error creating roles: %v", err)
		return err
	}
	for _, e := range roleErrs {
		log.Printf("⚠️ Role creation warning: %s - %s", e.Code, e.Msg)
	}
	roleMap := make(map[string]int64)
	for _, r := range createdRoles {
		roleMap[r.Name] = r.ID
	}

	// Assign RolePermissions using DB IDs
	for _, rp := range rolePerms {
		if roleID, ok := roleMap[rp.RoleName]; ok {
			rp.RoleID = roleID
		}
		if permID, ok := permMap[rp.PermissionCode]; ok {
			rp.PermissionTypeID = permID
		}
		if modID, ok := modMap[rp.ModuleCode]; ok {
			rp.ModuleID = modID
		} else {
			log.Printf("⚠️ Cannot map role-permission for role %s to module %s", rp.RoleName, rp.ModuleCode)
		}
	}

	_, rpErrs, err := s.repo.AssignRolePermissions(ctx, rolePerms)
	if err != nil {
		log.Printf("❌ Error assigning role permissions: %v", err)
		return err
	}
	for _, e := range rpErrs {
		log.Printf("⚠️ RolePermission assignment warning: role=%s, perm=%s", e.Ref, e.Msg)
	}

	log.Println("✅ Default RBAC roles, permissions, and role-permissions seeded successfully")
	return nil
}

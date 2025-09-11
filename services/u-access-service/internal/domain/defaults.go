package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// ModuleMeta holds extra info about a module
type ModuleMeta struct {
	URL   *string        `json:"url,omitempty"`
	Ports []map[string]any `json:"ports,omitempty"`
}


// GetDefaultModules returns default modules and submodules for seeding
func GetDefaultModules() ([]*Module, []*Submodule, error) {
	now := time.Now()
	createdBy := int64(0)

	// helper to marshal metadata
	marshalMeta := func(url *string, ports []map[string]any) []byte {
		m := ModuleMeta{URL: url, Ports: ports}
		b, _ := json.Marshal(m)
		return b
	}

	// Define modules
	modules := []*Module{
		{Code: "account", Name: "Account", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "auth", Name: "Auth", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "cashier", Name: "Cashier", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "wallet", Name: "Wallet", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "core", Name: "Core", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "email", Name: "Email", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "kyc", Name: "KYC", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "otp", Name: "OTP", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "session", Name: "Session", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "notification", Name: "Notification", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
		{Code: "urbac", Name: "RBAC", IsActive: true, CreatedAt: now, CreatedBy: createdBy, Meta: marshalMeta(nil, nil)},
	}

	// Map modules to submodules in one place
	submodules := []*Submodule{}
	defaultSubs := map[string][]string{
		"auth":         {"login", "register", "change_password", "change_email", "change_phone"},
		"cashier":      {"wallet"},
		"notification": {"sms", "email"},
	}

	for modCode, subs := range defaultSubs {
		modExists := false
		for _, m := range modules {
			if m.Code == modCode {
				modExists = true
				for _, s := range subs {
					submodules = append(submodules, &Submodule{
						ModuleCode: modCode,
						Code:       s,
						Name:       s, // can adjust display name if needed
						IsActive:   true,
						CreatedAt:  now,
						CreatedBy:  createdBy,
					})
				}
			}
		}
		if !modExists {
			return nil, nil, fmt.Errorf("module %s not defined for submodules", modCode)
		}
	}

	return modules, submodules, nil
}

func GetDefaultRolesAndPermissions() ([]*PermissionType, []*Role, []*RolePermission, error) {
	modules, submodules, err := GetDefaultModules()
	if err != nil {
		return nil, nil, nil, err
	}

	now := time.Now()
	createdBy := int64(0)

	// Default permission types
	permissions := []*PermissionType{
		{Code: "can_access", Description: "Can access module/submodule", IsActive: true, CreatedAt: now, CreatedBy: createdBy},
		{Code: "can_delete", Description: "Can delete resources", IsActive: true, CreatedAt: now, CreatedBy: createdBy},
		{Code: "can_edit", Description: "Can edit resources", IsActive: true, CreatedAt: now, CreatedBy: createdBy},
		{Code: "can_create", Description: "Can create resources", IsActive: true, CreatedAt: now, CreatedBy: createdBy},
	}

	roles := []*Role{
		{Name: "trader", Description: "A fully registered trader", IsActive: true, CreatedAt: now, CreatedBy: createdBy},
		{Name: "kyc_unverified", Description: "User who has not completed KYC", IsActive: true, CreatedAt: now, CreatedBy: createdBy},
		{Name: "any", Description: "A registering user (account creation)", IsActive: true, CreatedAt: now, CreatedBy: createdBy},
	}

	// Build module → submodule map
	modSubMap := map[string][]*Submodule{}
	for _, s := range submodules {
		modSubMap[s.ModuleCode] = append(modSubMap[s.ModuleCode], s)
	}

	var rolePermissions []*RolePermission

	for _, r := range roles {
		for _, m := range modules {
			assign := false
			switch r.Name {
			case "trader":
				assign = true
			case "any":
				if m.Code == "auth" {
					assign = true
				}
			case "kyc_unverified":
				if m.Code == "auth" || m.Code == "account" {
					assign = true
				}
			}
			if !assign {
				continue
			}

			for _, p := range permissions {
				// Module-level permission
				rolePermissions = append(rolePermissions, &RolePermission{
					RoleName:       r.Name,
					ModuleCode:     m.Code,
					PermissionCode: p.Code,
					Allow:          true,
					CreatedAt:      now,
					CreatedBy:      createdBy,
				})

				// Submodule-level permission
				if subs, ok := modSubMap[m.Code]; ok {
					for _, sm := range subs {
						sub := sm.Code
						rolePermissions = append(rolePermissions, &RolePermission{
							RoleName:       r.Name,
							ModuleCode:     m.Code,
							SubmoduleCode:  &sub,
							PermissionCode: p.Code,
							Allow:          true,
							CreatedAt:      now,
							CreatedBy:      createdBy,
						})
					}
				}
			}
		}
	}

	return permissions, roles, rolePermissions, nil
}

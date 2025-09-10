package domain

import "time"

type EffectivePermissionFilter struct {
	UserID        string
	ModuleCode    *string // optional: only this module
	SubmoduleCode *string // optional: only this submodule
}

type EffectivePermission struct {
	ModuleID     int64
	ModuleCode   string
	ModuleName   string
	ModuleActive bool

	SubmoduleID     *int64
	SubmoduleCode   *string
	SubmoduleName   *string
	SubmoduleActive *bool

	PermissionTypeID int64
	PermissionCode   string
	PermissionName   string
	Allow            bool // final allow after user override

	UserID    string
	RoleID    *int64 // nil if permission comes only from override
	CreatedAt time.Time
	UpdatedAt *time.Time
}


type ModuleWithPermissions struct {
	ID         int64                       `json:"id"`
	Code       string                      `json:"code"`
	Name       string                      `json:"name"`
	IsActive   bool                        `json:"is_active"`
	Permissions []*PermissionInfo          `json:"permissions"`   // ✅ module-level perms
	Submodules  []*SubmoduleWithPermissions `json:"submodules"`
}

type SubmoduleWithPermissions struct {
	ID          *int64            `json:"id,omitempty"`
	Code        *string           `json:"code,omitempty"`
	Name        *string           `json:"name,omitempty"`
	IsActive    *bool             `json:"is_active,omitempty"`
	Permissions []*PermissionInfo `json:"permissions"`
}

type PermissionInfo struct {
	ID      int64   `json:"id"`
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Allowed bool    `json:"allowed"`
	RoleID  *int64  `json:"role_id,omitempty"`
	UserID  string  `json:"user_id"`
}

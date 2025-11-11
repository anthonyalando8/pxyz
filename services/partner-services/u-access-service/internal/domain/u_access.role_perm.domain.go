package domain

import "time"

// RolePermission assigns permission types to a role
type RolePermission struct {
	ID               int64      `json:"id"`
	RoleID           int64      `json:"role_id"`
	ModuleID         int64      `json:"module_id"`
	SubmoduleID      *int64     `json:"submodule_id,omitempty"`
	PermissionTypeID int64      `json:"permission_type_id"`
	Allow            bool       `json:"allow"`
	CreatedAt        time.Time  `json:"created_at"`
	CreatedBy        int64      `json:"created_by"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	UpdatedBy        *int64     `json:"updated_by,omitempty"`

	RoleName       string  `json:"-"` // ignore in DB
	PermissionCode string  `json:"-"`
	SubmoduleCode  *string `json:"-"`
	ModuleCode     string  `json:"-"`
}

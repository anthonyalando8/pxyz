package domain

import "time"

// UserPermissionOverride overrides permissions at user-level
type UserPermissionOverride struct {
	ID               int64      `json:"id"`
	UserID           string      `json:"user_id"`
	ModuleID         int64      `json:"module_id"`
	SubmoduleID      *int64     `json:"submodule_id,omitempty"`
	PermissionTypeID int64      `json:"permission_type_id"`
	Allow            bool       `json:"allow"`
	CreatedAt        time.Time  `json:"created_at"`
	CreatedBy        int64      `json:"created_by"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	UpdatedBy        *int64     `json:"updated_by,omitempty"`
}

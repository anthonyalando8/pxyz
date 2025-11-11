package domain

import "time"

// PermissionType defines actions like can_access, can_edit, etc.
type PermissionType struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	Description string     `json:"description"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   int64      `json:"created_by"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	UpdatedBy   *int64     `json:"updated_by,omitempty"`
}

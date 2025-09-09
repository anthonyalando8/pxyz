package domain

import "time"

// UserRole assigns a role to a user
type UserRole struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	RoleID    int64      `json:"role_id"`
	AssignedBy int64     `json:"assigned_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	UpdatedBy *int64     `json:"updated_by,omitempty"`
}


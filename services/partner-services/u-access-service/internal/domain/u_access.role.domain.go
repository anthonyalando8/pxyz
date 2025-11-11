package domain

import "time"

// Role groups permissions
type Role struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   int64      `json:"created_by"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	UpdatedBy   *int64     `json:"updated_by,omitempty"`
}

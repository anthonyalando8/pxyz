package domain

import "time"

// Submodule belongs to a module
type Submodule struct {
	ID        int64      `json:"id"`
	ModuleID  int64      `json:"module_id"`
	ModuleCode string `json:"-"` // only for seeding
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Meta      []byte     `json:"meta,omitempty"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy int64      `json:"created_by"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	UpdatedBy *int64     `json:"updated_by,omitempty"`
}

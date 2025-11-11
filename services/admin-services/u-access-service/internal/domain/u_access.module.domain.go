package domain

import "time"

// Module represents a top-level or nested module
type Module struct {
	ID        int64      `json:"id"`
	ParentID  *int64     `json:"parent_id,omitempty"` // nullable
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Meta      []byte     `json:"meta,omitempty"` // JSONB as []byte
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy int64      `json:"created_by"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	UpdatedBy *int64     `json:"updated_by,omitempty"`
}

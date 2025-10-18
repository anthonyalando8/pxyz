package domain

import "time"

// Journal represents a transaction header
type Journal struct {
    ID             int64     `json:"id"`
    ExternalRef    string    `json:"external_ref"`    // e.g., external payment reference
    IdempotencyKey string    `json:"idempotency_key"` // to prevent double posting
    Description    string    `json:"description"`
    CreatedBy      int64     `json:"created_by"`
    CreatedByType  string    `json:"created_by_type"` // system, partner, user, admin
    CreatedAt      time.Time `json:"created_at"`
}

package domain

import (
	"time"
)

type PartnerStatus string

const (
	PartnerStatusActive    PartnerStatus = "active"
	PartnerStatusSuspended PartnerStatus = "suspended"
)

type Partner struct {
	ID           string          `json:"id"`
	Name         string         `json:"name"`
	Country      string         `json:"country,omitempty"`
	ContactEmail string         `json:"contact_email,omitempty"`
	ContactPhone string         `json:"contact_phone,omitempty"`
	Status       PartnerStatus  `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

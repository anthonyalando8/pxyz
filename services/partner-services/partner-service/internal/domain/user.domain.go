package domain

import "time"

type PartnerUserRole string

const (
	PartnerUserRoleAdmin PartnerUserRole = "partner_admin"
	PartnerUserRoleUser  PartnerUserRole = "partner_user"
)

type PartnerUser struct {
	ID           string           `json:"id"`
	PartnerID    string           `json:"partner_id"`
	Role         PartnerUserRole `json:"role"`
	Email        string          `json:"email"`
	UserID       string          `json:"user_id"`
	IsActive     bool            `json:"is_active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

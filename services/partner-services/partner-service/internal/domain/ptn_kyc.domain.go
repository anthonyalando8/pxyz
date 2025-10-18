package domain

import (
	"time"
	"encoding/json"
)

type PartnerKYCStatus string

const (
	PartnerKYCStatusPending  PartnerKYCStatus = "pending"
	PartnerKYCStatusApproved PartnerKYCStatus = "approved"
	PartnerKYCStatusRejected PartnerKYCStatus = "rejected"
)

type PartnerKYC struct {
	PartnerID string           `json:"partner_id"`
	Status    PartnerKYCStatus `json:"status"`
	KYCData   json.RawMessage `json:"kyc_data,omitempty"`
	Limits    json.RawMessage `json:"limits,omitempty"`
	RiskFlags json.RawMessage `json:"risk_flags,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

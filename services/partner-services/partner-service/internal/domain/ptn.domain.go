package domain

import (
	"time"

	"x/shared/genproto/partner/svcpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PartnerStatus string

const (
	PartnerStatusActive    PartnerStatus = "active"
	PartnerStatusSuspended PartnerStatus = "suspended"
)
type Partner struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Country      string         `json:"country,omitempty"`
	ContactEmail string         `json:"contact_email,omitempty"`
	ContactPhone string         `json:"contact_phone,omitempty"`
	Status       PartnerStatus  `json:"status"`
	Service      string         `json:"service,omitempty"`   // new field
	Currency     string         `json:"currency,omitempty"`  // new field
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ToProto converts domain.Partner to gRPC Partner message
func (p *Partner) ToProto() *partnersvcpb.Partner {
	if p == nil {
		return nil
	}

	return &partnersvcpb.Partner{
		Id:           p.ID,
		Name:         p.Name,
		Country:      p.Country,
		ContactEmail: p.ContactEmail,
		ContactPhone: p.ContactPhone,
		Status:       string(p.Status), // convert enum type to string
		Service:      p.Service,        // new field
		Currency:     p.Currency,       // new field
		CreatedAt:    timestamppb.New(p.CreatedAt),
		UpdatedAt:    timestamppb.New(p.UpdatedAt),
	}
}



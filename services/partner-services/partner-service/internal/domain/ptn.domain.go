// domain/partner.go
package domain

import (
	"time"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)
type PartnerStatus string

const (
	PartnerStatusActive   PartnerStatus = "active"
	PartnerStatusInactive PartnerStatus = "inactive"
	PartnerStatusSuspended PartnerStatus = "suspended"
)


type Partner struct {
    ID             string
    Name           string
    Country        string
    ContactEmail   string
    ContactPhone   string
    Status         PartnerStatus
    Service        string
    Currency       string
    LocalCurrency  string
    Rate           float64
    InverseRate    float64
    CommissionRate float64
    
    IsAPIEnabled   bool
    // API Integration fields
	APIKey          *string
	APISecretHash   *string
	PlainAPISecret  *string // Temporary, for notification purposes only
	WebhookURL      *string
	WebhookSecret   *string
	APIRateLimit   int32
    CallbackURL    *string
    AllowedIPs     []string
    Metadata        map[string]interface{}
    CreatedAt      time.Time
    UpdatedAt      time.Time
}


type PartnerAPILog struct {
	ID            int64
	PartnerID     string
	Endpoint      string
	Method        string
	RequestBody   map[string]interface{}
	ResponseBody  map[string]interface{}
	StatusCode    int
	IPAddress     string
	UserAgent     string
	LatencyMs     int
	ErrorMessage  *string
	CreatedAt     time.Time
}

type PartnerWebhook struct {
	ID             int64
	PartnerID      string
	EventType      string
	Payload        []byte
	Status         string
	Attempts       int
	MaxAttempts    int
	LastAttemptAt  *time.Time
	NextRetryAt    *time.Time
	ResponseStatus *int
	ResponseBody   *string
	ErrorMessage   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PartnerTransaction struct {
	ID             int64
	PartnerID      string
	TransactionRef string
	UserID         string
	Amount         float64
	Currency       string
	Status         string
	PaymentMethod  *string
	TransactionType  string
	ExternalRef    *string
	Metadata       map[string]interface{}
	ErrorMessage   string                 `json:"error_message,omitempty"`
	ProcessedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (p *Partner) ToProto() *partnersvcpb.Partner {
	proto := &partnersvcpb.Partner{
		Id:           p.ID,
		Name:         p.Name,
		Country:      p.Country,
		ContactEmail: p.ContactEmail,
		ContactPhone: p.ContactPhone,
		Status:       string(p.Status),
		Service:      p.Service,
		Currency:     p.Currency,
		IsApiEnabled: p.IsAPIEnabled,
		ApiRateLimit: int32(p.APIRateLimit),
		CreatedAt:    timestamppb.New(p.CreatedAt),
		UpdatedAt:    timestamppb.New(p.UpdatedAt),
	}
	
	if p.WebhookURL != nil {
		proto.WebhookUrl = *p.WebhookURL
	}
	if p.CallbackURL != nil {
		proto.CallbackUrl = *p.CallbackURL
	}
	if len(p.AllowedIPs) > 0 {
		proto.AllowedIps = p.AllowedIPs
	}
	
	return proto
}
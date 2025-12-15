// domain/partner. go
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
	LocalCurrency  string    // ✅ Added
	Rate           float64   // ✅ Added
	InverseRate    float64   // ✅ Added
	CommissionRate float64   // ✅ Added
	
	// API Integration fields
	APIKey         *string
	APISecretHash  *string
	PlainAPISecret *string // Temporary, for notification purposes only
	WebhookURL     *string
	WebhookSecret  *string
	CallbackURL    *string
	IsAPIEnabled   bool
	APIRateLimit   int32
	AllowedIPs     []string
	Metadata       map[string]interface{}
	
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
	LastAttemptAt  *time. Time
	NextRetryAt    *time.Time
	ResponseStatus *int
	ResponseBody   *string
	ErrorMessage   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PartnerTransaction struct {
	ID              int64
	PartnerID       string
	TransactionRef  string
	UserID          string
	Amount          float64
	Currency        string
	Status          string
	PaymentMethod   *string
	TransactionType string
	ExternalRef     *string
	Metadata        map[string]interface{}
	ErrorMessage    *string `json:"error_message,omitempty"`
	ProcessedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ✅ Complete ToProto conversion
func (p *Partner) ToProto() *partnersvcpb.Partner {
	proto := &partnersvcpb.Partner{
		Id:             p. ID,
		Name:           p.Name,
		Country:        p.Country,
		ContactEmail:   p.ContactEmail,
		ContactPhone:   p.ContactPhone,
		Status:         string(p.Status),
		Service:        p.Service,
		Currency:       p.Currency,
		LocalCurrency:  p. LocalCurrency,     // ✅ Added
		Rate:           p.Rate,              // ✅ Added
		InverseRate:    p. InverseRate,       // ✅ Added
		CommissionRate: p.CommissionRate,    // ✅ Added
		IsApiEnabled:   p.IsAPIEnabled,
		ApiRateLimit:   p.APIRateLimit,
		CreatedAt:      timestamppb.New(p.CreatedAt),
		UpdatedAt:      timestamppb.New(p.UpdatedAt),
	}
	
	// ✅ Handle optional pointer fields
	if p.APIKey != nil {
		proto.ApiKey = *p.APIKey
	}
	if p.WebhookURL != nil {
		proto.WebhookUrl = *p.WebhookURL
	}
	if p. CallbackURL != nil {
		proto.CallbackUrl = *p.CallbackURL
	}
	if len(p.AllowedIPs) > 0 {
		proto.AllowedIps = p.AllowedIPs
	}
	
	return proto
}

// ✅ FromProto conversion (optional but recommended)
func PartnerFromProto(proto *partnersvcpb.Partner) *Partner {
	if proto == nil {
		return nil
	}

	partner := &Partner{
		ID:              proto.Id,
		Name:           proto.Name,
		Country:        proto.Country,
		ContactEmail:   proto.ContactEmail,
		ContactPhone:   proto.ContactPhone,
		Status:         PartnerStatus(proto.Status),
		Service:        proto.Service,
		Currency:       proto.Currency,
		LocalCurrency:  proto. LocalCurrency,
		Rate:           proto.Rate,
		InverseRate:    proto.InverseRate,
		CommissionRate: proto.CommissionRate,
		IsAPIEnabled:   proto.IsApiEnabled,
		APIRateLimit:   proto.ApiRateLimit,
		AllowedIPs:     proto. AllowedIps,
	}

	// Handle optional fields
	if proto.ApiKey != "" {
		partner.APIKey = &proto.ApiKey
	}
	if proto.WebhookUrl != "" {
		partner.WebhookURL = &proto.WebhookUrl
	}
	if proto.CallbackUrl != "" {
		partner.CallbackURL = &proto.CallbackUrl
	}

	// Handle timestamps
	if proto.CreatedAt != nil {
		partner.CreatedAt = proto.CreatedAt.AsTime()
	}
	if proto. UpdatedAt != nil {
		partner.UpdatedAt = proto. UpdatedAt.AsTime()
	}

	return partner
}

// ✅ ToProto conversion for PartnerTransaction
func (t *PartnerTransaction) ToProto() *partnersvcpb.PartnerTransaction {
	proto := &partnersvcpb.PartnerTransaction{
		Id:              t.ID,
		PartnerId:       t.PartnerID,
		TransactionRef:  t.TransactionRef,
		UserId:          t.UserID,
		Amount:          t.Amount,
		Currency:        t. Currency,
		Status:          t.Status,
		TransactionType: t.TransactionType,
		ErrorMessage:    *t.ErrorMessage,
		CreatedAt:       timestamppb.New(t.CreatedAt),
		UpdatedAt:       timestamppb.New(t.UpdatedAt),
	}

	// Handle optional fields
	if t.PaymentMethod != nil {
		proto.PaymentMethod = *t.PaymentMethod
	}
	if t.ExternalRef != nil {
		proto.ExternalRef = *t. ExternalRef
	}
	if t.ProcessedAt != nil {
		proto.ProcessedAt = timestamppb.New(*t.ProcessedAt)
	}

	// Convert metadata
	if t.Metadata != nil {
		proto.Metadata = make(map[string]string)
		for k, v := range t.Metadata {
			if str, ok := v.(string); ok {
				proto.Metadata[k] = str
			}
		}
	}

	return proto
}

// ✅ FromProto conversion for PartnerTransaction
func PartnerTransactionFromProto(proto *partnersvcpb.PartnerTransaction) *PartnerTransaction {
	if proto == nil {
		return nil
	}

	txn := &PartnerTransaction{
		ID:              proto.Id,
		PartnerID:       proto.PartnerId,
		TransactionRef:  proto.TransactionRef,
		UserID:          proto. UserId,
		Amount:          proto.Amount,
		Currency:        proto.Currency,
		Status:          proto.Status,
		TransactionType: proto.TransactionType,
		ErrorMessage:    &proto.ErrorMessage,
	}

	// Handle optional fields
	if proto.PaymentMethod != "" {
		txn.PaymentMethod = &proto.PaymentMethod
	}
	if proto.ExternalRef != "" {
		txn.ExternalRef = &proto.ExternalRef
	}

	// Convert metadata
	if proto.Metadata != nil {
		txn.Metadata = make(map[string]interface{})
		for k, v := range proto.Metadata {
			txn.Metadata[k] = v
		}
	}

	// Handle timestamps
	if proto.ProcessedAt != nil {
		processedAt := proto.ProcessedAt.AsTime()
		txn.ProcessedAt = &processedAt
	}
	if proto.CreatedAt != nil {
		txn.CreatedAt = proto.CreatedAt.AsTime()
	}
	if proto.UpdatedAt != nil {
		txn.UpdatedAt = proto.UpdatedAt.AsTime()
	}

	return txn
}
package domain

import (
	"time"
	"encoding/json"
)

type PartnerConfig struct {
	PartnerID       string           `json:"partner_id"`
	DefaultFXSpread float64         `json:"default_fx_spread"`
	WebhookSecret   string          `json:"webhook_secret,omitempty"`
	ConfigData      json.RawMessage `json:"config_data,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

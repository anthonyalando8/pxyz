package domain

import (
	"time"
	"encoding/json"
)

type PartnerActorType string

const (
	ActorSystem      PartnerActorType = "system"
	ActorPartnerUser PartnerActorType = "partner_user"
	ActorPartner     PartnerActorType = "partner"
)

type PartnerAuditLog struct {
	ID         int64            `json:"id"`
	ActorType  PartnerActorType `json:"actor_type"`
	ActorID    int64            `json:"actor_id,omitempty"`
	Action     string           `json:"action"`
	TargetType string           `json:"target_type,omitempty"`
	TargetID   int64            `json:"target_id,omitempty"`
	Metadata   json.RawMessage  `json:"metadata,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

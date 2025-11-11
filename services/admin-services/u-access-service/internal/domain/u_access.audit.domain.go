package domain

import "time"

// PermissionsAudit logs permission-related actions
type PermissionsAudit struct {
	ID        int64     `json:"id"`
	ActorID   int64     `json:"actor_id"`
	ObjectType string   `json:"object_type"`
	ObjectID  int64     `json:"object_id"`
	Action    string    `json:"action"`
	Payload   []byte    `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

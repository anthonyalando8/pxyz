package ws

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

type AuthEventPublisher struct {
	rdb *redis.Client
}

func NewAuthEventPublisher(rdb *redis.Client) *AuthEventPublisher {
	return &AuthEventPublisher{rdb: rdb}
}

// Publish sends an auth event (e.g., logout, role_update) to Redis.
func (p *AuthEventPublisher) Publish(ctx context.Context, eventType, userID, deviceID string, data interface{}) error {
	event := Message{
		Type:     eventType,
		UserID:   userID,
		DeviceID: deviceID,
		Data:     data,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := p.rdb.Publish(ctx, "auth_events", payload).Err(); err != nil {
		log.Printf("[WARN] failed to publish %s event for user %s: %v", eventType, userID, err)
		return err
	}

	return nil
}

package ws

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

func ListenAuthEvents(ctx context.Context, rdb *redis.Client, hub *Hub) {
	sub := rdb.Subscribe(ctx, "auth_events")
	ch := sub.Channel()

	for msg := range ch {
		var event Message
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Println("Error parsing auth event:", err)
			continue
		}
		hub.broadcast <- event
	}
}

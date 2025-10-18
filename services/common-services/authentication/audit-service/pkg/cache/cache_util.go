package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.ClusterClient
}

func NewCache(addrs []string, password string) *Cache {
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    addrs, // e.g. []string{"127.0.0.1:7000","127.0.0.1:7001"}
		Password: password,
	})
	return &Cache{client: rdb}
}

// Set with namespace (simulate table)
func (c *Cache) Set(ctx context.Context, namespace, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, namespace+":"+key, value, ttl).Err()
}

// Get with namespace
func (c *Cache) Get(ctx context.Context, namespace, key string) (string, error) {
	return c.client.Get(ctx, namespace+":"+key).Result()
}

// Delete
func (c *Cache) Delete(ctx context.Context, namespace, key string) error {
	return c.client.Del(ctx, namespace+":"+key).Err()
}

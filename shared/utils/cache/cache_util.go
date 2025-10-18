package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client redis.UniversalClient // works with both single and cluster
}

func NewCache(addrs []string, password string, useCluster bool) *Cache {
	var rdb redis.UniversalClient

	if useCluster && len(addrs) > 1 {
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    addrs,
			Password: password,
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:     addrs[0],
			Password: password,
			DB:       0,
		})
	}

	return &Cache{client: rdb}
}

func (c *Cache) Set(ctx context.Context, namespace, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, namespace+":"+key, value, ttl).Err()
}

func (c *Cache) Get(ctx context.Context, namespace, key string) (string, error) {
	return c.client.Get(ctx, namespace+":"+key).Result()
}

func (c *Cache) Delete(ctx context.Context, namespace, key string) error {
	return c.client.Del(ctx, namespace+":"+key).Err()
}

func (c *Cache) GetTTL(ctx context.Context, namespace, key string) (time.Duration, error) {
	return c.client.TTL(ctx, namespace+":"+key).Result()
}

func (c *Cache) IncrWithExpire(ctx context.Context, namespace, key string, window time.Duration) (int64, error) {
	countKey := namespace + ":" + key

	cnt, err := c.client.Incr(ctx, countKey).Result()
	if err != nil {
		return 0, err
	}

	// If it's the first time the key is incremented, set its TTL
	if cnt == 1 {
		_ = c.client.Expire(ctx, countKey, window).Err()
	}

	return cnt, nil
}

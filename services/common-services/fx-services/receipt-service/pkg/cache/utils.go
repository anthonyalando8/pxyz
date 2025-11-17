package cache

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NewCacheServiceFromClient creates a CacheService from an existing Redis client
// This is a convenience wrapper for integration with existing code
func NewCacheServiceFromClient(client *redis.Client, logger *zap.Logger) *CacheService {
	return &CacheService{
		client: client,
		logger: logger,
		hits:   0,
		misses: 0,
	}
}
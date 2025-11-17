package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CacheService handles all caching operations for receipts
type CacheService struct {
	client *redis.Client
	logger *zap.Logger
	
	// Metrics
	hits   int64
	misses int64
}

// NewCacheService creates a new cache service
func NewCacheService(addr, password string, db int, logger *zap.Logger) (*CacheService, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            addr,
		Password:        password,
		DB:              db,
		PoolSize:        100,        // Connection pool size
		MinIdleConns:    10,         // Minimum idle connections
		PoolTimeout:     4 * time.Second,
		//IdleTimeout:     5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		ConnMaxLifetime: 30 * time.Minute,
		
		// Retry configuration
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	
	logger.Info("redis connected", zap.String("addr", addr))
	
	return &CacheService{
		client: client,
		logger: logger,
	}, nil
}

// ===============================
// Receipt Caching
// ===============================

// CacheKey formats a cache key for a receipt
func CacheKey(code string) string {
	return fmt.Sprintf("receipt:v1:%s", code)
}

// IdempotencyKey formats an idempotency cache key
func IdempotencyKey(key string) string {
	return fmt.Sprintf("idem:v1:%s", key)
}

// GetReceipt retrieves a cached receipt
func (c *CacheService) GetReceipt(ctx context.Context, code string) ([]byte, error) {
	key := CacheKey(code)
	
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			c.misses++
			return nil, nil // Cache miss (not an error)
		}
		return nil, fmt.Errorf("cache get: %w", err)
	}
	
	c.hits++
	return data, nil
}

// SetReceipt caches a receipt with TTL
func (c *CacheService) SetReceipt(ctx context.Context, code string, data []byte, ttl time.Duration) error {
	key := CacheKey(code)
	return c.client.Set(ctx, key, data, ttl).Err()
}

// DeleteReceipt removes a receipt from cache
func (c *CacheService) DeleteReceipt(ctx context.Context, code string) error {
	key := CacheKey(code)
	return c.client.Del(ctx, key).Err()
}

// ===============================
// Batch Operations (High Performance)
// ===============================

// GetReceiptsBatch retrieves multiple receipts using pipeline (10x faster)
func (c *CacheService) GetReceiptsBatch(ctx context.Context, codes []string) (map[string][]byte, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	
	// Use pipeline for parallel execution
	pipe := c.client.Pipeline()
	
	// Queue all GET commands
	cmds := make(map[string]*redis.StringCmd, len(codes))
	for _, code := range codes {
		key := CacheKey(code)
		cmds[code] = pipe.Get(ctx, key)
	}
	
	// Execute all at once
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("pipeline exec: %w", err)
	}
	
	// Collect results
	results := make(map[string][]byte, len(codes))
	for code, cmd := range cmds {
		if data, err := cmd.Bytes(); err == nil {
			results[code] = data
			c.hits++
		} else {
			c.misses++
		}
	}
	
	return results, nil
}

// SetReceiptsBatch caches multiple receipts using pipeline
func (c *CacheService) SetReceiptsBatch(ctx context.Context, receipts map[string][]byte, ttl time.Duration) error {
	if len(receipts) == 0 {
		return nil
	}
	
	pipe := c.client.Pipeline()
	
	for code, data := range receipts {
		key := CacheKey(code)
		pipe.Set(ctx, key, data, ttl)
	}
	
	_, err := pipe.Exec(ctx)
	return err
}

// DeleteReceiptsBatch removes multiple receipts using pipeline
func (c *CacheService) DeleteReceiptsBatch(ctx context.Context, codes []string) error {
	if len(codes) == 0 {
		return nil
	}
	
	pipe := c.client.Pipeline()
	
	for _, code := range codes {
		key := CacheKey(code)
		pipe.Del(ctx, key)
	}
	
	_, err := pipe.Exec(ctx)
	return err
}

// ===============================
// Idempotency Support
// ===============================

// GetIdempotent retrieves an idempotency cached result
func (c *CacheService) GetIdempotent(ctx context.Context, idemKey string) ([]byte, error) {
	key := IdempotencyKey(idemKey)
	
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("get idempotent: %w", err)
	}
	
	c.logger.Info("idempotency cache hit", zap.String("key", idemKey))
	return data, nil
}

// SetIdempotent caches a result for idempotency (24 hour TTL)
func (c *CacheService) SetIdempotent(ctx context.Context, idemKey string, data []byte) error {
	key := IdempotencyKey(idemKey)
	return c.client.Set(ctx, key, data, 24*time.Hour).Err()
}

// ===============================
// Distributed Locking
// ===============================

// Lock represents a distributed lock
type Lock struct {
	client *redis.Client
	key    string
	token  string
	ttl    time.Duration
}

// AcquireLock attempts to acquire a distributed lock
func (c *CacheService) AcquireLock(ctx context.Context, resourceID string, ttl time.Duration) (*Lock, error) {
	key := fmt.Sprintf("lock:%s", resourceID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	
	// SET key value NX EX ttl (atomic)
	success, err := c.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	
	if !success {
		return nil, fmt.Errorf("lock already held by another process")
	}
	
	c.logger.Debug("lock acquired", 
		zap.String("resource", resourceID),
		zap.Duration("ttl", ttl),
	)
	
	return &Lock{
		client: c.client,
		key:    key,
		token:  token,
		ttl:    ttl,
	}, nil
}

// Release releases the distributed lock (only if we own it)
func (l *Lock) Release(ctx context.Context) error {
	// Lua script for atomic check-and-delete
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	
	result, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Int()
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	
	if result == 0 {
		return fmt.Errorf("lock not owned by this token (expired or stolen)")
	}
	
	return nil
}

// Extend extends the lock TTL (if we still own it)
func (l *Lock) Extend(ctx context.Context, additionalTTL time.Duration) error {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	
	ttlSeconds := int(additionalTTL.Seconds())
	result, err := l.client.Eval(ctx, script, []string{l.key}, l.token, ttlSeconds).Int()
	if err != nil {
		return fmt.Errorf("extend lock: %w", err)
	}
	
	if result == 0 {
		return fmt.Errorf("lock not owned by this token")
	}
	
	l.ttl = additionalTTL
	return nil
}

// ===============================
// Rate Limiting (Token Bucket)
// ===============================

// RateLimit checks if a request is allowed (token bucket algorithm)
func (c *CacheService) RateLimit(ctx context.Context, userID string, maxRequests int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s", userID)
	
	// Lua script for atomic rate limiting
	script := `
		local key = KEYS[1]
		local max = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		
		local current = redis.call('incr', key)
		if current == 1 then
			redis.call('expire', key, ttl)
		end
		
		if current > max then
			return 0  -- Rate limit exceeded
		else
			return 1  -- Request allowed
		end
	`
	
	ttlSeconds := int(window.Seconds())
	result, err := c.client.Eval(ctx, script, []string{key}, maxRequests, ttlSeconds).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit check: %w", err)
	}
	
	return result == 1, nil
}

// GetRateLimitStatus returns current rate limit status
func (c *CacheService) GetRateLimitStatus(ctx context.Context, userID string) (int, error) {
	key := fmt.Sprintf("ratelimit:%s", userID)
	
	count, err := c.client.Get(ctx, key).Int()
	if err != nil {
		if err == redis.Nil {
			return 0, nil // No requests yet
		}
		return 0, fmt.Errorf("get rate limit: %w", err)
	}
	
	return count, nil
}

// ===============================
// Pub/Sub for Event Broadcasting
// ===============================

// PublishReceiptEvent publishes a receipt event to subscribers
func (c *CacheService) PublishReceiptEvent(ctx context.Context, event string, data map[string]interface{}) error {
	channel := "receipt:events"
	
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	
	return c.client.Publish(ctx, channel, payload).Err()
}

// SubscribeToReceiptEvents subscribes to receipt events
func (c *CacheService) SubscribeToReceiptEvents(ctx context.Context, handler func(event []byte)) error {
	pubsub := c.client.Subscribe(ctx, "receipt:events")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	for msg := range ch {
		handler([]byte(msg.Payload))
	}
	
	return nil
}

// ===============================
// Analytics & Metrics
// ===============================

// IncrementCounter increments a metric counter
func (c *CacheService) IncrementCounter(ctx context.Context, metric string, amount int64) error {
	key := fmt.Sprintf("metric:%s", metric)
	return c.client.IncrBy(ctx, key, amount).Err()
}

// GetCounter retrieves a metric counter value
func (c *CacheService) GetCounter(ctx context.Context, metric string) (int64, error) {
	key := fmt.Sprintf("metric:%s", metric)
	return c.client.Get(ctx, key).Int64()
}

// RecordLatency records a latency measurement
func (c *CacheService) RecordLatency(ctx context.Context, operation string, latencyMs float64) error {
	key := fmt.Sprintf("latency:%s", operation)
	
	// Use sorted set to track latencies (for percentile calculations)
	now := time.Now().Unix()
	member := fmt.Sprintf("%d:%.2f", now, latencyMs)
	
	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: latencyMs, Member: member})
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()))
	pipe.Expire(ctx, key, 24*time.Hour)
	
	_, err := pipe.Exec(ctx)
	return err
}

// GetLatencyPercentile calculates latency percentile
func (c *CacheService) GetLatencyPercentile(ctx context.Context, operation string, percentile float64) (float64, error) {
	key := fmt.Sprintf("latency:%s", operation)
	
	// Get total count
	count, err := c.client.ZCard(ctx, key).Result()
	if err != nil || count == 0 {
		return 0, err
	}
	
	// Calculate index for percentile
	index := int64(float64(count) * percentile / 100.0)
	if index >= count {
		index = count - 1
	}
	
	// Get value at index
	values, err := c.client.ZRange(ctx, key, index, index).Result()
	if err != nil || len(values) == 0 {
		return 0, err
	}
	
	// Parse latency from "timestamp:latency" format
	var latency float64
	fmt.Sscanf(values[0], "%*d:%f", &latency)
	return latency, nil
}

// ===============================
// Cache Statistics
// ===============================

// GetStats returns cache statistics
func (c *CacheService) GetStats() map[string]interface{} {
	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}
	
	return map[string]interface{}{
		"hits":      c.hits,
		"misses":    c.misses,
		"total":     total,
		"hit_rate":  hitRate,
	}
}

// ResetStats resets cache statistics
func (c *CacheService) ResetStats() {
	c.hits = 0
	c.misses = 0
}

// ===============================
// Health Check
// ===============================

// Health checks Redis connectivity and latency
func (c *CacheService) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	start := time.Now()
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	
	latency := time.Since(start)
	if latency > 100*time.Millisecond {
		c.logger.Warn("redis high latency", zap.Duration("latency", latency))
	}
	
	return nil
}

// Close closes the Redis client
func (c *CacheService) Close() error {
	return c.client.Close()
}

// ===============================
// Usage Examples
// ===============================
/*

// Example 1: Basic receipt caching
receipt := &Receipt{Code: "RCP-2025-000000012345", ...}
data, _ := json.Marshal(receipt)
cache.SetReceipt(ctx, receipt.Code, data, time.Hour)

// Example 2: Batch caching (10x faster)
receipts := map[string][]byte{
    "RCP-2025-000000012345": data1,
    "RCP-2025-000000012346": data2,
    // ... 100 more
}
cache.SetReceiptsBatch(ctx, receipts, time.Hour)

// Example 3: Distributed locking
lock, err := cache.AcquireLock(ctx, "process-payment-12345", 10*time.Second)
if err != nil {
    // Lock already held
    return err
}
defer lock.Release(ctx)

// Critical section (guaranteed single execution)
// ... process payment

// Example 4: Rate limiting
allowed, _ := cache.RateLimit(ctx, userID, 100, time.Minute)
if !allowed {
    return errors.New("rate limit exceeded")
}

// Example 5: Event broadcasting
cache.PublishReceiptEvent(ctx, "receipt.created", map[string]interface{}{
    "code": "RCP-2025-000000012345",
    "amount": 10000,
})

*/

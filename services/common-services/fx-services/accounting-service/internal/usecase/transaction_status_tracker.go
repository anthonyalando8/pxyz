package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TransactionStatus represents detailed transaction status
type TransactionStatus struct {
	ReceiptCode  string    `json:"receipt_code"`
	Status       string    `json:"status"` // "processing", "executing", "completed", "failed"
	ErrorMessage string    `json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	StartedAt    time.Time `json:"started_at"`
}

// TransactionStatusTracker tracks transaction status with Redis + in-memory cache
type TransactionStatusTracker struct {
	redisClient   *redis.Client
	localCache    map[string]*TransactionStatus
	mu            sync.RWMutex
	updateChan    chan *TransactionStatus
	stopChan      chan struct{}
	flushInterval time.Duration
}

func NewTransactionStatusTracker(redisClient *redis.Client, flushInterval time.Duration) *TransactionStatusTracker {
	return &TransactionStatusTracker{
		redisClient:   redisClient,
		localCache:    make(map[string]*TransactionStatus),
		updateChan:    make(chan *TransactionStatus, 1000),
		stopChan:      make(chan struct{}),
		flushInterval: flushInterval,
	}
}

func (t *TransactionStatusTracker) Start() {
	go t.worker()
	go t.cleaner()
}

func (t *TransactionStatusTracker) Stop() {
	close(t.stopChan)
	t.flushAll() // Flush remaining updates
}

// Track initializes tracking for a new transaction
func (t *TransactionStatusTracker) Track(receiptCode, status string) {
	t.mu.Lock()
	t.localCache[receiptCode] = &TransactionStatus{
		ReceiptCode: receiptCode,
		Status:      status,
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	t.mu.Unlock()

	// Send to update channel
	select {
	case t.updateChan <- t.localCache[receiptCode]:
	default:
		// Channel full, log warning
		fmt.Printf("[STATUS TRACKER] Update channel full for %s\n", receiptCode)
	}
}

// Update updates transaction status
func (t *TransactionStatusTracker) Update(receiptCode, status, errorMsg string) {
	t.mu.Lock()
	cached, exists := t.localCache[receiptCode]
	if !exists {
		cached = &TransactionStatus{
			ReceiptCode: receiptCode,
			StartedAt:   time.Now(),
		}
		t.localCache[receiptCode] = cached
	}

	cached.Status = status
	cached.ErrorMessage = errorMsg
	cached.UpdatedAt = time.Now()
	t.mu.Unlock()

	// Send to update channel
	select {
	case t.updateChan <- cached:
	default:
		// Channel full, write directly to Redis
		t.writeToRedis(receiptCode, cached)
	}
}

// Get retrieves transaction status (check local cache first)
func (t *TransactionStatusTracker) Get(receiptCode string) string {
	// Check local cache first
	t.mu.RLock()
	if status, exists := t.localCache[receiptCode]; exists {
		t.mu.RUnlock()
		return status.Status
	}
	t.mu.RUnlock()

	// Check Redis
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("transaction:status:%s", receiptCode)
	val, err := t.redisClient.Get(ctx, key).Result()
	if err != nil {
		return ""
	}

	var status TransactionStatus
	if json.Unmarshal([]byte(val), &status) == nil {
		// Cache locally for faster subsequent access
		t.mu.Lock()
		t.localCache[receiptCode] = &status
		t.mu.Unlock()
		
		return status.Status
	}

	return ""
}

// GetFull retrieves full transaction status details
func (t *TransactionStatusTracker) GetFull(receiptCode string) *TransactionStatus {
	// Check local cache
	t.mu.RLock()
	if status, exists := t.localCache[receiptCode]; exists {
		t.mu.RUnlock()
		return status
	}
	t.mu.RUnlock()

	// Check Redis
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("transaction:status:%s", receiptCode)
	val, err := t.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil
	}

	var status TransactionStatus
	if json.Unmarshal([]byte(val), &status) == nil {
		return &status
	}

	return nil
}

// Exists checks if a receipt code is being tracked (in-memory or Redis)
func (t *TransactionStatusTracker) Exists(receiptCode string) bool {
	// Check local cache first (fast path)
	t.mu.RLock()
	_, exists := t.localCache[receiptCode]
	t.mu.RUnlock()
	
	if exists {
		return true
	}

	// Check Redis (slow path)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("transaction:status:%s", receiptCode)
	result, err := t.redisClient.Exists(ctx, key).Result()
	
	return err == nil && result > 0
}

// worker processes status updates in background
func (t *TransactionStatusTracker) worker() {
	ticker := time.NewTicker(t.flushInterval)
	defer ticker.Stop()

	batch := make(map[string]*TransactionStatus)

	for {
		select {
		case status := <-t.updateChan:
			batch[status.ReceiptCode] = status

			// Flush if batch is large enough
			if len(batch) >= 100 {
				t.flushBatch(batch)
				batch = make(map[string]*TransactionStatus)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				t.flushBatch(batch)
				batch = make(map[string]*TransactionStatus)
			}

		case <-t.stopChan:
			// Flush remaining
			if len(batch) > 0 {
				t.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch writes a batch of status updates to Redis
func (t *TransactionStatusTracker) flushBatch(batch map[string]*TransactionStatus) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipe := t.redisClient.Pipeline()

	for receiptCode, status := range batch {
		key := fmt.Sprintf("transaction:status:%s", receiptCode)
		
		data, err := json.Marshal(status)
		if err != nil {
			continue
		}

		// Set with TTL (keep for 24 hours)
		pipe.Set(ctx, key, data, 24*time.Hour)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		fmt.Printf("[STATUS TRACKER] Failed to flush batch: %v\n", err)
	} else {
		fmt.Printf("[STATUS TRACKER] Flushed %d status updates to Redis\n", len(batch))
	}
}

// writeToRedis writes single status update directly to Redis
func (t *TransactionStatusTracker) writeToRedis(receiptCode string, status *TransactionStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("transaction:status:%s", receiptCode)
	
	data, err := json.Marshal(status)
	if err != nil {
		return
	}

	_ = t.redisClient.Set(ctx, key, data, 24*time.Hour).Err()
}

// flushAll writes all pending updates to Redis
func (t *TransactionStatusTracker) flushAll() {
	t.mu.RLock()
	batch := make(map[string]*TransactionStatus, len(t.localCache))
	for k, v := range t.localCache {
		batch[k] = v
	}
	t.mu.RUnlock()

	t.flushBatch(batch)
}

// cleaner removes old entries from local cache
func (t *TransactionStatusTracker) cleaner() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.cleanOldEntries()
		case <-t.stopChan:
			return
		}
	}
}

// cleanOldEntries removes entries older than 10 minutes from local cache
func (t *TransactionStatusTracker) cleanOldEntries() {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for receiptCode, status := range t.localCache {
		if status.UpdatedAt.Before(cutoff) {
			delete(t.localCache, receiptCode)
		}
	}
}
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"receipt-service/internal/domain"
	"receipt-service/internal/repository"
	"receipt-service/pkg/cache"
	"receipt-service/pkg/generator"
	"receipt-service/pkg/utils"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	notificationclient "x/shared/notification"
)

const (
	// CRITICAL: Optimal batch size for 4000+ TPS
	OptimalBatchSize = 100  // receipts per database batch
	MaxBatchSize     = 500  // absolute maximum per request
	MaxConcurrency   = 10   // concurrent batch processing

	// Rate limiting
	MaxReceiptsPerSecondPerUser = 1000
	MaxReceiptsPerMinutePerUser = 50000
)

// Metrics
var (
	usecaseReceiptsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "usecase_receipts_processed_total",
			Help: "Total number of receipts processed in usecase",
		},
		[]string{"operation", "status"},
	)

	usecaseProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "usecase_processing_duration_seconds",
			Help:    "Duration of usecase operations",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2, 5},
		},
		[]string{"operation"},
	)

	kafkaPublishErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_publish_errors_total",
			Help: "Total number of Kafka publish errors",
		},
	)
)

type ReceiptUsecase struct {
	repo               repository.ReceiptRepository
	cache              *cache.CacheService
	genLegacy          *generator.Generator
	genV2              *receiptutil.ReceiptGenerator
	notificationClient *notificationclient.NotificationService
	kafkaWriter        *kafka.Writer
	logger             *zap.Logger
}

func NewReceiptUsecase(
	r repository.ReceiptRepository,
	cache *cache.CacheService,
	genLegacy *generator.Generator,
	genV2 *receiptutil.ReceiptGenerator,
	notificationClient *notificationclient.NotificationService,
	kafkaWriter *kafka.Writer,
	logger *zap.Logger,
) *ReceiptUsecase {
	return &ReceiptUsecase{
		repo:               r,
		cache:              cache,
		genLegacy:          genLegacy,
		genV2:              genV2,
		notificationClient: notificationClient,
		kafkaWriter:        kafkaWriter,
		logger:             logger,
	}
}

// ===============================
// CREATE OPERATIONS (OPTIMIZED)
// ===============================

// CreateReceipts creates multiple receipts with intelligent batch splitting
func (uc *ReceiptUsecase) CreateReceipts(ctx context.Context, recs []*domain.Receipt) ([]*domain.Receipt, error) {
	if len(recs) == 0 {
		return nil, nil
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		usecaseProcessingDuration.WithLabelValues("create").Observe(duration)

		uc.logger.Info("receipts creation completed",
			zap.Int("count", len(recs)),
			zap.Duration("duration", time.Since(start)),
			zap.Float64("rps", float64(len(recs))/duration),
		)
	}()

	// OPTIMIZATION 1: Validate batch size
	if len(recs) > MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(recs), MaxBatchSize)
	}

	// OPTIMIZATION 2: Generate codes for all receipts (fast in-memory operation)
	now := time.Now()
	if err := uc.generateReceiptCodes(recs, now); err != nil {
		usecaseReceiptsProcessed.WithLabelValues("create", "error").Add(float64(len(recs)))
		return nil, err
	}

	// OPTIMIZATION 3: Split into optimal batches for database
	var allCreated []*domain.Receipt
	if len(recs) <= OptimalBatchSize {
		// Small batch - process directly
		if err := uc.repo.CreateBatch(ctx, recs); err != nil {
			usecaseReceiptsProcessed.WithLabelValues("create", "error").Add(float64(len(recs)))
			return nil, fmt.Errorf("failed to create receipts: %w", err)
		}
		allCreated = recs
	} else {
		// Large batch - split and process
		batches := splitIntoBatches(recs, OptimalBatchSize)
		allCreated = make([]*domain.Receipt, 0, len(recs))

		for _, batch := range batches {
			if err := uc.repo.CreateBatch(ctx, batch); err != nil {
				usecaseReceiptsProcessed.WithLabelValues("create", "error").Add(float64(len(batch)))
				return nil, fmt.Errorf("failed to create batch: %w", err)
			}
			allCreated = append(allCreated, batch...)
		}
	}

	usecaseReceiptsProcessed.WithLabelValues("create", "success").Add(float64(len(allCreated)))

	// OPTIMIZATION 4: Async Kafka publishing (non-blocking)
	go uc.publishReceiptCreated(allCreated)

	return allCreated, nil
}

// CreateReceiptsWithIdempotency creates receipts with idempotency support
func (uc *ReceiptUsecase) CreateReceiptsWithIdempotency(
	ctx context.Context,
	idempotencyKey string,
	recs []*domain.Receipt,
) ([]*domain.Receipt, bool, error) {
	if idempotencyKey == "" {
		created, err := uc.CreateReceipts(ctx, recs)
		return created, false, err
	}

	// Check idempotency cache
	if uc.cache != nil {
		cached, err := uc.cache.GetIdempotent(ctx, idempotencyKey)
		if err == nil && cached != nil {
			var result []*domain.Receipt
			if err := json.Unmarshal(cached, &result); err == nil {
				uc.logger.Info("idempotent request - returning cached result",
					zap.String("key", idempotencyKey),
					zap.Int("count", len(result)),
				)
				return result, true, nil
			}
		}
	}

	// Create receipts
	created, err := uc.CreateReceipts(ctx, recs)
	if err != nil {
		return nil, false, err
	}

	// Cache result asynchronously
	if uc.cache != nil {
		go func() {
			data, _ := json.Marshal(created)
			uc.cache.SetIdempotent(context.Background(), idempotencyKey, data)
		}()
	}

	return created, false, nil
}

// generateReceiptCodes generates codes for all receipts (in-memory, fast)
func (uc *ReceiptUsecase) generateReceiptCodes(recs []*domain.Receipt, now time.Time) error {
	for _, rec := range recs {
		// Determine account type for code generation
		accountType := "real"
		if rec.AccountType == receiptpb.AccountType_ACCOUNT_TYPE_DEMO {
			accountType = "demo"
		}

		// PRIMARY: Use enhanced generator (V2)
		code := uc.genV2.GenerateCode(accountType)

		// FALLBACK: If V2 fails (shouldn't happen), use legacy
		if code == "" {
			uc.logger.Warn("V2 generator failed, using legacy fallback",
				zap.String("transaction_type", rec.TransactionType.String()),
				zap.String("account_type", accountType),
			)

			var err error
			code, err = uc.genLegacy.GenerateUnique(nil)
			if err != nil {
				return fmt.Errorf("failed to generate receipt code: %w", err)
			}
		}

		// Set receipt fields
		rec.Code = code
		rec.Status = receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING
		rec.CreditorStatus = receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING
		rec.DebitorStatus = receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING
		rec.CreatedAt = now
		rec.UpdatedAt = &now
	}

	return nil
}

// ===============================
// UPDATE OPERATIONS (OPTIMIZED)
// ===============================

// UpdateReceipts updates multiple receipts
func (uc *ReceiptUsecase) UpdateReceipts(ctx context.Context, updates []*domain.ReceiptUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		usecaseProcessingDuration.WithLabelValues("update").Observe(duration)

		uc.logger.Info("receipts update completed",
			zap.Int("count", len(updates)),
			zap.Duration("duration", time.Since(start)),
		)
	}()

	// Update in database
	updatedReceipts, err := uc.repo.UpdateBatch(ctx, updates)
	if err != nil {
		usecaseReceiptsProcessed.WithLabelValues("update", "error").Add(float64(len(updates)))
		return fmt.Errorf("failed to update receipts: %w", err)
	}

	usecaseReceiptsProcessed.WithLabelValues("update", "success").Add(float64(len(updatedReceipts)))

	// Publish to Kafka (async)
	go uc.publishReceiptUpdated(updates)

	return nil
}

// UpdateReceiptsBatch updates multiple receipts and returns updated receipts
func (uc *ReceiptUsecase) UpdateReceiptsBatch(ctx context.Context, updates []*domain.ReceiptUpdate) ([]*domain.Receipt, error) {
	if len(updates) == 0 {
		return nil, nil
	}

	start := time.Now()
	defer func() {
		usecaseProcessingDuration.WithLabelValues("update_batch").Observe(time.Since(start).Seconds())
	}()

	// Update in database
	updatedReceipts, err := uc.repo.UpdateBatch(ctx, updates)
	if err != nil {
		usecaseReceiptsProcessed.WithLabelValues("update_batch", "error").Add(float64(len(updates)))
		return nil, fmt.Errorf("failed to update receipts: %w", err)
	}

	usecaseReceiptsProcessed.WithLabelValues("update_batch", "success").Add(float64(len(updatedReceipts)))

	// Publish to Kafka (async)
	go uc.publishReceiptUpdated(updates)

	return updatedReceipts, nil
}

// ===============================
// READ OPERATIONS (CACHED)
// ===============================

// GetReceiptByCode retrieves a single receipt by code (cache-aware)
func (uc *ReceiptUsecase) GetReceiptByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	if code == "" {
		return nil, fmt.Errorf("receipt code cannot be empty")
	}

	start := time.Now()
	defer func() {
		usecaseProcessingDuration.WithLabelValues("get_by_code").Observe(time.Since(start).Seconds())
	}()

	rec, err := uc.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get receipt by code: %w", err)
	}

	return rec, nil
}

// GetReceiptsBatch retrieves multiple receipts by codes (cache-aware)
func (uc *ReceiptUsecase) GetReceiptsBatch(ctx context.Context, codes []string) ([]*domain.Receipt, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	start := time.Now()
	defer func() {
		usecaseProcessingDuration.WithLabelValues("get_batch").Observe(time.Since(start).Seconds())
	}()

	receipts, err := uc.repo.GetBatchByCodes(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("get receipts batch: %w", err)
	}

	return receipts, nil
}

// ===============================
// QUERY OPERATIONS
// ===============================

// ListReceipts retrieves receipts with filters and pagination
func (uc *ReceiptUsecase) ListReceipts(ctx context.Context, filters *domain.ReceiptFilters) ([]*domain.Receipt, error) {
	if filters == nil {
		return nil, fmt.Errorf("filters cannot be nil")
	}

	start := time.Now()
	defer func() {
		usecaseProcessingDuration.WithLabelValues("list").Observe(time.Since(start).Seconds())
	}()

	receipts, err := uc.repo.ListByFilters(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("list receipts: %w", err)
	}

	return receipts, nil
}

// CountReceipts counts receipts matching filters
func (uc *ReceiptUsecase) CountReceipts(ctx context.Context, filters *domain.ReceiptFilters) (int64, error) {
	if filters == nil {
		return 0, fmt.Errorf("filters cannot be nil")
	}

	count, err := uc.repo.CountByFilters(ctx, filters)
	if err != nil {
		return 0, fmt.Errorf("count receipts: %w", err)
	}

	return count, nil
}

// ===============================
// RATE LIMITING
// ===============================

// CheckRateLimit checks if user is within rate limits
func (uc *ReceiptUsecase) CheckRateLimit(ctx context.Context, userID string, requestCount int) error {
	if uc.cache == nil {
		return nil // Rate limiting disabled
	}

	// Check per-second limit
	allowed, err := uc.cache.RateLimit(ctx, fmt.Sprintf("%s:sec", userID), MaxReceiptsPerSecondPerUser, time.Second)
	if err != nil {
		uc.logger.Warn("rate limit check failed", zap.Error(err))
		return nil // Fail open
	}
	if !allowed {
		return fmt.Errorf("rate limit exceeded: max %d receipts per second", MaxReceiptsPerSecondPerUser)
	}

	// Check per-minute limit
	allowed, err = uc.cache.RateLimit(ctx, fmt.Sprintf("%s:min", userID), MaxReceiptsPerMinutePerUser, time.Minute)
	if err != nil {
		uc.logger.Warn("rate limit check failed", zap.Error(err))
		return nil // Fail open
	}
	if !allowed {
		return fmt.Errorf("rate limit exceeded: max %d receipts per minute", MaxReceiptsPerMinutePerUser)
	}

	return nil
}

// ===============================
// HEALTH CHECK
// ===============================

// Health checks usecase and dependencies health
func (uc *ReceiptUsecase) Health(ctx context.Context) error {
	// Check repository
	if err := uc.repo.Health(ctx); err != nil {
		return fmt.Errorf("repository unhealthy: %w", err)
	}

	// Check cache
	if uc.cache != nil {
		if err := uc.cache.Health(ctx); err != nil {
			uc.logger.Warn("cache unhealthy", zap.Error(err))
			// Don't fail health check for cache issues
		}
	}

	return nil
}

// ===============================
// KAFKA PUBLISHING (OPTIMIZED)
// ===============================

// publishReceiptCreated publishes receipt creation events to Kafka (async, batched)
func (uc *ReceiptUsecase) publishReceiptCreated(recs []*domain.Receipt) {
	if len(recs) == 0 {
		return
	}

	start := time.Now()
	defer func() {
		uc.logger.Debug("kafka publish completed",
			zap.Int("count", len(recs)),
			zap.Duration("duration", time.Since(start)),
		)
	}()

	// Build Kafka messages
	msgs := make([]kafka.Message, 0, len(recs))
	for _, rec := range recs {
		msg := &receiptpb.ReceiptMessage{
			Type:    receiptpb.ReceiptMessageType_TYPE_CREATED,
			Receipt: rec.ToProto(),
		}
		data, err := proto.Marshal(msg)
		if err != nil {
			uc.logger.Error("failed to marshal create message",
				zap.Error(err),
				zap.String("code", rec.Code),
			)
			kafkaPublishErrors.Inc()
			continue
		}
		msgs = append(msgs, kafka.Message{
			Key:   []byte(rec.Code),
			Value: data,
			Time:  time.Now(),
		})
	}

	if len(msgs) == 0 {
		uc.logger.Warn("no messages to publish after marshaling")
		return
	}

	// Publish with timeout and retry
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := uc.kafkaWriter.WriteMessages(ctx, msgs...); err != nil {
		uc.logger.Error("failed to publish receipts to Kafka",
			zap.Error(err),
			zap.Int("count", len(msgs)),
		)
		kafkaPublishErrors.Add(float64(len(msgs)))

		// TODO: Implement dead letter queue for failed messages
	} else {
		uc.logger.Info("receipts published to Kafka successfully",
			zap.Int("count", len(msgs)),
		)
	}
}

// publishReceiptUpdated publishes receipt update events to Kafka (async, batched)
func (uc *ReceiptUsecase) publishReceiptUpdated(updates []*domain.ReceiptUpdate) {
	if len(updates) == 0 {
		return
	}

	start := time.Now()
	defer func() {
		uc.logger.Debug("kafka publish updates completed",
			zap.Int("count", len(updates)),
			zap.Duration("duration", time.Since(start)),
		)
	}()

	msgs := make([]kafka.Message, 0, len(updates))
	for _, upd := range updates {
		msg := &receiptpb.ReceiptMessage{
			Type:   receiptpb.ReceiptMessageType_TYPE_UPDATED,
			Update: upd.ToProto(),
		}
		data, err := proto.Marshal(msg)
		if err != nil {
			uc.logger.Error("failed to marshal update message",
				zap.Error(err),
				zap.String("code", upd.Code),
			)
			kafkaPublishErrors.Inc()
			continue
		}
		msgs = append(msgs, kafka.Message{
			Key:   []byte(upd.Code),
			Value: data,
			Time:  time.Now(),
		})
	}

	if len(msgs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := uc.kafkaWriter.WriteMessages(ctx, msgs...); err != nil {
		uc.logger.Error("failed to publish updated receipts to Kafka",
			zap.Error(err),
			zap.Int("count", len(msgs)),
		)
		kafkaPublishErrors.Add(float64(len(msgs)))
	}
}

// ===============================
// HELPER FUNCTIONS
// ===============================

// splitIntoBatches splits receipts into optimal-sized batches
func splitIntoBatches(receipts []*domain.Receipt, batchSize int) [][]*domain.Receipt {
	if len(receipts) <= batchSize {
		return [][]*domain.Receipt{receipts}
	}

	numBatches := (len(receipts) + batchSize - 1) / batchSize
	batches := make([][]*domain.Receipt, 0, numBatches)

	for i := 0; i < len(receipts); i += batchSize {
		end := i + batchSize
		if end > len(receipts) {
			end = len(receipts)
		}
		batches = append(batches, receipts[i:end])
	}

	return batches
}

// extractCodes extracts receipt codes from receipts
func extractCodes(receipts []*domain.Receipt) []string {
	codes := make([]string, len(receipts))
	for i, r := range receipts {
		codes[i] = r.Code
	}
	return codes
}

// extractCodesFromUpdates extracts receipt codes from updates
func extractCodesFromUpdates(updates []*domain.ReceiptUpdate) []string {
	codes := make([]string, len(updates))
	for i, u := range updates {
		codes[i] = u.Code
	}
	return codes
}

// ===============================
// CONCURRENT PROCESSING (Advanced)
// ===============================

// CreateReceiptsConcurrent creates receipts with concurrent batch processing
// Use this for very large batches (500+) to maximize throughput
func (uc *ReceiptUsecase) CreateReceiptsConcurrent(ctx context.Context, recs []*domain.Receipt) ([]*domain.Receipt, error) {
	if len(recs) == 0 {
		return nil, nil
	}

	if len(recs) <= OptimalBatchSize {
		// Small batch - use regular flow
		return uc.CreateReceipts(ctx, recs)
	}

	start := time.Now()
	defer func() {
		uc.logger.Info("concurrent receipts creation completed",
			zap.Int("count", len(recs)),
			zap.Duration("duration", time.Since(start)),
		)
	}()

	// Generate codes first (fast in-memory)
	now := time.Now()
	if err := uc.generateReceiptCodes(recs, now); err != nil {
		return nil, err
	}

	// Split into batches
	batches := splitIntoBatches(recs, OptimalBatchSize)

	// Process batches concurrently with semaphore
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, MaxConcurrency)
	errChan := make(chan error, len(batches))
	results := make(chan []*domain.Receipt, len(batches))

	for _, batch := range batches {
		wg.Add(1)
		go func(b []*domain.Receipt) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create batch
			if err := uc.repo.CreateBatch(ctx, b); err != nil {
				errChan <- err
				return
			}
			results <- b
		}(batch)
	}

	// Wait for all goroutines
	wg.Wait()
	close(errChan)
	close(results)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	// Collect results
	allCreated := make([]*domain.Receipt, 0, len(recs))
	for batch := range results {
		allCreated = append(allCreated, batch...)
	}

	// Async Kafka publishing
	go uc.publishReceiptCreated(allCreated)

	return allCreated, nil
}

// ===============================
// METRICS & ANALYTICS
// ===============================

// GetMetrics retrieves service metrics
func (uc *ReceiptUsecase) GetMetrics(ctx context.Context, fromTime, toTime time.Time) (*ReceiptMetrics, error) {
	uc.logger.Debug("getting metrics",
		zap.Time("from", fromTime),
		zap.Time("to", toTime),
	)

	// TODO: Implement actual metrics collection from cache/database
	return &ReceiptMetrics{
		TotalReceipts:     0,
		ReceiptsPerSecond: 0,
		AvgCreationTimeMs: 0,
		P95CreationTimeMs: 0,
		P99CreationTimeMs: 0,
	}, nil
}

// ReceiptMetrics holds receipt service metrics
type ReceiptMetrics struct {
	TotalReceipts     int64
	ReceiptsPerSecond int64
	AvgCreationTimeMs float64
	P95CreationTimeMs float64
	P99CreationTimeMs float64
	ReceiptsByType    map[string]int64
	ReceiptsByStatus  map[string]int64
}
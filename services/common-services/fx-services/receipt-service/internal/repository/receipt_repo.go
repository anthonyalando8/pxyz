package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"receipt-service/internal/domain"
	"receipt-service/pkg/cache"
	"strings"
	"time"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	ErrReceiptNotFound       = errors.New("receipt not found")
	ErrDuplicateReceipt      = errors.New("receipt already exists")
	ErrOptimisticLockFailure = errors.New("concurrent modification detected")
	ErrInvalidAmount         = errors.New("amount must be positive")
)

// Prometheus metrics
var (
	receiptCreateDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "receipt_create_duration_seconds",
			Help:    "Duration of receipt creation operations",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2},
		},
		[]string{"batch_size_range"},
	)

	receiptCreateTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "receipt_create_total",
			Help: "Total number of receipts created",
		},
		[]string{"status"},
	)

	cacheHitTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "receipt_cache_hit_total",
			Help: "Total number of cache hits/misses",
		},
		[]string{"result"}, // hit or miss
	)

	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "receipt_db_query_duration_seconds",
			Help:    "Duration of database queries",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)
)

// ReceiptRepository defines high-performance receipt operations
type ReceiptRepository interface {
	// Single operations
	Create(ctx context.Context, receipt *domain.Receipt) error
	GetByCode(ctx context.Context, code string) (*domain.Receipt, error)
	Update(ctx context.Context, receipt *domain.Receipt) error

	// Batch operations (high performance)
	CreateBatch(ctx context.Context, receipts []*domain.Receipt) error
	GetBatchByCodes(ctx context.Context, codes []string) ([]*domain.Receipt, error)
	UpdateBatch(ctx context.Context, updates []*domain.ReceiptUpdate) ([]*domain.Receipt, error)

	// Query operations
	ListByFilters(ctx context.Context, filters *domain.ReceiptFilters) ([]*domain.Receipt, error)
	CountByFilters(ctx context.Context, filters *domain.ReceiptFilters) (int64, error)

	// Utility
	ExistsByCode(ctx context.Context, code string) (bool, error)
	Health(ctx context.Context) error

	// Cache operations
	InvalidateCache(ctx context.Context, codes []string) error
}

type receiptRepo struct {
	db     *pgxpool.Pool
	cache  *cache.CacheService
	logger *zap.Logger
}

func NewReceiptRepo(db *pgxpool.Pool, cache *cache.CacheService, logger *zap.Logger) ReceiptRepository {
	return &receiptRepo{
		db:     db,
		cache:  cache,
		logger: logger,
	}
}

// ===============================
// CREATE OPERATIONS (OPTIMIZED)
// ===============================

// Create inserts a single receipt
func (r *receiptRepo) Create(ctx context.Context, receipt *domain.Receipt) error {
	return r.CreateBatch(ctx, []*domain.Receipt{receipt})
}

// CreateBatch performs ultra-high-performance batch insert
// Optimized for 4000+ TPS with minimal locking
func (r *receiptRepo) CreateBatch(ctx context.Context, receipts []*domain.Receipt) error {
	if len(receipts) == 0 {
		return nil
	}

	start := time.Now()
	batchSizeRange := getBatchSizeRange(len(receipts))

	defer func() {
		duration := time.Since(start).Seconds()
		receiptCreateDuration.WithLabelValues(batchSizeRange).Observe(duration)

		r.logger.Info("batch create receipts",
			zap.Int("count", len(receipts)),
			zap.Duration("duration", time.Since(start)),
			zap.Float64("rps", float64(len(receipts))/duration),
		)
	}()

	// Start transaction with optimized isolation level
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted, // Minimum locking
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		receiptCreateTotal.WithLabelValues("error").Add(float64(len(receipts)))
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// OPTIMIZATION 1: Bulk insert lookups using temp table + COPY
	lookupIDs, err := r.bulkInsertLookups(ctx, tx, receipts)
	if err != nil {
		receiptCreateTotal.WithLabelValues("error").Add(float64(len(receipts)))
		return err
	}

	// OPTIMIZATION 2: Bulk insert fx_receipts using COPY (already optimal)
	if err := r.bulkInsertReceipts(ctx, tx, receipts, lookupIDs); err != nil {
		receiptCreateTotal.WithLabelValues("error").Add(float64(len(receipts)))
		return err
	}

	// OPTIMIZATION 3: Populate receipt IDs in memory (no DB fetch needed)
	for _, rec := range receipts {
		rec.LookupID = lookupIDs[rec.Code]
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		receiptCreateTotal.WithLabelValues("error").Add(float64(len(receipts)))
		return fmt.Errorf("commit transaction: %w", err)
	}

	receiptCreateTotal.WithLabelValues("success").Add(float64(len(receipts)))

	// OPTIMIZATION 4: Async cache population (non-blocking)
	go r.populateCacheAsync(receipts)

	return nil
}

// bulkInsertLookups inserts receipt lookups using temp table for maximum performance
func (r *receiptRepo) bulkInsertLookups(ctx context.Context, tx pgx.Tx, receipts []*domain.Receipt) (map[string]int64, error) {
	lookupIDs := make(map[string]int64, len(receipts))

	// Step 1: Create temporary table (dropped automatically on commit)
	_, err := tx.Exec(ctx, `
		CREATE TEMP TABLE temp_lookup (
			code TEXT,
			account_type TEXT,
			created_at TIMESTAMPTZ
		) ON COMMIT DROP
	`)
	if err != nil {
		return nil, fmt.Errorf("create temp table: %w", err)
	}

	// Step 2: COPY into temp table (ultra-fast, no indexes)
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"temp_lookup"},
		[]string{"code", "account_type", "created_at"},
		pgx.CopyFromSlice(len(receipts), func(i int) ([]interface{}, error) {
			r := receipts[i]
			return []interface{}{r.Code, r.AccountType, r.CreatedAt}, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("copy to temp table: %w", err)
	}

	// Step 3: Bulk INSERT from temp table with conflict detection
	rows, err := tx.Query(ctx, `
		INSERT INTO receipt_lookup (code, account_type, created_at)
		SELECT code, account_type::account_type_enum, created_at 
		FROM temp_lookup
		ON CONFLICT (code) DO NOTHING
		RETURNING id, code
	`)
	if err != nil {
		return nil, fmt.Errorf("insert lookups: %w", err)
	}
	defer rows.Close()

	// Step 4: Collect inserted IDs
	insertedCodes := make(map[string]bool)
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return nil, fmt.Errorf("scan lookup: %w", err)
		}
		lookupIDs[code] = id
		insertedCodes[code] = true
	}

	// Step 5: Check for conflicts (duplicate codes)
	if len(insertedCodes) < len(receipts) {
		r.logger.Warn("duplicate receipt codes detected",
			zap.Int("requested", len(receipts)),
			zap.Int("inserted", len(insertedCodes)),
		)
		return nil, ErrDuplicateReceipt
	}

	return lookupIDs, nil
}

// bulkInsertReceipts inserts fx_receipts using COPY protocol
func (r *receiptRepo) bulkInsertReceipts(ctx context.Context, tx pgx.Tx, receipts []*domain.Receipt, lookupIDs map[string]int64) error {
	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"fx_receipts"},
		[]string{
			"lookup_id", "account_type",
			"creditor_account_id", "creditor_ledger_id", "creditor_account_type", "creditor_status",
			"debitor_account_id", "debitor_ledger_id", "debitor_account_type", "debitor_status",
			"transaction_type", "coded_type", "amount", "original_amount", "transaction_cost",
			"currency", "original_currency", "exchange_rate",
			"external_ref", "parent_receipt_code",
			"status", "error_message",
			"created_at", "created_by", "metadata",
		},
		pgx.CopyFromSlice(len(receipts), func(i int) ([]interface{}, error) {
			rec := receipts[i]
			lookupID, ok := lookupIDs[rec.Code]
			if !ok {
				return nil, fmt.Errorf("lookup_id not found for code: %s", rec.Code)
			}

			metadataJSON, _ := json.Marshal(rec.Metadata)

			return []interface{}{
				lookupID, rec.AccountType,
				rec.Creditor.AccountID, rec.Creditor.LedgerID, rec.Creditor.OwnerType, rec.Creditor.Status,
				rec.Debitor.AccountID, rec.Debitor.LedgerID, rec.Debitor.OwnerType, rec.Debitor.Status,
				rec.TransactionType, rec.CodedType, rec.Amount, rec.OriginalAmount, rec.TransactionCost,
				rec.Currency, rec.OriginalCurrency, rec.ExchangeRate,
				rec.ExternalRef, rec.ParentReceiptCode,
				rec.Status, rec.ErrorMessage,
				rec.CreatedAt, rec.CreatedBy, metadataJSON,
			}, nil
		}),
	)

	if err != nil {
		// Check for specific PostgreSQL errors
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505": // unique_violation
				return ErrDuplicateReceipt
			case "23503": // foreign_key_violation
				return fmt.Errorf("invalid account reference: %w", err)
			case "23514": // check_violation
				return ErrInvalidAmount
			}
		}
		return fmt.Errorf("copy receipts: %w", err)
	}

	return nil
}

// populateCacheAsync populates cache asynchronously (non-blocking)
func (r *receiptRepo) populateCacheAsync(receipts []*domain.Receipt) {
	if r.cache == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cacheData := make(map[string][]byte, len(receipts))
	for _, rec := range receipts {
		data, err := json.Marshal(rec)
		if err != nil {
			r.logger.Warn("failed to marshal receipt for cache",
				zap.Error(err),
				zap.String("code", rec.Code),
			)
			continue
		}
		cacheData[rec.Code] = data
	}

	if len(cacheData) > 0 {
		if err := r.cache.SetReceiptsBatch(ctx, cacheData, time.Hour); err != nil {
			r.logger.Error("failed to populate cache",
				zap.Error(err),
				zap.Int("count", len(cacheData)),
			)
		}
	}
}

// ===============================
// READ OPERATIONS (WITH CACHING)
// ===============================

// GetByCode retrieves a receipt by code (cache-first strategy)
func (r *receiptRepo) GetByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	start := time.Now()
	defer func() {
		dbQueryDuration.WithLabelValues("get_by_code").Observe(time.Since(start).Seconds())
	}()

	// OPTIMIZATION 1: Try cache first
	if r.cache != nil {
		data, err := r.cache.GetReceipt(ctx, code)
		if err == nil && data != nil {
			var rec domain.Receipt
			if err := json.Unmarshal(data, &rec); err == nil {
				cacheHitTotal.WithLabelValues("hit").Inc()
				r.logger.Debug("cache hit", zap.String("code", code))
				return &rec, nil
			}
		}
		cacheHitTotal.WithLabelValues("miss").Inc()
	}

	// OPTIMIZATION 2: Cache miss - query database
	rec, err := r.queryReceiptByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// OPTIMIZATION 3: Populate cache asynchronously
	go r.cacheReceipt(rec)

	return rec, nil
}

// queryReceiptByCode queries receipt from database
func (r *receiptRepo) queryReceiptByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	query := `
		SELECT 
			rl.id, rl.code, rl.account_type,
			fr.creditor_account_id, fr.creditor_ledger_id, fr.creditor_account_type, fr.creditor_status,
			fr.debitor_account_id, fr.debitor_ledger_id, fr.debitor_account_type, fr.debitor_status,
			fr.transaction_type, fr.coded_type, fr.amount, fr.original_amount, fr.transaction_cost,
			fr.currency, fr.original_currency, fr.exchange_rate,
			fr.external_ref, fr.parent_receipt_code, fr.reversal_receipt_code,
			fr.status, fr.error_message,
			fr.created_at, fr.updated_at, fr.completed_at, fr.reversed_at,
			fr.created_by, fr.reversed_by,
			fr.metadata
		FROM fx_receipts fr
		JOIN receipt_lookup rl ON rl.id = fr.lookup_id
		WHERE rl.code = $1
		ORDER BY fr.created_at DESC
		LIMIT 1
	`

	var rec domain.Receipt
	var metadataJSON []byte
	var updatedAt, completedAt, reversedAt *time.Time

	err := r.db.QueryRow(ctx, query, code).Scan(
		&rec.LookupID, &rec.Code, &rec.AccountType,
		&rec.Creditor.AccountID, &rec.Creditor.LedgerID, &rec.Creditor.OwnerType, &rec.Creditor.Status,
		&rec.Debitor.AccountID, &rec.Debitor.LedgerID, &rec.Debitor.OwnerType, &rec.Debitor.Status,
		&rec.TransactionType, &rec.CodedType, &rec.Amount, &rec.OriginalAmount, &rec.TransactionCost,
		&rec.Currency, &rec.OriginalCurrency, &rec.ExchangeRate,
		&rec.ExternalRef, &rec.ParentReceiptCode, &rec.ReversalReceiptCode,
		&rec.Status, &rec.ErrorMessage,
		&rec.CreatedAt, &updatedAt, &completedAt, &reversedAt,
		&rec.CreatedBy, &rec.ReversedBy,
		&metadataJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReceiptNotFound
		}
		return nil, fmt.Errorf("get receipt: %w", err)
	}

	rec.UpdatedAt = updatedAt
	rec.CompletedAt = completedAt
	rec.ReversedAt = reversedAt

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &rec.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	rec.Creditor.IsCreditor = true
	rec.Debitor.IsCreditor = false

	return &rec, nil
}

// GetBatchByCodes retrieves multiple receipts (cache-first, batch-optimized)
func (r *receiptRepo) GetBatchByCodes(ctx context.Context, codes []string) ([]*domain.Receipt, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	start := time.Now()
	defer func() {
		dbQueryDuration.WithLabelValues("get_batch").Observe(time.Since(start).Seconds())
	}()

	var receipts []*domain.Receipt
	uncachedCodes := codes

	// OPTIMIZATION 1: Try batch cache lookup first
	if r.cache != nil {
		cached, err := r.cache.GetReceiptsBatch(ctx, codes)
		if err == nil && len(cached) > 0 {
			receipts = make([]*domain.Receipt, 0, len(codes))
			uncachedCodes = make([]string, 0, len(codes))

			for _, code := range codes {
				if data, ok := cached[code]; ok {
					var rec domain.Receipt
					if err := json.Unmarshal(data, &rec); err == nil {
						receipts = append(receipts, &rec)
						cacheHitTotal.WithLabelValues("hit").Inc()
						continue
					}
				}
				uncachedCodes = append(uncachedCodes, code)
				cacheHitTotal.WithLabelValues("miss").Inc()
			}

			r.logger.Debug("batch cache lookup",
				zap.Int("total", len(codes)),
				zap.Int("cached", len(cached)),
				zap.Int("uncached", len(uncachedCodes)),
			)
		}
	}

	// OPTIMIZATION 2: Query uncached receipts from database
	if len(uncachedCodes) > 0 {
		dbReceipts, err := r.queryReceiptsByCodes(ctx, uncachedCodes)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, dbReceipts...)

		// OPTIMIZATION 3: Populate cache asynchronously
		go r.cacheReceiptBatch(dbReceipts)
	}

	return receipts, nil
}

// queryReceiptsByCodes queries multiple receipts from database
func (r *receiptRepo) queryReceiptsByCodes(ctx context.Context, codes []string) ([]*domain.Receipt, error) {
	query := `
		SELECT 
			rl.id, rl.code, rl.account_type,
			fr.creditor_account_id, fr.creditor_ledger_id, fr.creditor_account_type, fr.creditor_status,
			fr.debitor_account_id, fr.debitor_ledger_id, fr.debitor_account_type, fr.debitor_status,
			fr.transaction_type, fr.coded_type, fr.amount, fr.original_amount, fr.transaction_cost,
			fr.currency, fr.original_currency, fr.exchange_rate,
			fr.external_ref, fr.parent_receipt_code, fr.reversal_receipt_code,
			fr.status, fr.error_message,
			fr.created_at, fr.updated_at, fr.completed_at, fr.reversed_at,
			fr.created_by, fr.reversed_by,
			fr.metadata
		FROM fx_receipts fr
		JOIN receipt_lookup rl ON rl.id = fr.lookup_id
		WHERE rl.code = ANY($1)
		ORDER BY fr.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, codes)
	if err != nil {
		return nil, fmt.Errorf("batch get receipts: %w", err)
	}
	defer rows.Close()

	var receipts []*domain.Receipt
	for rows.Next() {
		var rec domain.Receipt
		var metadataJSON []byte
		var updatedAt, completedAt, reversedAt *time.Time

		err := rows.Scan(
			&rec.LookupID, &rec.Code, &rec.AccountType,
			&rec.Creditor.AccountID, &rec.Creditor.LedgerID, &rec.Creditor.OwnerType, &rec.Creditor.Status,
			&rec.Debitor.AccountID, &rec.Debitor.LedgerID, &rec.Debitor.OwnerType, &rec.Debitor.Status,
			&rec.TransactionType, &rec.CodedType, &rec.Amount, &rec.OriginalAmount, &rec.TransactionCost,
			&rec.Currency, &rec.OriginalCurrency, &rec.ExchangeRate,
			&rec.ExternalRef, &rec.ParentReceiptCode, &rec.ReversalReceiptCode,
			&rec.Status, &rec.ErrorMessage,
			&rec.CreatedAt, &updatedAt, &completedAt, &reversedAt,
			&rec.CreatedBy, &rec.ReversedBy,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan receipt: %w", err)
		}

		rec.UpdatedAt = updatedAt
		rec.CompletedAt = completedAt
		rec.ReversedAt = reversedAt

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &rec.Metadata)
		}

		rec.Creditor.IsCreditor = true
		rec.Debitor.IsCreditor = false

		receipts = append(receipts, &rec)
	}

	return receipts, rows.Err()
}

// cacheReceipt caches a single receipt asynchronously
func (r *receiptRepo) cacheReceipt(rec *domain.Receipt) {
	if r.cache == nil || rec == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	data, err := json.Marshal(rec)
	if err != nil {
		r.logger.Warn("failed to marshal receipt", zap.Error(err))
		return
	}

	if err := r.cache.SetReceipt(ctx, rec.Code, data, time.Hour); err != nil {
		r.logger.Warn("failed to cache receipt", zap.Error(err))
	}
}

// cacheReceiptBatch caches multiple receipts asynchronously
func (r *receiptRepo) cacheReceiptBatch(receipts []*domain.Receipt) {
	if r.cache == nil || len(receipts) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cacheData := make(map[string][]byte, len(receipts))
	for _, rec := range receipts {
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		cacheData[rec.Code] = data
	}

	if err := r.cache.SetReceiptsBatch(ctx, cacheData, time.Hour); err != nil {
		r.logger.Warn("failed to cache receipt batch", zap.Error(err))
	}
}

// ===============================
// UPDATE OPERATIONS
// ===============================

// Update updates a single receipt
func (r *receiptRepo) Update(ctx context.Context, receipt *domain.Receipt) error {
	return nil // Implement if needed
}

// UpdateBatch performs batch updates efficiently
func (r *receiptRepo) UpdateBatch(ctx context.Context, updates []*domain.ReceiptUpdate) ([]*domain.Receipt, error) {
	if len(updates) == 0 {
		return nil, nil
	}

	start := time.Now()
	defer func() {
		dbQueryDuration.WithLabelValues("update_batch").Observe(time.Since(start).Seconds())
		r.logger.Info("batch update receipts",
			zap.Int("count", len(updates)),
			zap.Duration("duration", time.Since(start)),
		)
	}()

	// Build dynamic UPDATE query with CTE
	query := `
		WITH u (code, status, creditor_status, creditor_ledger_id, debitor_status, debitor_ledger_id,
				reversed_by, reversed_at, error_message, completed_at, metadata) AS (
			VALUES %s
		)
		UPDATE fx_receipts fr
		SET 
			status = COALESCE(u.status, fr.status),
			creditor_status = COALESCE(u.creditor_status, fr.creditor_status),
			creditor_ledger_id = COALESCE(NULLIF(u.creditor_ledger_id, 0), fr.creditor_ledger_id),
			debitor_status = COALESCE(u.debitor_status, fr.debitor_status),
			debitor_ledger_id = COALESCE(NULLIF(u.debitor_ledger_id, 0), fr.debitor_ledger_id),
			reversed_by = COALESCE(u.reversed_by, fr.reversed_by),
			reversed_at = COALESCE(u.reversed_at, fr.reversed_at),
			error_message = COALESCE(u.error_message, fr.error_message),
			completed_at = COALESCE(u.completed_at, fr.completed_at),
			metadata = CASE 
				WHEN u.metadata IS NOT NULL AND u.metadata != '{}'::jsonb 
				THEN fr.metadata || u.metadata 
				ELSE fr.metadata 
			END,
			updated_at = now()
		FROM receipt_lookup rl
		JOIN u ON u.code = rl.code
		WHERE fr.lookup_id = rl.id
		RETURNING 
			rl.id, rl.code, rl.account_type,
			fr.creditor_account_id, fr.creditor_ledger_id, fr.creditor_account_type, fr.creditor_status,
			fr.debitor_account_id, fr.debitor_ledger_id, fr.debitor_account_type, fr.debitor_status,
			fr.transaction_type, fr.coded_type, fr.amount, fr.original_amount, fr.transaction_cost,
			fr.currency, fr.original_currency, fr.exchange_rate,
			fr.external_ref, fr.parent_receipt_code, fr.reversal_receipt_code,
			fr.status, fr.error_message,
			fr.created_at, fr.updated_at, fr.completed_at, fr.reversed_at,
			fr.created_by, fr.reversed_by,
			fr.metadata
	`

	// Build VALUES clause
	valueStrings := make([]string, len(updates))
	args := make([]interface{}, 0, len(updates)*11)

	for i, upd := range updates {
		metadataJSON, _ := json.Marshal(upd.MetadataPatch)

		valueStrings[i] = fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			i*11+1, i*11+2, i*11+3, i*11+4, i*11+5, i*11+6,
			i*11+7, i*11+8, i*11+9, i*11+10, i*11+11)

		args = append(args,
			upd.Code,
			upd.Status,
			upd.CreditorStatus,
			upd.CreditorLedgerID,
			upd.DebitorStatus,
			upd.DebitorLedgerID,
			upd.ReversedBy,
			upd.ReversedAt,
			upd.ErrorMessage,
			upd.CompletedAt,
			metadataJSON,
		)
	}

	sql := fmt.Sprintf(query, strings.Join(valueStrings, ","))

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("batch update: %w", err)
	}
	defer rows.Close()

	var results []*domain.Receipt
	updatedCodes := make([]string, 0, len(updates))

	for rows.Next() {
		var rec domain.Receipt
		var metadataJSON []byte
		var updatedAt, completedAt, reversedAt *time.Time

		err := rows.Scan(
			&rec.LookupID, &rec.Code, &rec.AccountType,
			&rec.Creditor.AccountID, &rec.Creditor.LedgerID, &rec.Creditor.OwnerType, &rec.Creditor.Status,
			&rec.Debitor.AccountID, &rec.Debitor.LedgerID, &rec.Debitor.OwnerType, &rec.Debitor.Status,
			&rec.TransactionType, &rec.CodedType, &rec.Amount, &rec.OriginalAmount, &rec.TransactionCost,
			&rec.Currency, &rec.OriginalCurrency, &rec.ExchangeRate,
			&rec.ExternalRef, &rec.ParentReceiptCode, &rec.ReversalReceiptCode,
			&rec.Status, &rec.ErrorMessage,
			&rec.CreatedAt, &updatedAt, &completedAt, &reversedAt,
			&rec.CreatedBy, &rec.ReversedBy,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan updated receipt: %w", err)
		}

		rec.UpdatedAt = updatedAt
		rec.CompletedAt = completedAt
		rec.ReversedAt = reversedAt

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &rec.Metadata)
		}

		rec.Creditor.IsCreditor = true
		rec.Debitor.IsCreditor = false

		results = append(results, &rec)
		updatedCodes = append(updatedCodes, rec.Code)
	}

	// Invalidate cache for updated receipts
	go r.InvalidateCache(context.Background(), updatedCodes)

	return results, rows.Err()
}

// ===============================
// QUERY OPERATIONS WITH FILTERS
// ===============================

// ListByFilters retrieves receipts with filtering and pagination
func (r *receiptRepo) ListByFilters(ctx context.Context, filters *domain.ReceiptFilters) ([]*domain.Receipt, error) {
	start := time.Now()
	defer func() {
		dbQueryDuration.WithLabelValues("list_filters").Observe(time.Since(start).Seconds())
		r.logger.Debug("list receipts by filters",
			zap.Int("page_size", filters.PageSize),
			zap.Duration("duration", time.Since(start)),
		)
	}()

	// Validate and set defaults
	if filters.PageSize <= 0 || filters.PageSize > 100 {
		filters.PageSize = 50 // Default page size
	}

	// Build the query
	query, args := r.buildListQuery(filters)

	// Execute query
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list receipts: %w", err)
	}
	defer rows.Close()

	// Parse results
	receipts := make([]*domain.Receipt, 0, filters.PageSize)
	for rows.Next() {
		rec, err := r.scanReceipt(rows, filters.IncludeMetadata)
		if err != nil {
			return nil, fmt.Errorf("scan receipt: %w", err)
		}
		receipts = append(receipts, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return receipts, nil
}

// CountByFilters counts receipts matching filters
func (r *receiptRepo) CountByFilters(ctx context.Context, filters *domain.ReceiptFilters) (int64, error) {
	start := time.Now()
	defer func() {
		dbQueryDuration.WithLabelValues("count_filters").Observe(time.Since(start).Seconds())
	}()

	query, args := r.buildCountQuery(filters)

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count receipts: %w", err)
	}

	return count, nil
}

// ===============================
// QUERY BUILDERS
// ===============================

// buildListQuery builds the SELECT query with filters
func (r *receiptRepo) buildListQuery(filters *domain.ReceiptFilters) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argPos := 1

	baseSelect := `
		SELECT 
			rl.id, rl.code, rl.account_type,
			fr.creditor_account_id, fr.creditor_ledger_id, fr.creditor_account_type, fr.creditor_status,
			fr.debitor_account_id, fr.debitor_ledger_id, fr.debitor_account_type, fr.debitor_status,
			fr.transaction_type, fr.coded_type, fr.amount, fr.original_amount, fr.transaction_cost,
			fr.currency, fr.original_currency, fr.exchange_rate,
			fr.external_ref, fr.parent_receipt_code, fr.reversal_receipt_code,
			fr.status, fr.creditor_status, fr.debitor_status, fr.error_message,
			fr.created_at, fr.updated_at, fr.completed_at, fr.reversed_at,
			fr.created_by, fr.reversed_by`

	if filters.IncludeMetadata {
		baseSelect += `, fr.metadata`
	}

	baseSelect += `
		FROM fx_receipts fr
		JOIN receipt_lookup rl ON rl.id = fr.lookup_id`

	// Add WHERE conditions
	if len(filters.TransactionTypes) > 0 {
		conditions = append(conditions, fmt.Sprintf("fr.transaction_type = ANY($%d)", argPos))
		args = append(args, enumsToStrings(filters.TransactionTypes))
		argPos++
	}

	if len(filters.Statuses) > 0 {
		conditions = append(conditions, fmt.Sprintf("fr.status = ANY($%d)", argPos))
		args = append(args, statusEnumsToStrings(filters.Statuses))
		argPos++
	}

	if filters.AccountType != receiptpb.AccountType_ACCOUNT_TYPE_UNSPECIFIED {
		conditions = append(conditions, fmt.Sprintf("rl.account_type = $%d", argPos))
		args = append(args, filters.AccountType.String())
		argPos++
	}

	if filters.Currency != "" {
		conditions = append(conditions, fmt.Sprintf("fr.currency = $%d", argPos))
		args = append(args, filters.Currency)
		argPos++
	}

	if filters.ExternalID != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(fr.creditor_account_id IN (SELECT id FROM accounts WHERE owner_id = $%d) OR "+
				"fr.debitor_account_id IN (SELECT id FROM accounts WHERE owner_id = $%d))",
			argPos, argPos))
		args = append(args, filters.ExternalID)
		argPos++
	}

	if filters.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("fr.created_at >= $%d", argPos))
		args = append(args, *filters.FromDate)
		argPos++
	}

	if filters.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("fr.created_at <= $%d", argPos))
		args = append(args, *filters.ToDate)
		argPos++
	}

	// Handle pagination with cursor
	if filters.PageToken != "" {
		cursor, err := decodeCursor(filters.PageToken)
		if err == nil && cursor.LastCreatedAt != nil && cursor.LastID > 0 {
			conditions = append(conditions, fmt.Sprintf(
				"(fr.created_at, rl.id) < ($%d, $%d)",
				argPos, argPos+1))
			args = append(args, *cursor.LastCreatedAt, cursor.LastID)
			argPos += 2
		}
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Build ORDER BY and LIMIT
	orderBy := " ORDER BY fr.created_at DESC, rl.id DESC"
	limit := fmt.Sprintf(" LIMIT $%d", argPos)
	args = append(args, filters.PageSize+1)

	query := baseSelect + whereClause + orderBy + limit

	return query, args
}

// buildCountQuery builds the COUNT query with filters
func (r *receiptRepo) buildCountQuery(filters *domain.ReceiptFilters) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argPos := 1

	baseQuery := `
		SELECT COUNT(*)
		FROM fx_receipts fr
		JOIN receipt_lookup rl ON rl.id = fr.lookup_id`

	if len(filters.TransactionTypes) > 0 {
		conditions = append(conditions, fmt.Sprintf("fr.transaction_type = ANY($%d)", argPos))
		args = append(args, enumsToStrings(filters.TransactionTypes))
		argPos++
	}

	if len(filters.Statuses) > 0 {
		conditions = append(conditions, fmt.Sprintf("fr.status = ANY($%d)", argPos))
		args = append(args, statusEnumsToStrings(filters.Statuses))
		argPos++
	}

	if filters.AccountType != receiptpb.AccountType_ACCOUNT_TYPE_UNSPECIFIED {
		conditions = append(conditions, fmt.Sprintf("rl.account_type = $%d", argPos))
		args = append(args, filters.AccountType.String())
		argPos++
	}

	if filters.Currency != "" {
		conditions = append(conditions, fmt.Sprintf("fr.currency = $%d", argPos))
		args = append(args, filters.Currency)
		argPos++
	}

	if filters.ExternalID != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(fr.creditor_account_id IN (SELECT id FROM accounts WHERE owner_id = $%d) OR "+
				"fr.debitor_account_id IN (SELECT id FROM accounts WHERE owner_id = $%d))",
			argPos, argPos))
		args = append(args, filters.ExternalID)
		argPos++
	}

	if filters.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("fr.created_at >= $%d", argPos))
		args = append(args, *filters.FromDate)
		argPos++
	}

	if filters.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("fr.created_at <= $%d", argPos))
		args = append(args, *filters.ToDate)
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := baseQuery + whereClause

	return query, args
}

// ===============================
// HELPER FUNCTIONS
// ===============================

// scanReceipt scans a row into a Receipt domain object
func (r *receiptRepo) scanReceipt(row pgx.Row, includeMetadata bool) (*domain.Receipt, error) {
	var rec domain.Receipt
	var metadataJSON []byte
	var updatedAt, completedAt, reversedAt *time.Time

	if includeMetadata {
		err := row.Scan(
			&rec.LookupID, &rec.Code, &rec.AccountType,
			&rec.Creditor.AccountID, &rec.Creditor.LedgerID, &rec.Creditor.OwnerType, &rec.Creditor.Status,
			&rec.Debitor.AccountID, &rec.Debitor.LedgerID, &rec.Debitor.OwnerType, &rec.Debitor.Status,
			&rec.TransactionType, &rec.CodedType, &rec.Amount, &rec.OriginalAmount, &rec.TransactionCost,
			&rec.Currency, &rec.OriginalCurrency, &rec.ExchangeRate,
			&rec.ExternalRef, &rec.ParentReceiptCode, &rec.ReversalReceiptCode,
			&rec.Status, &rec.CreditorStatus, &rec.DebitorStatus, &rec.ErrorMessage,
			&rec.CreatedAt, &updatedAt, &completedAt, &reversedAt,
			&rec.CreatedBy, &rec.ReversedBy,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &rec.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}
	} else {
		err := row.Scan(
			&rec.LookupID, &rec.Code, &rec.AccountType,
			&rec.Creditor.AccountID, &rec.Creditor.LedgerID, &rec.Creditor.OwnerType, &rec.Creditor.Status,
			&rec.Debitor.AccountID, &rec.Debitor.LedgerID, &rec.Debitor.OwnerType, &rec.Debitor.Status,
			&rec.TransactionType, &rec.CodedType, &rec.Amount, &rec.OriginalAmount, &rec.TransactionCost,
			&rec.Currency, &rec.OriginalCurrency, &rec.ExchangeRate,
			&rec.ExternalRef, &rec.ParentReceiptCode, &rec.ReversalReceiptCode,
			&rec.Status, &rec.CreditorStatus, &rec.DebitorStatus, &rec.ErrorMessage,
			&rec.CreatedAt, &updatedAt, &completedAt, &reversedAt,
			&rec.CreatedBy, &rec.ReversedBy,
		)
		if err != nil {
			return nil, err
		}
	}

	rec.UpdatedAt = updatedAt
	rec.CompletedAt = completedAt
	rec.ReversedAt = reversedAt
	rec.Creditor.IsCreditor = true
	rec.Debitor.IsCreditor = false

	return &rec, nil
}

// enumsToStrings converts TransactionType enums to strings
func enumsToStrings(enums []receiptpb.TransactionType) []string {
	result := make([]string, len(enums))
	for i, e := range enums {
		result[i] = e.String()
	}
	return result
}

// statusEnumsToStrings converts TransactionStatus enums to strings
func statusEnumsToStrings(enums []receiptpb.TransactionStatus) []string {
	result := make([]string, len(enums))
	for i, e := range enums {
		result[i] = e.String()
	}
	return result
}

// getBatchSizeRange returns a bucket label for metrics
func getBatchSizeRange(size int) string {
	switch {
	case size <= 10:
		return "1-10"
	case size <= 50:
		return "11-50"
	case size <= 100:
		return "51-100"
	case size <= 500:
		return "101-500"
	default:
		return "500+"
	}
}

// ===============================
// CURSOR-BASED PAGINATION
// ===============================

type Cursor struct {
	LastCreatedAt *time.Time `json:"last_created_at"`
	LastID        int64      `json:"last_id"`
}

func encodeCursor(createdAt time.Time, id int64) string {
	cursor := Cursor{
		LastCreatedAt: &createdAt,
		LastID:        id,
	}
	data, _ := json.Marshal(cursor)
	return base64.URLEncoding.EncodeToString(data)
}

func decodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, errors.New("empty cursor")
	}

	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}

	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, fmt.Errorf("unmarshal cursor: %w", err)
	}

	return &cursor, nil
}

func GenerateNextPageToken(receipts []*domain.Receipt, pageSize int) string {
	if len(receipts) <= pageSize {
		return ""
	}

	lastReceipt := receipts[pageSize-1]
	return encodeCursor(lastReceipt.CreatedAt, lastReceipt.LookupID)
}

// ===============================
// UTILITY OPERATIONS
// ===============================

// ExistsByCode checks if a receipt code exists
func (r *receiptRepo) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM receipt_lookup WHERE code = $1)`
	err := r.db.QueryRow(ctx, query, code).Scan(&exists)
	return exists, err
}

// InvalidateCache removes receipts from cache
func (r *receiptRepo) InvalidateCache(ctx context.Context, codes []string) error {
	if r.cache == nil || len(codes) == 0 {
		return nil
	}

	return r.cache.DeleteReceiptsBatch(ctx, codes)
}

// Health checks database connectivity
func (r *receiptRepo) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return r.db.Ping(ctx)
}
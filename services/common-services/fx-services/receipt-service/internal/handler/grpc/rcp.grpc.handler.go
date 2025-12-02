package hgrpc

import (
	"context"
	"fmt"
	"io"
	"strconv"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"

	"receipt-service/internal/domain"
	"receipt-service/internal/repository"
	"receipt-service/internal/usecase"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	MaxBatchSizePerRequest = 500
)

// Metrics
var (
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Duration of gRPC requests",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2, 5},
		},
		[]string{"method"},
	)

	rateLimitExceeded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_exceeded_total",
			Help: "Total number of rate limit exceeded errors",
		},
		[]string{"user_id"},
	)
)

// ReceiptGRPCHandler implements the ReceiptServiceServer gRPC interface (v3)
type ReceiptGRPCHandler struct {
	receiptpb.UnimplementedReceiptServiceServer
	receiptUC *usecase.ReceiptUsecase
	logger    *zap.Logger
}

func NewReceiptGRPCHandler(receiptUC *usecase.ReceiptUsecase, logger *zap.Logger) *ReceiptGRPCHandler {
	return &ReceiptGRPCHandler{
		receiptUC: receiptUC,
		logger:    logger,
	}
}

// ===============================
// SINGLE OPERATIONS
// ===============================

// CreateReceipt - single receipt creation
func (h *ReceiptGRPCHandler) CreateReceipt(
	ctx context.Context,
	req *receiptpb.CreateReceiptRequest,
) (*receiptpb.CreateReceiptResponse, error) {
	timer := prometheus.NewTimer(grpcRequestDuration.WithLabelValues("CreateReceipt"))
	defer timer.ObserveDuration()

	h.logger.Debug("CreateReceipt called",
		zap.String("transaction_type", req.TransactionType.String()),
		zap.String("currency", req.Currency),
	)

	// Extract user ID for rate limiting
	userID := extractUserID(ctx)

	// Check rate limit
	if err := h.receiptUC.CheckRateLimit(ctx, userID, 1); err != nil {
		rateLimitExceeded.WithLabelValues(userID).Inc()
		grpcRequestsTotal.WithLabelValues("CreateReceipt", "rate_limited").Inc()
		return nil, status.Error(codes.ResourceExhausted, err.Error())
	}

	// Validate request
	if err := validateCreateReceiptRequest(req); err != nil {
		grpcRequestsTotal.WithLabelValues("CreateReceipt", "invalid").Inc()
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Reuse batch endpoint with single item
	batchReq := &receiptpb.CreateReceiptsBatchRequest{
		Receipts:         []*receiptpb.CreateReceiptRequest{req},
		FailOnFirstError: true,
	}

	batchResp, err := h.CreateReceiptsBatch(ctx, batchReq)
	if err != nil {
		grpcRequestsTotal.WithLabelValues("CreateReceipt", "error").Inc()
		return nil, err
	}

	if len(batchResp.Receipts) == 0 {
		grpcRequestsTotal.WithLabelValues("CreateReceipt", "error").Inc()
		return nil, status.Error(codes.Internal, "no receipt created")
	}

	grpcRequestsTotal.WithLabelValues("CreateReceipt", "success").Inc()

	return &receiptpb.CreateReceiptResponse{
		Receipt:   batchResp.Receipts[0],
		FromCache: false,
	}, nil
}

// GetReceipt - get single receipt by code
func (h *ReceiptGRPCHandler) GetReceipt(
	ctx context.Context,
	req *receiptpb.GetReceiptRequest,
) (*receiptpb.Receipt, error) {
	timer := prometheus.NewTimer(grpcRequestDuration.WithLabelValues("GetReceipt"))
	defer timer.ObserveDuration()

	if req.Code == "" {
		grpcRequestsTotal.WithLabelValues("GetReceipt", "invalid").Inc()
		return nil, status.Error(codes.InvalidArgument, "receipt code is required")
	}

	h.logger.Debug("GetReceipt called", zap.String("code", req.Code))

	rec, err := h.receiptUC.GetReceiptByCode(ctx, req.Code)
	if err != nil {
		if err.Error() == "receipt not found" {
			grpcRequestsTotal.WithLabelValues("GetReceipt", "not_found").Inc()
			return nil, status.Error(codes.NotFound, "receipt not found")
		}
		grpcRequestsTotal.WithLabelValues("GetReceipt", "error").Inc()
		return nil, status.Errorf(codes.Internal, "failed to get receipt: %v", err)
	}

	grpcRequestsTotal.WithLabelValues("GetReceipt", "success").Inc()
	return rec.ToProto(), nil
}

// UpdateReceipt - single receipt update
func (h *ReceiptGRPCHandler) UpdateReceipt(
	ctx context.Context,
	req *receiptpb.UpdateReceiptRequest,
) (*receiptpb.UpdateReceiptResponse, error) {
	timer := prometheus.NewTimer(grpcRequestDuration.WithLabelValues("UpdateReceipt"))
	defer timer.ObserveDuration()

	if req.Code == "" {
		grpcRequestsTotal.WithLabelValues("UpdateReceipt", "invalid").Inc()
		return nil, status.Error(codes.InvalidArgument, "receipt code is required")
	}

	h.logger.Debug("UpdateReceipt called", zap.String("code", req.Code))

	// Reuse batch endpoint
	batchReq := &receiptpb.UpdateReceiptsBatchRequest{
		Updates: []*receiptpb.UpdateReceiptRequest{req},
	}

	batchResp, err := h.UpdateReceiptsBatch(ctx, batchReq)
	if err != nil {
		grpcRequestsTotal.WithLabelValues("UpdateReceipt", "error").Inc()
		return nil, err
	}

	if len(batchResp.Receipts) == 0 {
		grpcRequestsTotal.WithLabelValues("UpdateReceipt", "error").Inc()
		return nil, status.Error(codes.Internal, "receipt not updated")
	}

	grpcRequestsTotal.WithLabelValues("UpdateReceipt", "success").Inc()

	return &receiptpb.UpdateReceiptResponse{
		Receipt: batchResp.Receipts[0],
		Updated: true,
	}, nil
}

// ===============================
// BATCH OPERATIONS (OPTIMIZED)
// ===============================

// CreateReceiptsBatch - batch create receipts with validation and rate limiting
func (h *ReceiptGRPCHandler) CreateReceiptsBatch(
	ctx context.Context,
	req *receiptpb.CreateReceiptsBatchRequest,
) (*receiptpb.CreateReceiptsBatchResponse, error) {
	timer := prometheus.NewTimer(grpcRequestDuration.WithLabelValues("CreateReceiptsBatch"))
	defer timer.ObserveDuration()

	if len(req.Receipts) == 0 {
		grpcRequestsTotal.WithLabelValues("CreateReceiptsBatch", "success").Inc()
		return &receiptpb.CreateReceiptsBatchResponse{
			SuccessCount: 0,
			ErrorCount:   0,
		}, nil
	}

	h.logger.Info("CreateReceiptsBatch called",
		zap.Int("count", len(req.Receipts)),
		zap.Bool("fail_on_first_error", req.FailOnFirstError),
	)

	// VALIDATION 1: Check batch size limit
	if len(req.Receipts) > MaxBatchSizePerRequest {
		grpcRequestsTotal.WithLabelValues("CreateReceiptsBatch", "invalid").Inc()
		return nil, status.Errorf(codes.InvalidArgument,
			"batch size %d exceeds maximum %d", len(req.Receipts), MaxBatchSizePerRequest)
	}

	// VALIDATION 2: Extract user ID and check rate limit
	userID := extractUserID(ctx)
	if err := h.receiptUC.CheckRateLimit(ctx, userID, len(req.Receipts)); err != nil {
		rateLimitExceeded.WithLabelValues(userID).Inc()
		grpcRequestsTotal.WithLabelValues("CreateReceiptsBatch", "rate_limited").Inc()
		return nil, status.Error(codes.ResourceExhausted, err.Error())
	}

	// VALIDATION 3: Validate all receipts
	for i, r := range req.Receipts {
		if err := validateCreateReceiptRequest(r); err != nil {
			grpcRequestsTotal.WithLabelValues("CreateReceiptsBatch", "invalid").Inc()
			return nil, status.Errorf(codes.InvalidArgument,
				"invalid receipt at index %d: %v", i, err)
		}
	}

	// Convert proto to domain using helper functions
	receipts := make([]*domain.Receipt, 0, len(req.Receipts))
	for i, r := range req.Receipts {
		metadata := make(map[string]interface{})
		if r.Metadata != nil {
			metadata = r.Metadata.AsMap()
		}

		// Convert amounts (proto uses float64 cents, domain uses float64 dollars)
		amount := float64(r.Amount) / 1.0
		originalAmount := float64(r.OriginalAmount) / 1.0
		transactionCost := float64(r.TransactionCost) / 1.0

		// 🔥 USE CONVERSION HELPERS
		accountType := domain.AccountTypeToString(r.AccountType)
		transactionType := domain.TransactionTypeToString(r.TransactionType)

		// Debug log for first receipt
		if i == 0 {
			h.logger.Debug("converting first receipt",
				zap.String("account_type_proto", r.AccountType.String()),
				zap.String("account_type_domain", accountType),
				zap.String("transaction_type_proto", r.TransactionType.String()),
				zap.String("transaction_type_domain", transactionType),
			)
		}

		receipts = append(receipts, &domain.Receipt{
			TransactionType:   transactionType, // ✅ Converted to string
			CodedType:         r.CodedType,
			AccountType:       accountType, // ✅ Converted to string
			Amount:            amount,
			OriginalAmount:    originalAmount,
			TransactionCost:   transactionCost,
			Currency:          r.Currency,
			OriginalCurrency:  r.OriginalCurrency,
			ExchangeRate:      r.ExchangeRateDecimal,
			ExternalRef:       r.ExternalRef,
			ParentReceiptCode: r.ParentReceiptCode,
			Creditor:          domain.PartyInfoFromProto(r.Creditor),                                                    // ✅ Already converts enums to strings
			Debitor:           domain.PartyInfoFromProto(r.Debitor),                                                     // ✅ Already converts enums to strings
			Status:            domain.TransactionStatusToString(receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING), // Default status
			CreditorStatus:    domain.TransactionStatusToString(receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING),
			DebitorStatus:     domain.TransactionStatusToString(receiptpb.TransactionStatus_TRANSACTION_STATUS_PENDING),
			CreatedBy:         r.CreatedBy,
			Metadata:          metadata,
		})
	}

	// Check for idempotency key
	var created []*domain.Receipt
	var fromCache bool
	var err error

	if req.IdempotencyKey != "" {
		created, fromCache, err = h.receiptUC.CreateReceiptsWithIdempotency(ctx, req.IdempotencyKey, receipts)
	} else {
		created, err = h.receiptUC.CreateReceipts(ctx, receipts)
	}

	if err != nil {
		h.logger.Error("failed to create receipts",
			zap.Error(err),
			zap.Int("count", len(receipts)),
		)
		grpcRequestsTotal.WithLabelValues("CreateReceiptsBatch", "error").Inc()
		return nil, status.Errorf(codes.Internal, "failed to create receipts: %v", err)
	}

	// Convert domain to proto (ToProto() already handles string → enum conversion)
	resp := &receiptpb.CreateReceiptsBatchResponse{
		Receipts:     make([]*receiptpb.Receipt, len(created)),
		Errors:       []*receiptpb.ReceiptError{},
		SuccessCount: int32(len(created)),
		ErrorCount:   0,
		FromCache:    fromCache,
	}
	for i, rc := range created {
		resp.Receipts[i] = rc.ToProto() // ✅ ToProto handles string → enum conversion
	}

	h.logger.Info("receipts created successfully",
		zap.Int("count", len(created)),
		zap.Bool("from_cache", fromCache),
	)

	grpcRequestsTotal.WithLabelValues("CreateReceiptsBatch", "success").Inc()

	return resp, nil
}

// GetReceiptsBatch - batch get receipts (cache-optimized)
func (h *ReceiptGRPCHandler) GetReceiptsBatch(
	ctx context.Context,
	req *receiptpb.GetReceiptsRequest,
) (*receiptpb.GetReceiptsResponse, error) {
	timer := prometheus.NewTimer(grpcRequestDuration.WithLabelValues("GetReceiptsBatch"))
	defer timer.ObserveDuration()

	if len(req.Codes) == 0 {
		grpcRequestsTotal.WithLabelValues("GetReceiptsBatch", "success").Inc()
		return &receiptpb.GetReceiptsResponse{}, nil
	}

	if len(req.Codes) > MaxBatchSizePerRequest {
		grpcRequestsTotal.WithLabelValues("GetReceiptsBatch", "invalid").Inc()
		return nil, status.Errorf(codes.InvalidArgument,
			"batch size %d exceeds maximum %d", len(req.Codes), MaxBatchSizePerRequest)
	}

	h.logger.Debug("GetReceiptsBatch called", zap.Int("count", len(req.Codes)))

	receipts, err := h.receiptUC.GetReceiptsBatch(ctx, req.Codes)
	if err != nil {
		grpcRequestsTotal.WithLabelValues("GetReceiptsBatch", "error").Inc()
		return nil, status.Errorf(codes.Internal, "failed to get receipts: %v", err)
	}

	// Find missing codes
	foundCodes := make(map[string]bool, len(receipts))
	for _, r := range receipts {
		foundCodes[r.Code] = true
	}

	notFoundCodes := []string{}
	for _, code := range req.Codes {
		if !foundCodes[code] {
			notFoundCodes = append(notFoundCodes, code)
		}
	}

	// Convert to proto
	protoReceipts := make([]*receiptpb.Receipt, len(receipts))
	for i, r := range receipts {
		protoReceipts[i] = r.ToProto()
	}

	grpcRequestsTotal.WithLabelValues("GetReceiptsBatch", "success").Inc()

	return &receiptpb.GetReceiptsResponse{
		Receipts:      protoReceipts,
		NotFoundCodes: notFoundCodes,
	}, nil
}

// UpdateReceiptsBatch - batch update receipts
func (h *ReceiptGRPCHandler) UpdateReceiptsBatch(
	ctx context.Context,
	req *receiptpb.UpdateReceiptsBatchRequest,
) (*receiptpb.UpdateReceiptsBatchResponse, error) {
	timer := prometheus.NewTimer(grpcRequestDuration.WithLabelValues("UpdateReceiptsBatch"))
	defer timer.ObserveDuration()

	if len(req.Updates) == 0 {
		grpcRequestsTotal.WithLabelValues("UpdateReceiptsBatch", "success").Inc()
		return &receiptpb.UpdateReceiptsBatchResponse{
			SuccessCount: 0,
			ErrorCount:   0,
		}, nil
	}

	if len(req.Updates) > MaxBatchSizePerRequest {
		grpcRequestsTotal.WithLabelValues("UpdateReceiptsBatch", "invalid").Inc()
		return nil, status.Errorf(codes.InvalidArgument,
			"batch size %d exceeds maximum %d", len(req.Updates), MaxBatchSizePerRequest)
	}

	h.logger.Info("UpdateReceiptsBatch called", zap.Int("count", len(req.Updates)))

	// Convert proto to domain updates
	patches := make([]*domain.ReceiptUpdate, len(req.Updates))
	for i, u := range req.Updates {
		patches[i] = domain.ReceiptUpdateFromProto(u)
	}

	// Update receipts
	updatedReceipts, err := h.receiptUC.UpdateReceiptsBatch(ctx, patches)
	if err != nil {
		h.logger.Error("failed to update receipts",
			zap.Error(err),
			zap.Int("count", len(patches)),
		)
		grpcRequestsTotal.WithLabelValues("UpdateReceiptsBatch", "error").Inc()
		return nil, status.Errorf(codes.Internal, "failed to update receipts: %v", err)
	}

	// Convert to proto
	protoReceipts := make([]*receiptpb.Receipt, len(updatedReceipts))
	for i, r := range updatedReceipts {
		protoReceipts[i] = r.ToProto()
	}

	h.logger.Info("receipts updated successfully", zap.Int("count", len(updatedReceipts)))

	grpcRequestsTotal.WithLabelValues("UpdateReceiptsBatch", "success").Inc()

	return &receiptpb.UpdateReceiptsBatchResponse{
		Receipts:     protoReceipts,
		Errors:       []*receiptpb.ReceiptError{},
		SuccessCount: int32(len(updatedReceipts)),
		ErrorCount:   0,
	}, nil
}

// ===============================
// STREAMING OPERATIONS
// ===============================

// CreateReceiptsStream - streaming receipt creation (high throughput)
func (h *ReceiptGRPCHandler) CreateReceiptsStream(
	stream receiptpb.ReceiptService_CreateReceiptsStreamServer,
) error {
	h.logger.Info("CreateReceiptsStream started")

	count := 0
	batchBuffer := make([]*receiptpb.CreateReceiptRequest, 0, 100)

	// Process stream
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// Process remaining buffer
			if len(batchBuffer) > 0 {
				if err := h.processBatchAndSend(stream, batchBuffer); err != nil {
					return err
				}
				count += len(batchBuffer)
			}

			h.logger.Info("CreateReceiptsStream completed", zap.Int("total_count", count))
			return nil
		}
		if err != nil {
			h.logger.Error("stream receive error", zap.Error(err))
			return status.Errorf(codes.Internal, "stream error: %v", err)
		}

		// Add to buffer
		batchBuffer = append(batchBuffer, req)

		// Process batch when buffer is full
		if len(batchBuffer) >= 100 {
			if err := h.processBatchAndSend(stream, batchBuffer); err != nil {
				return err
			}
			count += len(batchBuffer)
			batchBuffer = make([]*receiptpb.CreateReceiptRequest, 0, 100)

			if count%1000 == 0 {
				h.logger.Info("stream progress", zap.Int("count", count))
			}
		}
	}
}

// processBatchAndSend processes a batch and sends responses
func (h *ReceiptGRPCHandler) processBatchAndSend(
	stream receiptpb.ReceiptService_CreateReceiptsStreamServer,
	batch []*receiptpb.CreateReceiptRequest,
) error {
	// Create batch request
	batchReq := &receiptpb.CreateReceiptsBatchRequest{
		Receipts:         batch,
		FailOnFirstError: false,
	}

	// Process batch
	batchResp, err := h.CreateReceiptsBatch(stream.Context(), batchReq)
	if err != nil {
		return err
	}

	// Send individual responses
	for _, receipt := range batchResp.Receipts {
		resp := &receiptpb.CreateReceiptResponse{
			Receipt:   receipt,
			FromCache: false,
		}
		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}
	}

	return nil
}

// UpdateReceiptsStream - streaming receipt updates
func (h *ReceiptGRPCHandler) UpdateReceiptsStream(
	stream receiptpb.ReceiptService_UpdateReceiptsStreamServer,
) error {
	h.logger.Info("UpdateReceiptsStream started")

	count := 0
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			h.logger.Info("UpdateReceiptsStream completed", zap.Int("total_count", count))
			return nil
		}
		if err != nil {
			h.logger.Error("stream receive error", zap.Error(err))
			return status.Errorf(codes.Internal, "stream error: %v", err)
		}

		// Process single update
		resp, err := h.UpdateReceipt(stream.Context(), req)
		if err != nil {
			h.logger.Error("failed to update receipt in stream",
				zap.Error(err),
				zap.Int("count", count),
			)
			return err
		}

		// Send response
		if err := stream.Send(resp); err != nil {
			h.logger.Error("stream send error", zap.Error(err))
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}

		count++
		if count%100 == 0 {
			h.logger.Debug("stream progress", zap.Int("count", count))
		}
	}
}

// ===============================
// QUERY OPERATIONS
// ===============================

// ListReceipts - query receipts with filters and pagination
func (h *ReceiptGRPCHandler) ListReceipts(
	ctx context.Context,
	req *receiptpb.ListReceiptsRequest,
) (*receiptpb.ListReceiptsResponse, error) {
	timer := prometheus.NewTimer(grpcRequestDuration.WithLabelValues("ListReceipts"))
	defer timer.ObserveDuration()

	h.logger.Debug("ListReceipts called",
		zap.Int("page_size", int(req.PageSize)),
		zap.String("page_token", req.PageToken),
	)

	// Convert proto filters to domain
	filters := domain.FiltersFromProto(req)

	// List receipts
	receipts, err := h.receiptUC.ListReceipts(ctx, filters)
	if err != nil {
		grpcRequestsTotal.WithLabelValues("ListReceipts", "error").Inc()
		return nil, status.Errorf(codes.Internal, "failed to list receipts: %v", err)
	}

	// Count total (if requested)
	var totalCount int64
	if req.IncludeCount {
		totalCount, err = h.receiptUC.CountReceipts(ctx, filters)
		if err != nil {
			h.logger.Warn("failed to count receipts", zap.Error(err))
		}
	}

	// Convert to proto
	protoReceipts := make([]*receiptpb.Receipt, len(receipts))
	for i, r := range receipts {
		protoReceipts[i] = r.ToProto()
	}

	// Generate next page token
	nextPageToken := ""
	if len(receipts) > int(req.PageSize) {
		// We fetched PageSize + 1, so there's a next page
		receipts = receipts[:req.PageSize]
		protoReceipts = protoReceipts[:req.PageSize]
		nextPageToken = repository.GenerateNextPageToken(receipts, int(req.PageSize))
	}

	grpcRequestsTotal.WithLabelValues("ListReceipts", "success").Inc()

	return &receiptpb.ListReceiptsResponse{
		Receipts:      protoReceipts,
		Summaries:     []*receiptpb.ReceiptSummary{}, // TODO: Implement summaries
		NextPageToken: nextPageToken,
		TotalCount:    totalCount,
	}, nil
}

// ===============================
// HEALTH & METRICS
// ===============================

// HealthCheck - check service health
func (h *ReceiptGRPCHandler) HealthCheck(
	ctx context.Context,
	req *receiptpb.HealthCheckRequest,
) (*receiptpb.HealthCheckResponse, error) {
	h.logger.Debug("HealthCheck called", zap.String("service", req.Service))

	// Check usecase health (includes repo and cache)
	if err := h.receiptUC.Health(ctx); err != nil {
		return &receiptpb.HealthCheckResponse{
			Status:  receiptpb.HealthCheckResponse_NOT_SERVING,
			Message: fmt.Sprintf("Service unhealthy: %v", err),
			Metrics: map[string]string{
				"error": err.Error(),
			},
		}, nil
	}

	return &receiptpb.HealthCheckResponse{
		Status:  receiptpb.HealthCheckResponse_SERVING,
		Message: "Receipt service is healthy",
		Metrics: map[string]string{
			"status":  "ok",
			"version": "v3",
		},
	}, nil
}

// GetMetrics - get service metrics
func (h *ReceiptGRPCHandler) GetMetrics(
	ctx context.Context,
	req *receiptpb.GetMetricsRequest,
) (*receiptpb.GetMetricsResponse, error) {
	h.logger.Debug("GetMetrics called")

	// Get metrics from usecase
	fromTime := req.FromTimestamp.AsTime()
	toTime := req.ToTimestamp.AsTime()

	metrics, err := h.receiptUC.GetMetrics(ctx, fromTime, toTime)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get metrics: %v", err)
	}

	return &receiptpb.GetMetricsResponse{
		TotalReceipts:     metrics.TotalReceipts,
		ReceiptsPerSecond: metrics.ReceiptsPerSecond,
		AvgCreationTimeMs: metrics.AvgCreationTimeMs,
		P95CreationTimeMs: metrics.P95CreationTimeMs,
		P99CreationTimeMs: metrics.P99CreationTimeMs,
		ReceiptsByType:    metrics.ReceiptsByType,
		ReceiptsByStatus:  metrics.ReceiptsByStatus,
		CacheHitRate:      0.0, // TODO: Get from cache service
	}, nil
}

// ===============================
// HELPER FUNCTIONS
// ===============================

// extractUserID extracts user ID from gRPC context metadata
func extractUserID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "anonymous"
	}

	// Try different metadata keys
	if userIDs := md.Get("user-id"); len(userIDs) > 0 {
		return userIDs[0]
	}
	if userIDs := md.Get("x-user-id"); len(userIDs) > 0 {
		return userIDs[0]
	}
	if clientIDs := md.Get("client-id"); len(clientIDs) > 0 {
		return clientIDs[0]
	}

	return "anonymous"
}

// validateCreateReceiptRequest validates a create receipt request
func validateCreateReceiptRequest(req *receiptpb.CreateReceiptRequest) error {
	if req.TransactionType == receiptpb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED {
		return fmt.Errorf("transaction_type is required")
	}

	if req.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}

	if req.Creditor == nil {
		return fmt.Errorf("creditor is required")
	}

	if req.Debitor == nil {
		return fmt.Errorf("debitor is required")
	}

	if req.Creditor.AccountId == req.Debitor.AccountId {
		return fmt.Errorf("creditor and debitor accounts must be different")
	}

	// Validate conversion fields
	if req.TransactionType == receiptpb.TransactionType_TRANSACTION_TYPE_CONVERSION {
		if req.OriginalCurrency == "" {
			return fmt.Errorf("original_currency is required for conversion")
		}
		exchangeRate, err := strconv.ParseFloat(req.ExchangeRateDecimal, 64)
		if err != nil || exchangeRate <= 0 {
			return fmt.Errorf("exchange_rate must be a positive number for conversion")
		}
	}

	return nil
}

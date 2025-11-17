package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"receipt-service/internal/config"
	hgrpc "receipt-service/internal/handler/grpc"
	"receipt-service/internal/repository"
	"receipt-service/internal/usecase"
	"receipt-service/pkg/cache"
	"receipt-service/pkg/generator"
	"receipt-service/pkg/utils"

	receiptpb "x/shared/genproto/shared/accounting/receipt/v3"
	notificationclient "x/shared/notification"
	"x/shared/utils/id"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// NewReceiptGRPCServer starts the gRPC server for receipt service (OPTIMIZED FOR 4000+ TPS)
func NewReceiptGRPCServer(cfg config.AppConfig) error {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("🚀 Starting Receipt Service v3 (OPTIMIZED)",
		zap.String("grpc_addr", cfg.GRPCAddr),
		zap.Int64("machine_id", cfg.MachineID),
		zap.String("performance_target", "4000+ receipts/sec"),
	)

	// --- DB connection (OPTIMIZED: 100 connections with statement caching) ---
	dbpool, err := config.ConnectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dbpool.Close()
	logger.Info("✅ Database connected (OPTIMIZED)",
		zap.Int32("max_conns", dbpool.Config().MaxConns),
		zap.Int32("min_conns", dbpool.Config().MinConns),
		zap.String("statement_cache", "enabled"),
	)

	// --- Redis client (OPTIMIZED: Used for caching + rate limiting) ---
	rdb := redis.NewClient(&redis.Options{
		Addr:            cfg.RedisAddr,
		Password:        cfg.RedisPass,
		DB:              cfg.RedisDB,
		PoolSize:        100,           // Increased pool size
		MinIdleConns:    10,
		MaxRetries:      3,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolTimeout:     4 * time.Second,
		ConnMaxIdleTime: 5 * time.Minute,   // Optimized
		ConnMaxLifetime: 30 * time.Minute,  // Optimized
	})
	defer rdb.Close()

	// Test Redis connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cacheService *cache.CacheService
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Warn("⚠️ Redis connection failed, caching and rate limiting disabled", zap.Error(err))
		cacheService = nil // Service will work without Redis (degraded mode)
	} else {
		// OPTIMIZATION: Wrap Redis client in cache service
		cacheService = cache.NewCacheServiceFromClient(rdb, logger)
		logger.Info("✅ Redis connected with cache service (OPTIMIZED)",
			zap.String("addr", cfg.RedisAddr),
			zap.Int("db", cfg.RedisDB),
			zap.Int("pool_size", 100),
			zap.String("features", "caching+rate_limiting+distributed_locks"),
		)
	}

	// --- Snowflake ID generator ---
	sf, err := id.NewSnowflake(cfg.MachineID)
	if err != nil {
		return fmt.Errorf("failed to init snowflake: %w", err)
	}
	logger.Info("✅ Snowflake ID generator initialized",
		zap.Int64("machine_id", cfg.MachineID),
		zap.Int64("max_machine_id", 1023),
	)

	// --- Receipt code generator (ENHANCED VERSION V2) ---
	codeGenV2 := receiptutil.NewReceiptGenerator(sf)
	logger.Info("✅ Receipt code generator V2 initialized",
		zap.String("format", "RCP-YYYY-############"),
	)

	// --- Legacy generator (fallback only) ---
	codeGenLegacy := generator.NewGenerator()
	logger.Info("✅ Legacy receipt generator initialized (fallback only)")

	// --- Repositories (OPTIMIZED: Cache integration) ---
	receiptRepo := repository.NewReceiptRepo(dbpool, cacheService, logger)
	logger.Info("✅ Repository initialized (OPTIMIZED)",
		zap.String("optimizations", "cache-first_reads+bulk_inserts+temp_tables"),
	)

	// --- Notification client ---
	notificationCli := notificationclient.NewNotificationService()
	logger.Info("✅ Notification client initialized")

	// --- Kafka writer (OPTIMIZED: Async with batching) ---
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Topic:        cfg.KafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,        // Wait for leader acknowledgment
		Async:        true,                     // OPTIMIZED: Async for non-blocking writes
		MaxAttempts:  3,                        // Retry up to 3 times
		BatchSize:    100,                      // Batch up to 100 messages
		BatchTimeout: 10 * time.Millisecond,   // Send batch every 10ms
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Compression:  kafka.Snappy,             // Enable compression
		Logger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			logger.Debug(fmt.Sprintf(msg, args...))
		}),
	}
	defer writer.Close()
	logger.Info("✅ Kafka writer initialized (OPTIMIZED)",
		zap.Strings("brokers", cfg.KafkaBrokers),
		zap.String("topic", cfg.KafkaTopic),
		zap.Int("batch_size", 100),
		zap.String("mode", "async"),
		zap.String("compression", "snappy"),
	)

	// --- Usecases (OPTIMIZED: With cache and rate limiting) ---
	receiptUC := usecase.NewReceiptUsecase(
		receiptRepo,
		cacheService,     // ADDED: Cache service for rate limiting
		codeGenLegacy,    // Legacy fallback
		codeGenV2,        // Primary generator (V2)
		notificationCli,
		writer,
		logger,
	)
	logger.Info("✅ Use case initialized (OPTIMIZED)",
		zap.String("features", "batch_splitting+rate_limiting+idempotency"),
	)

	// --- gRPC Handler (OPTIMIZED: With validation and metrics) ---
	receiptHandler := hgrpc.NewReceiptGRPCHandler(receiptUC, logger)
	logger.Info("✅ gRPC handler initialized (OPTIMIZED)",
		zap.String("features", "validation+rate_limiting+metrics+batch_limits"),
	)

	// --- gRPC Server with production settings (OPTIMIZED) ---
	grpcServer := grpc.NewServer(
		// Message size limits
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB

		// Connection settings (OPTIMIZED: Support 1000+ concurrent streams)
		grpc.MaxConcurrentStreams(1000),

		// Keepalive settings (prevent connection drops)
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Minute,
			Time:                  5 * time.Minute,
			Timeout:               1 * time.Minute,
		}),

		// Keepalive enforcement policy
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	// Register service (V3 - UPDATED)
	receiptpb.RegisterReceiptServiceServer(grpcServer, receiptHandler)
	reflection.Register(grpcServer)

	logger.Info("✅ gRPC services registered",
		zap.String("service", "accounting.receipt.v3.ReceiptService"),
		zap.Int("max_concurrent_streams", 1000),
	)

	// --- Start listening ---
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", cfg.GRPCAddr, err)
	}

	logger.Info("🚀 Receipt gRPC server listening (OPTIMIZED FOR 4000+ TPS)",
		zap.String("addr", cfg.GRPCAddr),
		zap.String("version", "v3"),
		zap.String("performance_target", "4000+ receipts/sec"),
		zap.String("features", "cache+rate_limit+batch_ops+metrics"),
	)

	// Serve (blocking)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}
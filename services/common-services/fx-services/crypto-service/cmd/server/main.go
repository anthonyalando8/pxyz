// cmd/server/main.go
package main

import (
	"context"
	"crypto-service/internal/chains/bitcoin"
	registry "crypto-service/internal/chains/registry"
	"crypto-service/internal/chains/tron"
	"crypto-service/internal/config"
	"crypto-service/internal/handler"
	"crypto-service/internal/repository"
	"crypto-service/internal/security"
	"crypto-service/internal/server"
	"crypto-service/internal/usecase"
	"crypto-service/internal/worker"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load . env file
	_ = godotenv.Load()

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Crypto Service")

	// Load configuration
	cfg, err := config.Load(logger)
	if err != nil {
		logger. Fatal("Failed to load config", zap.Error(err))
	}

	// Initialize database connection
	dbPool, err := initDatabase()
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	// Initialize encryption
	encryption, err := security.NewEncryption(cfg.Security.MasterKey)
	if err != nil {
		logger.Fatal("Failed to initialize encryption", zap. Error(err))
	}

	// Initialize repositories
	walletRepo := repository.NewCryptoWalletRepository(dbPool)
	transactionRepo := repository.NewCryptoTransactionRepository(dbPool)
	depositRepo := repository.NewCryptoDepositRepository(dbPool)

	// Initialize blockchain registry
	chainRegistry := registry.NewRegistry()

	// Register TRON chain
	logger.Info("Initializing TRON chain",
		zap.String("network", cfg.Tron.Network),
	)

	tronChain, err := tron.NewTronChain(
		cfg.Tron.APIKey,
		cfg.Tron.Network,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to initialize TRON chain", zap.Error(err))
	}

	//  Register TRON (it already implements domain.Chain interface)
	chainRegistry.Register(tronChain)

	// Register Bitcoin chain
	logger.Info("Initializing Bitcoin chain", zap.String("network", cfg.Bitcoin.Network))
	bitcoinChain, err := bitcoin.NewBitcoinChain(
		cfg.Bitcoin.RPCURL,
		cfg.Bitcoin. APIKey,
		cfg.Bitcoin.Network,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to initialize Bitcoin chain", zap. Error(err))
	}
	chainRegistry.Register(bitcoinChain)

	// TODO: Register Ethereum chain
	// ethereumChain, err := ethereum. NewEthereumChain(cfg.Ethereum, logger)
	// if err != nil {
	//     logger.Fatal("Failed to initialize Ethereum chain", zap.Error(err))
	// }
	// chainRegistry.Register(ethereumChain)

	registeredChains := chainRegistry.List()
	logger.Info("Registered blockchain chains",
		zap.Int("count", len(registeredChains)),
		zap. Strings("chains", registeredChains),
	)
	// ✅ Initialize system usecase
	systemUsecase := usecase.NewSystemUsecase(walletRepo, chainRegistry, encryption, logger)

	// ✅ Initialize system wallets (create if not exist)
	ctx := context.Background()
	if err := systemUsecase.InitializeSystemWallets(ctx); err != nil {
		logger. Error("Failed to initialize system wallets", zap.Error(err))
		// Don't fatal - service can still run, just log the error
	}

	// Initialize use cases
	walletUsecase := usecase.NewWalletUsecase(walletRepo, chainRegistry, encryption, logger)
	transactionUsecase := usecase.NewTransactionUsecase(transactionRepo, walletRepo, chainRegistry, encryption, systemUsecase, logger)
	depositUsecase := usecase.NewDepositUsecase(depositRepo, walletRepo, transactionRepo, chainRegistry, logger)

	// Initialize handlers
	walletHandler := handler. NewWalletHandler(walletUsecase, systemUsecase, logger)
	transactionHandler := handler.NewTransactionHandler(transactionUsecase, logger)
	depositHandler := handler.NewDepositHandler(depositUsecase, logger)
	cryptoHandler := handler.NewCryptoHandler(chainRegistry, walletUsecase, systemUsecase, logger)

	// Get gRPC port from environment or use default
	grpcPort := getEnvAsInt("GRPC_PORT", 8028)

	// Initialize gRPC server
	grpcServer := server.NewGRPCServer(
		walletHandler,
		transactionHandler,
		depositHandler,
		cryptoHandler,
		logger,
		grpcPort,
	)

	// Start deposit monitor worker
	depositMonitor := worker.NewDepositMonitor(depositUsecase, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go depositMonitor.Start(ctx)

	// Start gRPC server in goroutine
	go func() {
		logger.Info("Starting gRPC server", zap.Int("port", grpcPort))
		if err := grpcServer. Start(); err != nil {
			logger.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	logger.Info("Crypto Service started successfully",
		zap.Int("grpc_port", grpcPort),
		zap.String("tron_network", cfg.Tron.Network),
	)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall. SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")

	// Stop workers
	depositMonitor.Stop()

	// Stop TRON chain
	if err := tronChain.Stop(); err != nil {
		logger.Error("Failed to stop TRON chain", zap.Error(err))
	}

	// Stop gRPC server
	grpcServer.Stop()

	logger.Info("Crypto Service stopped")
}

// initDatabase initializes database connection pool
func initDatabase() (*pgxpool.Pool, error) {
	// Get database config from environment
	connString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "212.95.35.81"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "sam"),
		getEnv("DB_PASSWORD", "Kenya_2025!"),
		getEnv("DB_NAME", "pxyz_fx_crypto"),
		getEnv("DB_SSLMODE", "disable"),
	)

	poolConfig, err := pgxpool. ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Set pool config
	poolConfig.MaxConns = int32(getEnvAsInt("DB_MAX_CONNS", 25))
	poolConfig.MinConns = int32(getEnvAsInt("DB_MIN_CONNS", 5))

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt. Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool. Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	var value int
	_, err := fmt.Sscanf(valueStr, "%d", &value)
	if err != nil {
		return defaultValue
	}

	return value
}
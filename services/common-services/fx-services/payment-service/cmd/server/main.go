// cmd/server/main.go
package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "payment-service/config"
    "payment-service/internal/handler"
    "payment-service/internal/provider/mpesa"
    "payment-service/internal/repository"
    "payment-service/internal/router"
    "payment-service/internal/usecase"
    "payment-service/pkg/client"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "go.uber.org/zap"
)

func main() {
    // Initialize logger
    logger, err := zap.NewProduction()
    if err != nil {
        panic(fmt.Sprintf("failed to initialize logger: %v", err))
    }
    defer logger. Sync()

    logger.Info("starting payment service")

    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        logger.Fatal("failed to load configuration", zap.Error(err))
    }

    logger.Info("configuration loaded",
        zap.String("environment", cfg.Server. Env),
        zap.String("port", cfg.Server.Port))

    // Connect to database
    dbConnStr := fmt.Sprintf(
        "postgres://%s:%s@%s:%s/%s?sslmode=%s",
        cfg.Database. User,
        cfg.Database.Password,
        cfg.Database.Host,
        cfg.Database.Port,
        cfg.Database.DBName,
        cfg.Database. SSLMode,
    )

    dbPool, err := pgxpool. New(context.Background(), dbConnStr)
    if err != nil {
        logger.Fatal("failed to connect to database", zap. Error(err))
    }
    defer dbPool.Close()

    logger.Info("connected to database",
        zap. String("database", cfg.Database.DBName))

    // Initialize repositories
    paymentRepo := repository.NewPaymentRepository(dbPool)
    providerTxRepo := repository.NewProviderTransactionRepository(dbPool)

    // Initialize providers
    mpesaProvider := mpesa.NewMpesaProvider(cfg.Mpesa)

    // Initialize clients
    partnerClient := client.NewPartnerClient(cfg.Partner, logger)
	partnerCreditClient := client.NewPartnerCreditClient(cfg. Partner, logger)

    // Initialize usecases
    paymentUC := usecase.NewPaymentUsecase(
        paymentRepo,
        providerTxRepo,
        mpesaProvider,
        cfg.Partner,
        logger,
    )

    callbackUC := usecase.NewCallbackUsecase(
        paymentRepo,
        providerTxRepo,
        mpesaProvider,
        partnerClient,
		partnerCreditClient,
        logger,
    )

    // Initialize handlers
    webhookHandler := handler.NewWebhookHandler(paymentUC, logger)
    callbackHandler := handler.NewCallbackHandler(callbackUC, logger)

    // Setup routes
    r := router.SetupRoutes(webhookHandler, callbackHandler, logger)

    // Create HTTP server
    srv := &http.Server{
        Addr:         ":" + cfg.Server.Port,
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Start server in goroutine
    go func() {
        logger.Info("server starting", zap.String("port", cfg.Server.Port))
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatal("failed to start server", zap.Error(err))
        }
    }()

    logger.Info("payment service started successfully",
        zap.String("port", cfg.Server.Port),
        zap.String("environment", cfg.Server.Env))

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall. SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("shutting down server...")

    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        logger.Error("server forced to shutdown", zap.Error(err))
    }

    logger.Info("server stopped")
}
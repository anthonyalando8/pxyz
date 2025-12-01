package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/handler"
	"auth-service/internal/repository"
	"auth-service/internal/router"
	telegramclient "auth-service/internal/service/telegram"
	"auth-service/internal/usecase"
	"auth-service/internal/ws"

	accountclient "x/shared/account"
	"auth-service/pkg/kafka"
	"x/shared/auth/middleware"
	otpclient "x/shared/auth/otp"
	coreclient "x/shared/core"
	emailclient "x/shared/email"
	notificationclient "x/shared/notification"
	smsclient "x/shared/sms"
	urbacservice "x/shared/urbac/utils"
	oauthmid "auth-service/pkg/middleware"

	"x/shared/utils/cache"

	"x/shared/utils/id"

	authpb "x/shared/genproto/authpb"

	oauths "auth-service/internal/service/app_oauth2_client"
	accountingclient "x/shared/common/accounting" // 👈 added

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewServer(cfg config.AppConfig) *http.Server {
	db, _ := config.ConnectDB()

	userRepo := repository.NewUserRepository(db)
	sf, err := id.NewSnowflake(8)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}
	cache := cache.NewCache([]string{cfg.RedisAddr}, cfg.RedisPass, false)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	auth := middleware.RequireAuth()
	otpSvc := otpclient.NewOTPService()
	accountClient := accountclient.NewAccountClient()
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	coreClient := coreclient.NewCoreService()
	notificationCli := notificationclient.NewNotificationService()

	config := &handler.Config{GoogleClientID: cfg.GoogleClientID, Apple: cfg.Apple}
	telegramClient := telegramclient.NewTelegramClient(cfg.TelegramBotToken)
	urbacSvc := urbacservice.NewService(auth.RBACClient, rdb)

	accountingClient := accountingclient.NewAccountingClient()

	authEvPub := ws.NewAuthEventPublisher(rdb)

	producer, err := kafka.NewUserRegistrationProducer(cfg.KafkaBrokers)
	if err != nil {
		log.Fatal("Failed to create Kafka producer:", err)
	}

	userUC := usecase.NewUserUsecase(userRepo, sf, cache, producer, otpSvc)

	oauth2Svc := oauths.NewOAuth2Service(userRepo, cache)
	oauthMid := oauthmid.NewOAuth2Middleware(oauth2Svc)

	authHandler := handler.NewAuthHandler(
		userUC,
		auth,
		otpSvc,
		accountClient,
		emailCli,
		smsCli,
		rdb,
		coreClient,
		notificationCli,
		urbacSvc,
		auth.Client,
		config,
		telegramClient,
		oauth2Svc,
		authEvPub,
		accountingClient,
	)

	grpcAuthHandler := handler.NewGRPCAuthHandler(
		userUC,
		otpSvc,
		accountClient,
		emailCli,
		smsCli,
		notificationCli,
		config,
		telegramClient,
	)
	// Initialize OAuth2 handler
    oauthHandler := handler.NewOAuth2Handler(oauth2Svc, userUC, authHandler)

	consumer, err := kafka.NewUserRegistrationConsumer(
		cfg.KafkaBrokers,
		"user-registration-consumer-group",
		userUC,
		producer,
	)
	if err != nil {
		log.Fatal("Failed to create Kafka consumer:", err)
	}

	dlqConsumer, err := kafka.NewDLQConsumer(
		cfg.KafkaBrokers,
		"user-registration-dlq-consumer-group",
		userUC,
		producer,
	)
	if err != nil {
		log.Fatal("Failed to create DLQ consumer:", err)
	}

	// Create a background context for long-running services
	// This context will NOT be cancelled until explicit shutdown signal
	bgCtx := context.Background()

	// Setup graceful shutdown signal handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start graceful shutdown handler in background
	go func() {
		<-sigChan
		log.Println("🛑 Shutdown signal received, initiating graceful shutdown...")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = shutdownCtx
		defer cancel()

		// Stop consumers
		log.Println("Stopping Kafka consumers...")
		consumer.Close()
		dlqConsumer.Close()

		// Stop producer
		log.Println("Stopping Kafka producer...")
		producer.Close()

		// Close Redis
		log.Println("Closing Redis connection...")
		if err := rdb.Close(); err != nil {
			log.Printf("Error closing Redis: %v", err)
		}

		// Close database
		log.Println("Closing database connection...")
		db.Close()

		log.Println("✅ Graceful shutdown complete")
		
		// Give a moment for logs to flush
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()

	// Start main consumer
	go func() {
		log.Println("Starting main consumer...")
		if err := consumer.Start(bgCtx); err != nil {
			log.Printf("Main consumer error: %v", err)
		}
	}()

	// Start DLQ consumer
	go func() {
		log.Println("Starting DLQ consumer...")
		if err := dlqConsumer.Start(bgCtx); err != nil {
			log.Printf("DLQ consumer error: %v", err)
		}
	}()

	log.Println("✅ Kafka consumers started successfully")

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
		}

		grpcServer := grpc.NewServer()
		authpb.RegisterAuthServiceServer(grpcServer, grpcAuthHandler)
		reflection.Register(grpcServer)

		log.Printf("🚀 Auth gRPC server listening at %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// Start WebSocket hub
	wsServer := ws.NewServer()
	wsServer.Start()

	go ws.ListenAuthEvents(bgCtx, rdb, wsServer.Hub())

	wsHandler := handler.NewWSHandler(wsServer)

	// Setup HTTP routes
	r := chi.NewRouter()
	r = router.SetupRoutes(r, authHandler, oauthHandler, auth, wsHandler, cache, rdb).(*chi.Mux)
	router.SetupOAuth2Routes(r, oauthHandler, auth, oauthMid)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}
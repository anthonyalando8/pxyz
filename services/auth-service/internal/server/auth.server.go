package server

import (
	"context"
	"log"
	"net"
	"net/http"

	"auth-service/internal/config"
	"auth-service/internal/handler"
	"auth-service/internal/repository"
	"auth-service/internal/router"
	telegramclient "auth-service/internal/service/telegram"
	"auth-service/internal/usecase"
	"auth-service/internal/ws"

	accountclient "x/shared/account"
	//authclient "x/shared/auth"
	"x/shared/auth/middleware"
	otpclient "x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	coreclient "x/shared/core"
	notificationclient "x/shared/notification" // ✅ added
	urbacservice "x/shared/urbac/utils"

	"x/shared/utils/id"

	authpb "x/shared/genproto/authpb"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewServer(cfg config.AppConfig) *http.Server {
	db, _ := config.ConnectDB()

	userRepo := repository.NewUserRepository(db)
	sf, err := id.NewSnowflake(8) // Node ID 1 for this service
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	userUC := usecase.NewUserUsecase(userRepo, sf)

	auth := middleware.RequireAuth()
	otpSvc := otpclient.NewOTPService()
	accountClient := accountclient.NewAccountClient()
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	coreClient := coreclient.NewCoreService()
	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client

	config := &handler.Config{GoogleClientID: cfg.GoogleClientID, Apple: cfg.Apple}
	telegramClient := telegramclient.NewTelegramClient(cfg.TelegramBotToken)
	urbacSvc := urbacservice.NewService(auth.RBACClient, rdb)

	// HTTP handler
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
	)

	// gRPC handler
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

	// start gRPC server in background
	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
		}

		grpcServer := grpc.NewServer()
		authpb.RegisterAuthServiceServer(grpcServer, grpcAuthHandler)

		// enable reflection in dev
		reflection.Register(grpcServer)

		log.Printf("Auth gRPC server listening at %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// start WebSocket hub
	wsServer := ws.NewServer()
	wsServer.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ws.ListenAuthEvents(ctx, rdb, wsServer.Hub())

	wsHandler := handler.NewWSHandler(wsServer)

	// HTTP routes
	r := chi.NewRouter()
	r = router.SetupRoutes(r, authHandler, auth, wsHandler, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}

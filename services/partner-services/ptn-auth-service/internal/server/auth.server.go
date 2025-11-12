package server

import (
	"context"
	"log"
	"net"
	"net/http"

	"ptn-auth-service/internal/config"
	"ptn-auth-service/internal/handler"
	"ptn-auth-service/internal/repository"
	"ptn-auth-service/internal/router"
	"ptn-auth-service/internal/usecase"
	"ptn-auth-service/internal/ws"
	notificationclient "x/shared/notification" // ✅ added
	urbacservice "x/shared/factory/partner/urbac/utils"
	urbac "x/shared/factory/partner/urbac"

	//authclient "x/shared/auth"
	"x/shared/auth/middleware"
	otpclient "x/shared/auth/otp"
	coreclient "x/shared/core"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"

	"x/shared/utils/id"

	authpb "x/shared/genproto/partner/authpb"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewServer(cfg config.AppConfig) *http.Server {
	db, _ := config.ConnectDB()

	userRepo := repository.NewUserRepository(db)
	sf, err := id.NewSnowflake(5) // Node ID 1 for this service
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	userUC := usecase.NewUserUsecase(userRepo, sf)

	urbacCli := urbac.NewRBACService()
	urbacSvc :=	urbacservice.NewService(urbacCli.Client, rdb)

	//ctx := context.Background()
	

	auth := middleware.RequireAuth()
	otpSvc := otpclient.NewOTPService()
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	coreClient := coreclient.NewCoreService()
	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client

	authHandler := handler.NewAuthHandler(
		userUC, auth, otpSvc, emailCli, smsCli, rdb, coreClient, auth.PartnerClient, notificationCli,urbacSvc,
	)

	// gRPC handler
	grpcAuthHandler := handler.NewGRPCAuthHandler(
		userUC, otpSvc, emailCli, smsCli, authHandler,
	)

	// start gRPC server in background
	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
		}

		grpcServer := grpc.NewServer()
		authpb.RegisterPartnerAuthServiceServer(grpcServer, grpcAuthHandler)

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

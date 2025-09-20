package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"admin-auth-service/internal/config"
	"admin-auth-service/internal/handler"
	"admin-auth-service/internal/repository"
	"admin-auth-service/internal/router"
	"admin-auth-service/internal/usecase"
	"admin-auth-service/internal/ws"
	//authclient "x/shared/auth"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	coreclient "x/shared/core"
	notificationclient "x/shared/notification" // ✅ added

	"x/shared/utils/id"
	"x/shared/utils/errors"

	authpb "x/shared/genproto/admin/authpb"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewServer(cfg config.AppConfig) *http.Server {
	db, _ := config.ConnectDB()

	userRepo := repository.NewUserRepository(db)
	sf, err := id.NewSnowflake(1) // Node ID 1 for this service
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	userUC := usecase.NewUserUsecase(userRepo, sf)

	ctx := context.Background()
	if err := seedSystemAdmin(ctx, userUC, cfg); err != nil {
		log.Printf("Warning: failed to seed system admin: %v", err)
	} else {
		log.Println("System admin seeding complete")
	}


	auth := middleware.RequireAuth()
	otpSvc := otpclient.NewOTPService()
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	coreClient := coreclient.NewCoreService()
	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client

	authHandler := handler.NewAuthHandler(
		userUC, auth, otpSvc, emailCli, smsCli, rdb, coreClient,auth.AdminClient, notificationCli,
	)

	// gRPC handler
	grpcAuthHandler := handler.NewGRPCAuthHandler(
		userUC, otpSvc, emailCli, smsCli,
	)

	// start gRPC server in background
	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
		}

		grpcServer := grpc.NewServer()
		authpb.RegisterAdminAuthServiceServer(grpcServer, grpcAuthHandler)

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


func seedSystemAdmin(ctx context.Context, uc *usecase.UserUsecase, cfg config.AppConfig) error {
	adminEmail := cfg.SystemAdminEmail
	adminPassword := cfg.SystemAdminPassword
	if adminEmail == "" || adminPassword == "" {
		log.Println("System admin email or password not set, skipping seeding")
		return nil
	}

	// Check if user already exists
	existingUser, err := uc.FindUserByIdentifier(ctx, adminEmail)
	if err != nil && !errors.Is(err, xerrors.ErrUserNotFound) {
		// Only fail for unexpected errors
		log.Printf("Warning: failed to check existing system admin: %v\n", err)
	} 

	if existingUser != nil {
		log.Println("System admin already exists, skipping seeding")
		return nil
	}

	// Attempt to create system admin
	user, err := uc.RegisterUser(ctx, adminEmail, adminPassword, "System", "Admin", "system_admin")
	if err != nil {
		return fmt.Errorf("failed to create system admin: %w", err)
	}

	log.Printf("Seeded system admin: %s (id=%s)\n", adminEmail, user.ID)
	return nil
}


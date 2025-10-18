package server

import (
	"log"
	"net"
	"net/http"
	"time"

	"notification-service/internal/config"
	hgrpc "notification-service/internal/handler/grpc"
	hrest "notification-service/internal/handler/http"
	wshandler "notification-service/internal/handler/ws"
	"notification-service/internal/repository"
	"notification-service/internal/router"
	"notification-service/internal/usecase"
	"notification-service/pkg/notifier"
	ws "notification-service/pkg/notifier/ws"
	"notification-service/pkg/template"

	"x/shared/auth/middleware"
	email "x/shared/email"
	notificationpb "x/shared/genproto/shared/notificationpb"
	sms "x/shared/sms"
	authclient "x/shared/auth"

	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewServer(cfg config.AppConfig) *http.Server {
	// --- DB connection ---
	dbpool, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// --- Init repos ---
	notifRepo := repository.NewRepository(dbpool)

	// --- Snowflake ID generator ---
	sf, err := id.NewSnowflake(14)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}
	_ = sf

	// --- Redis ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	authClient, err := authclient.DialAuthService(authclient.AllAuthServices)
	if err != nil {
        log.Fatalf("failed to dial auth service: %v", err)
    }

	// --- Auth middleware ---
	auth := middleware.RequireAuth()

	// --- Clients ---
	emailClient := email.NewEmailClient()
	smsClient := sms.NewSMSClient()

	// --- WS manager and handler ---
	wsManager := ws.NewManager()
	go wsManager.Heartbeat(30 * time.Second)
	wsHandler := wshandler.NewWSHandler(wsManager)

	// --- Notifier ---
	// Initialize template service
	tmplService := template.NewTemplateService(
		"./templates/email",
		"./templates/sms",
		"./templates/whatsapp",
	)

	// Initialize notifier with all clients + templates
	notif := notifier.NewNotifier(emailClient, smsClient, wsManager, tmplService)


	// --- Usecases ---
	uc := usecase.NewNotificationUsecase(notifRepo, notif, authClient)

	// --- Handlers ---
	restHandler := hrest.NewNotificationHandler(uc, emailClient, smsClient)
	grpcHandler := hgrpc.NewNotificationGRPCHandler(uc)

	// --- HTTP routes ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, restHandler, wsHandler, auth, rdb).(*chi.Mux)

	// --- gRPC server ---
	grpcServer := grpc.NewServer()
	notificationpb.RegisterNotificationServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
		}
		log.Printf("gRPC server listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// --- HTTP server ---
	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}

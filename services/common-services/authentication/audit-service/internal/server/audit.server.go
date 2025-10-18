package server

import (
	"audit-service/internal/config"
	"audit-service/internal/handler"
	"audit-service/internal/repository"
	"audit-service/internal/service/audit"
	"audit-service/internal/service/kafka"
	"audit-service/internal/service/workers"
	"audit-service/internal/service/ws"
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	emailclient "x/shared/email"
	pb "x/shared/genproto/authentication/audit-service/grpcpb"
	notificationclient "x/shared/notification"
	smsclient "x/shared/sms"

	grpcServer "google.golang.org/grpc"

	"github.com/go-chi/chi/v5"
	//"github.com/jackc/pgx/v5/pgxpool"
)

func NewServer(cfg config.AppConfig) {
	// Initialize database connection
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	userRepo := repository.NewUserRepository(db)

	// Initialize services
	//auth := middleware.RequireAuth()
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	notificationCli := notificationclient.NewNotificationService()

	geoIPService := initGeoIPService() // Your GeoIP implementation

	auditService := service.NewSecurityAuditService(userRepo, geoIPService)

	// ================================
	// SETUP GRPC SERVER
	// ================================
	securityHandler := handler.NewSecurityAuditGRPCServer(auditService,/* auth, */emailCli, smsCli, notificationCli)
	
	grpcSrv := grpcServer.NewServer()
	pb.RegisterSecurityAuditServiceServer(grpcSrv, securityHandler)

	go func() {
		listener, err := net.Listen("tcp", cfg.GRPCAddress)
		if err != nil {
			log.Fatalf("Failed to listen for gRPC: %v", err)
		}
		log.Printf("gRPC server listening on %s", cfg.GRPCAddress)
		if err := grpcSrv.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// ================================
	// SETUP KAFKA
	// ================================
	
	// Kafka Producer
	kafkaProducer, err := kafka.NewSecurityAuditProducer(cfg.KafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	// Kafka Consumer
	kafkaConsumer, err := kafka.NewSecurityAuditConsumer(
		cfg.KafkaBrokers,
		"security-audit-consumer-group",
		auditService,
	)
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %v", err)
	}
	defer kafkaConsumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Kafka consumer
	go func() {
		if err := kafkaConsumer.Start(ctx); err != nil {
			log.Printf("Kafka consumer error: %v", err)
		}
	}()

	// ================================
	// SETUP WEBSOCKET
	// ================================
	
	wsHub := websocket.NewHub()
	go wsHub.Run()

	notifier := websocket.NewSecurityAuditNotifier(wsHub, auditService)

	// ================================
	// SETUP BACKGROUND WORKERS
	// ================================
	
	auditWorkers := workers.NewSecurityAuditWorkers(auditService, notifier)
	auditWorkers.Start()
	defer auditWorkers.Stop()

	// ================================
	// SETUP HTTP SERVER (for WebSocket)
	// ================================
	
	r := chi.NewRouter()

	// WebSocket endpoint
	r.Get("/ws/security-audit", func(w http.ResponseWriter, req *http.Request) {
		websocket.ServeWebSocket(wsHub, w, req)
	})

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddress,
		Handler: r,
	}

	go func() {
		log.Printf("HTTP server listening on %s", cfg.HTTPAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// ================================
	// GRACEFUL SHUTDOWN
	// ================================
	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Stop gRPC server
	grpcSrv.GracefulStop()

	// Cancel context for workers
	cancel()

	log.Println("Server stopped gracefully")
}

func initGeoIPService() service.GeoIPService {
	geoip, err := service.NewGeoIPService("/usr/local/share/GeoIP/GeoLite2-City.mmdb")
	if err != nil {
		log.Printf("GeoIP init failed: %v", err)
		return nil
	}
	return geoip
}

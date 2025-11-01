// internal/server/audit.server.go
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
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	emailclient "x/shared/email"
	pb "x/shared/genproto/authentication/audit-service/grpcpb"
	notificationclient "x/shared/notification"
	smsclient "x/shared/sms"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	grpcServer "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	config          config.AppConfig
	db              interface{ Close() }
	grpcServer      *grpcServer.Server
	httpServer      *http.Server
	kafkaProducer   *kafka.SecurityAuditProducer
	kafkaConsumer   *kafka.SecurityAuditConsumer
	workers         *workers.SecurityAuditWorkers
	cancelFuncs     []context.CancelFunc
	wg              sync.WaitGroup
}

func NewServer(cfg config.AppConfig) error {
	srv := &Server{
		config:      cfg,
		cancelFuncs: make([]context.CancelFunc, 0),
	}

	if err := srv.initialize(); err != nil {
		return fmt.Errorf("server initialization failed: %w", err)
	}

	return srv.run()
}

func (s *Server) initialize() error {
	log.Println("🚀 Initializing Audit Service...")

	// 1. Connect to database
	db, err := config.ConnectDB()
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	s.db = db

	// 2. Initialize repository
	userRepo := repository.NewUserRepository(db)

	// 3. Initialize external clients
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	notificationCli := notificationclient.NewNotificationService()

	// 4. Initialize GeoIP service (optional)
	var geoIPService service.GeoIPService
	if s.config.EnableGeoIP {
		geoIPService, err = s.initGeoIPService()
		if err != nil {
			log.Printf("⚠️  GeoIP service initialization failed: %v. Continuing without GeoIP.", err)
		} else {
			log.Println("✅ GeoIP service initialized")
		}
	}

	// 5. Initialize audit service
	auditService := service.NewSecurityAuditServiceWithConfig(
		userRepo,
		geoIPService,
		s.config.MaxFailedLogins,
		s.config.LockoutDuration,
	)

	// 6. Setup gRPC server
	if err := s.setupGRPCServer(auditService, emailCli, smsCli, notificationCli); err != nil {
		return fmt.Errorf("gRPC setup failed: %w", err)
	}

	// 7. Setup Kafka (optional)
	if s.config.EnableKafka {
		if err := s.setupKafka(auditService); err != nil {
			log.Printf("⚠️  Kafka setup failed: %v. Continuing without Kafka.", err)
		} else {
			log.Println("✅ Kafka initialized")
		}
	}

	// 8. Setup WebSocket (optional)
	var wsHub *websocket.Hub
	var notifier *websocket.SecurityAuditNotifier
	
	if s.config.EnableWebSocket {
		wsHub = websocket.NewHub()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			wsHub.Run()
		}()
		notifier = websocket.NewSecurityAuditNotifier(wsHub, auditService)
		log.Println("✅ WebSocket hub initialized")
	}

	// 9. Setup background workers (optional)
	if s.config.EnableWorkers {
		s.workers = workers.NewSecurityAuditWorkersWithConfig(
			auditService,
			notifier,
			s.config.MaintenanceInterval,
			s.config.SuspiciousCheckInterval,
			s.config.CriticalEventInterval,
		)
		s.workers.Start()
		log.Println("✅ Background workers started")
	}

	// 10. Setup HTTP server
	s.setupHTTPServer(wsHub)

	log.Println("✨ Server initialization complete!")
	return nil
}

func (s *Server) setupGRPCServer(
	auditService *service.SecurityAuditService,
	emailCli *emailclient.EmailClient,
	smsCli *smsclient.SMSClient,
	notificationCli *notificationclient.NotificationService,
) error {
	// Create gRPC server with options
	opts := []grpcServer.ServerOption{
		grpcServer.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpcServer.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
	}

	s.grpcServer = grpcServer.NewServer(opts...)

	// Register audit service
	securityHandler := handler.NewSecurityAuditGRPCServer(
		auditService,
		emailCli,
		smsCli,
		notificationCli,
	)
	pb.RegisterSecurityAuditServiceServer(s.grpcServer, securityHandler)

	// Register health check service
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(s.grpcServer, healthServer)
	healthServer.SetServingStatus("audit.SecurityAuditService", healthpb.HealthCheckResponse_SERVING)

	// Enable reflection for grpcurl
	if s.config.Environment == "development" {
		reflection.Register(s.grpcServer)
	}

	// Start gRPC server in goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		
		listener, err := net.Listen("tcp", s.config.GRPCAddress)
		if err != nil {
			log.Fatalf("❌ Failed to create gRPC listener: %v", err)
		}

		log.Printf("🔊 gRPC server listening on %s", s.config.GRPCAddress)
		if err := s.grpcServer.Serve(listener); err != nil {
			log.Printf("❌ gRPC server error: %v", err)
		}
	}()

	return nil
}

func (s *Server) setupKafka(auditService *service.SecurityAuditService) error {
	// Create Kafka producer
	producer, err := kafka.NewSecurityAuditProducer(s.config.KafkaBrokers)
	if err != nil {
		return fmt.Errorf("kafka producer creation failed: %w", err)
	}
	s.kafkaProducer = producer

	// Create Kafka consumer
	consumer, err := kafka.NewSecurityAuditConsumer(
		s.config.KafkaBrokers,
		"security-audit-consumer-group",
		auditService,
	)
	if err != nil {
		s.kafkaProducer.Close()
		return fmt.Errorf("kafka consumer creation failed: %w", err)
	}
	s.kafkaConsumer = consumer

	// Start Kafka consumer
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFuncs = append(s.cancelFuncs, cancel)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := consumer.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("⚠️  Kafka consumer error: %v", err)
		}
	}()

	return nil
}

func (s *Server) setupHTTPServer(wsHub *websocket.Hub) {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health endpoints
	r.Get("/health", s.healthCheckHandler)
	r.Get("/health/ready", s.readinessHandler)
	r.Get("/health/live", s.livenessHandler)

	// Metrics endpoint (placeholder)
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "# Metrics endpoint - implement Prometheus metrics here\n")
	})

	// WebSocket endpoint
	if s.config.EnableWebSocket && wsHub != nil {
		r.Get("/ws/security-audit", func(w http.ResponseWriter, req *http.Request) {
			websocket.ServeWebSocket(wsHub, w, req)
		})
		log.Println("✅ WebSocket endpoint enabled")
	}

	s.httpServer = &http.Server{
		Addr:         s.config.HTTPAddress,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		
		log.Printf("🌐 HTTP server listening on %s", s.config.HTTPAddress)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ HTTP server error: %v", err)
		}
	}()
}

func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","service":"audit-service"}`)
}

func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	// Check if service is ready (e.g., database connectivity)
	if pool, ok := s.db.(*pgxpool.Pool); ok {
		if err := config.HealthCheck(pool); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unavailable","reason":"database"}`)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ready"}`)
}

func (s *Server) livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"alive"}`)
}

func (s *Server) run() error {
	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	log.Println("✅ Server is running. Press Ctrl+C to shutdown...")
	<-quit

	log.Println("🛑 Shutting down servers gracefully...")
	return s.shutdown()
}

func (s *Server) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var shutdownErrors []error

	// 1. Stop accepting new HTTP requests
	if s.httpServer != nil {
		log.Println("Shutting down HTTP server...")
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("HTTP shutdown error: %w", err))
		}
	}

	// 2. Stop gRPC server
	if s.grpcServer != nil {
		log.Println("Shutting down gRPC server...")
		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			log.Println("⚠️  gRPC graceful stop timeout, forcing stop...")
			s.grpcServer.Stop()
		}
	}

	// 3. Stop background workers
	if s.workers != nil {
		log.Println("Stopping background workers...")
		s.workers.Stop()
	}

	// 4. Cancel all contexts
	for _, cancelFunc := range s.cancelFuncs {
		cancelFunc()
	}

	// 5. Close Kafka connections
	if s.kafkaConsumer != nil {
		log.Println("Closing Kafka consumer...")
		if err := s.kafkaConsumer.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("kafka consumer close error: %w", err))
		}
	}

	if s.kafkaProducer != nil {
		log.Println("Closing Kafka producer...")
		if err := s.kafkaProducer.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("kafka producer close error: %w", err))
		}
	}

	// 6. Wait for all goroutines
	log.Println("Waiting for goroutines to finish...")
	waitDone := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(10 * time.Second):
		log.Println("⚠️  Some goroutines didn't finish in time")
	}

	// 7. Close database connection
	if s.db != nil {
		log.Println("Closing database connection...")
		s.db.Close()
	}

	if len(shutdownErrors) > 0 {
		log.Printf("⚠️  Shutdown completed with %d error(s)", len(shutdownErrors))
		for _, err := range shutdownErrors {
			log.Printf("  - %v", err)
		}
		return fmt.Errorf("shutdown completed with errors")
	}

	log.Println("✅ Server stopped gracefully")
	return nil
}

func (s *Server) initGeoIPService() (service.GeoIPService, error) {
	return service.NewGeoIPService(s.config.GeoIPDBPath)
}
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kyc-service/internal/config"
	"kyc-service/internal/handler"
	"kyc-service/internal/repository"
	"kyc-service/internal/service"
	"x/shared/auth/middleware"
	emailclient "x/shared/email"
	notificationclient "x/shared/notification" // ✅ added

	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()

	// db connection
	dbpool, err := pgxpool.New(context.Background(), cfg.DBConnString)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer dbpool.Close()

	// redis
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr, Password: cfg.RedisPass,
	})
	defer rdb.Close()

	// snowflake
	sf, err := id.NewSnowflake(10)
	if err != nil {
		log.Fatalf("sf: %v", err)
	}
	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client

	// repos & service
	emailCli := emailclient.NewEmailClient()
	kycRepo := repository.NewKYCRepo(dbpool)
	kycSvc := service.NewKYCService(kycRepo, sf)
	kycHandler := handler.NewKYCHandler(kycSvc, emailCli, notificationCli)

	// chi router
	r := chi.NewRouter()
	// r.Use(middleware.Logger)
	// r.Use(middleware.Recoverer)
	

	uploadDir := "/app/uploads"

	// Ensure directory exists
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	

	auth := middleware.RequireAuth()
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // allow all origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false, // must be false when using "*"
		MaxAge:           300,
	}))

	// Public route - serve uploaded files
	r.Handle("/api/v1/kyc/uploads/*", http.StripPrefix("/api/v1/kyc/uploads/", http.FileServer(http.Dir(uploadDir))))

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.Require([]string{"main"}, []string{"general", "kyc_review"}, nil))
		r.Use(auth.RateLimit(rdb, 15, 5*time.Minute, 5*time.Minute, "kyc"))

		r.Post("/api/v1/kyc/upload", kycHandler.UploadKYC)
		r.Get("/api/v1/kyc/status", kycHandler.GetKYCStatus)
		r.Get("/api/v1/kyc/submission/get", kycHandler.GetKYCSubmission)
		r.Post("/api/v1/kyc/review/{kycID}", kycHandler.ReviewKYC)
		r.Get("/api/v1/kyc/audit/{kycID}", kycHandler.GetKYCAuditLogs)
	})


	

	// HTTP server
	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}

	// run server in goroutine
	go func() {
		log.Printf("KYC REST server listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}
}

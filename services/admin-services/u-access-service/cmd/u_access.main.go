package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"admin-rbac-service/internal/config"
	"admin-rbac-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env (optional)
	if err := godotenv.Load(); err != nil {
		log.Println("RBAC: No .env file found, relying on system env vars")
	}

	// Load config
	cfg := config.Load()

	// Start server
	srv := server.NewServer(cfg)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("🛡️  RBAC service HTTP server starting on %s", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Println("🛑 RBAC service shutting down gracefully...")
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("RBAC service shutdown error: %v", err)
		}
	case err := <-errCh:
		log.Fatalf("RBAC service failed: %v", err)
	}
}

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"accounting-service/internal/config"
	"accounting-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env (optional)
	if err := godotenv.Load(); err != nil {
		log.Println("Accounting: No .env file found, relying on system env vars")
	}

	// Load config
	cfg := config.Load()

	// Start Accounting gRPC server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("🌍 Accounting gRPC server starting on %s", cfg.GRPCAddr)
		// This blocks until server exits
		server.NewAccountingGRPCServer(cfg)
		errCh <- nil
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("🛑 Accounting service shutting down gracefully...")
		// If you have any cleanup logic (e.g., close DB, Redis) add it here
		_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Add any shutdown logic if needed
	case err := <-errCh:
		if err != nil {
			log.Fatalf("Accounting service failed: %v", err)
		}
	}
}

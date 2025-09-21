package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"receipt-service/internal/config"
	"receipt-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env (optional)
	if err := godotenv.Load(); err != nil {
		log.Println("Receipt: No .env file found, relying on system env vars")
	}

	// Load config
	cfg := config.Load()

	// Start Receipt gRPC server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("🌍 Receipt gRPC server starting on %s", cfg.GRPCAddr)
		// This blocks until server exits
		server.NewReceiptGRPCServer(cfg)
		errCh <- nil
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("🛑 Receipt service shutting down gracefully...")
		// Timeout context for shutdown
		_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Add any cleanup logic here if needed (DB close, Redis close, etc.)
	case err := <-errCh:
		if err != nil {
			log.Fatalf("Receipt service failed: %v", err)
		}
	}
}

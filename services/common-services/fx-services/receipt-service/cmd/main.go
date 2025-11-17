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
	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)
		log.Printf("🌍 Receipt gRPC server starting on %s", cfg.GRPCAddr)
		if err := server.NewReceiptGRPCServer(cfg); err != nil {
			errCh <- err
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("🛑 Receipt service shutting down gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Cleanup logic handled in server
		<-ctx.Done()

	case err := <-errCh:
		log.Fatalf("❌ Receipt service failed: %v", err)

	case <-serverDone:
		log.Println("✅ Receipt service stopped")
	}
}
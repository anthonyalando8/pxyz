package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"partner-service/internal/config"
	"partner-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env
	if err := godotenv.Load(); err != nil {
		log.Println("Partner: No .env file found, relying on system env vars")
	}

	// Load app config
	cfg := config.Load()

	// Initialize servers (HTTP + gRPC)
	srv := server.NewServer(cfg)

	// Channel to capture errors
	errCh := make(chan error, 2)

	// Run HTTP server in background
	go func() {
		log.Printf("🌍 Partner HTTP server starting on %s", cfg.HTTPAddr)
		if err := srv.StartHTTP(); err != nil {
			errCh <- err
		}
	}()

	// Run gRPC server in background
	go func() {
		log.Printf("🔗 Partner gRPC server starting on %s", cfg.GRPCAddr)
		if err := srv.StartGRPC(cfg.GRPCAddr); err != nil {
			errCh <- err
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Shutting down partner servers...")
		// Gracefully stop HTTP
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.HTTP.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown HTTP server: %v", err)
		}
		// Gracefully stop gRPC
		srv.GRPC.GracefulStop()

	case err := <-errCh:
		log.Fatalf("Server error: %v", err)
	}
}

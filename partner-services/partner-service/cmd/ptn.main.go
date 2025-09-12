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

	// Initialize server
	srv := server.NewServer(cfg)

	// Run server in background
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Partner server starting on %s", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Println("Shutting down partner server...")
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Failed to shutdown server: %v", err)
		}
	case err := <-errCh:
		log.Fatal(err)
	}
}

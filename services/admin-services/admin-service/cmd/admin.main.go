package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"admin-service/internal/config"
	"admin-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	// Load env vars
	if err := godotenv.Load(); err != nil {
		log.Println("Admin: No .env file found, relying on system env vars")
	}

	// Load config
	cfg := config.Load()

	// Init server
	srv := server.NewServer(cfg)

	// Run server
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Admin service starting on %s", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Println("Shutting down admin service...")
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Admin shutdown failed: %v", err)
		}
	case err := <-errCh:
		log.Fatal(err)
	}
}

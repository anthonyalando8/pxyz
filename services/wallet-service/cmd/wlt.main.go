package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wallet-service/internal/config"
	"wallet-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Wallet: No .env file found, relying on system env vars")
	}

	cfg := config.Load()
	srv := server.New(cfg)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Wallet service starting on %s", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Shutting down gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	case err := <-errCh:
		log.Fatal(err)
	}
}

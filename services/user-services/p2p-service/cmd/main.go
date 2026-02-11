package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"p2p-service/internal/config"
	"p2p-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Cashier: No .env file found, relying on system env vars")
	}
	cfg := config.Load()
	server := server.NewServer(cfg)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Server starting on %s", cfg.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Println("Shutting down...")
		server.Shutdown(ctx)
	case err := <-errCh:
		log.Fatal(err)
	}
}

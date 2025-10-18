package main

import (
	"log"

	"audit-service/internal/config"
	"audit-service/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Audit: No .env file found, relying on system env vars")
	}

	cfg := config.Load()
	server.NewServer(cfg) // handles lifecycle & shutdown internally
}

package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)
type AppConfig struct {
	GRPCAddr string
}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("Session: No .env file found, relying on system env vars")
	}
	return AppConfig{
		GRPCAddr: os.Getenv("GRPC_ADDR"),
	}
}

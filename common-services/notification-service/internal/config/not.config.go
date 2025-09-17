package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	GRPCPort string
	HTTPPort string
}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("Auth: No .env file found, relying on system env vars")
	}
	return AppConfig{
		GRPCPort: getEnv("GRPCPort", ":8014"),
		HTTPPort: getEnv("HTTPPort", ":8013"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

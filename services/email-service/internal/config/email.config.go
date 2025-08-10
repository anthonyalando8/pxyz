package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"

)



type AppConfig struct {
	GRPCPort  string
	SMTPHost string
	SMTPUser string
	SMTPPort string
	SMTPPass string
}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("Auth: No .env file found, relying on system env vars")
	}
	return AppConfig{
		GRPCPort: getEnv("GRPCPort", ":50054"),
		SMTPHost: getEnv("SMTPHost", ":50054"),
		SMTPUser: getEnv("SMTPUser", ":50054"),
		SMTPPort: getEnv("SMTPPort", ":50054"),
		SMTPPass: getEnv("SMTPPass", ":50054"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

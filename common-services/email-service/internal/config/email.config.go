package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	GRPCPort string
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
		GRPCPort: getEnv("GRPCPort", ":8011"),
		SMTPHost: getEnv("SMTPHost", ":8011"),
		SMTPUser: getEnv("SMTPUser", ":8011"),
		SMTPPort: getEnv("SMTPPort", ":8011"),
		SMTPPass: getEnv("SMTPPass", ":8011"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

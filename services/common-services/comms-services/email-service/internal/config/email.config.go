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
	FromName   string
	ReplyTo    string
	DomainName string

}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("Auth: No .env file found, relying on system env vars")
	}
	return AppConfig{
		GRPCPort: getEnv("GRPCPort", ":8011"),
		SMTPHost: getEnv("SMTPHost", ""),
		SMTPUser: getEnv("SMTPUser", ""),
		SMTPPort: getEnv("SMTPPort", ""),
		SMTPPass: getEnv("SMTPPass", ""),
		FromName:   getEnv("EMAIL_FROM_NAME", "Derinance Support"),
		ReplyTo:    getEnv("EMAIL_REPLY_TO", "support@derinance.com"),
		DomainName: getEnv("EMAIL_DOMAIN", "derinance.com"),

	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

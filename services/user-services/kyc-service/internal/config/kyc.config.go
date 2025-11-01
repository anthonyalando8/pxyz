package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr     string
	DBConnString string
	RedisAddr    string
	RedisPass    string
	EmailSvcAddr string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("OTP: No .env file found, relying on system env vars")
	}

	return Config{
		HTTPAddr:     getEnv("HTTP_ADDR", ":8005"),
		DBConnString: getEnv("DB_CONN", "postgres://sam:password@host.docker.internal:5432/pxyz"),
		RedisAddr:    getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:    getEnv("REDIS_PASS", ""),
		EmailSvcAddr: getEnv("EMAIL_SVC_ADDR", "email-service:8011"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoiOrDefault(s string, def int) int {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil || i <= 0 {
		return def
	}
	return i
}

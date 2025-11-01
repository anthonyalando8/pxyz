package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	GRPCAddr         string
	DBConnString     string
	RedisAddr        string
	RedisPass        string
	OTP_TTL          time.Duration
	OTP_Window       time.Duration
	OTP_MaxPerWindow int
	OTP_Cooldown     time.Duration
	EmailSvcAddr     string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("OTP: No .env file found, relying on system env vars")
	}
	ttl, _ := time.ParseDuration(getEnv("OTP_TTL", "15m"))
	window, _ := time.ParseDuration(getEnv("OTP_WINDOW", "10m"))
	cool, _ := time.ParseDuration(getEnv("OTP_COOLDOWN", "45s"))

	return Config{
		GRPCAddr:         getEnv("GRPC_ADDR", ":8003"),
		DBConnString:     getEnv("DB_CONN", "postgres://sam:password@host.docker.internal:5432/pxyz_user"),
		RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:        getEnv("REDIS_PASS", ""),
		OTP_TTL:          ttl,
		OTP_Window:       window,
		OTP_MaxPerWindow: atoiOrDefault(getEnv("OTP_MAX_PER_WINDOW", "5"), 5),
		OTP_Cooldown:     cool,
		EmailSvcAddr:     getEnv("EMAIL_SVC_ADDR", "email-service:8011"),
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

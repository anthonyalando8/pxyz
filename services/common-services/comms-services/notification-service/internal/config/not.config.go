package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	GRPCAddr  string
	HTTPAddr  string
	RedisAddr string
	RedisPass string
}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("Auth: No .env file found, relying on system env vars")
	}
	return AppConfig{
		GRPCAddr:  getEnv("GRPC_ADDR", ":8014"),
		HTTPAddr:  getEnv("HTTP_ADDR", ":8013"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"

)

type Config struct {
	GRPCAddr     string
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
		GRPCAddr:     getEnv("GRPC_ADDR", ":50056"),
		DBConnString: getEnv("DB_CONN", "postgres://postgres:password@host.docker.internal:5432/pxyz"),
		RedisAddr:    getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:    getEnv("REDIS_PASS", ""),
		EmailSvcAddr: getEnv("EMAIL_SVC_ADDR", "email-service:50054"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

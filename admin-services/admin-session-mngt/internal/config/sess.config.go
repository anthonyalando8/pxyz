package config

import (
	"log"
	"os"
	"time"

	"admin-session-service/pkg/jwtutil"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	GRPCAddr  string
	JWT       jwtutil.JWTConfig
	RedisPass string
	RedisAddr string
}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("Session: No .env file found, relying on system env vars")
	}

	ttl, _ := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "5h"))

	return AppConfig{
		GRPCAddr:  getEnv("GRPC_ADDR", ":7003"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
		JWT: jwtutil.JWTConfig{
			PrivPath: getEnv("JWT_PRIVATE_KEY_PATH", "/app/secrets/jwt_private.pem"),
			PubPath:  getEnv("JWT_PUBLIC_KEY_PATH", "/app/secrets/jwt_public.pem"),
			Issuer:   getEnv("JWT_ISSUER", "admin-auth-service"),
			Audience: getEnv("JWT_AUDIENCE", "pxyz-admin-clients"),
			TTL:      ttl,
			KID:      "kid-v1",
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

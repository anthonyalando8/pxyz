package config

import (
	"os"
	"time"
	"auth-service/pkg/jwtutil"
)



type AppConfig struct {
	HTTPAddr  string
	JWT       jwtutil.JWTConfig
	RedisPass string
	RedisAddr string
}

func Load() AppConfig {
	ttl, _ := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))

	return AppConfig{
		HTTPAddr: getEnv("HTTP_ADDR", ":50051"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
		JWT: jwtutil.JWTConfig{
			PrivPath: getEnv("JWT_PRIVATE_KEY_PATH", "./secrets/jwt_private.pem"),
			PubPath:  getEnv("JWT_PUBLIC_KEY_PATH", "./secrets/jwt_public.pem"),
			Issuer:   getEnv("JWT_ISSUER", "auth-service"),
			Audience: getEnv("JWT_AUDIENCE", "pxyz-clients"),
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

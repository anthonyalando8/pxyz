package config

import (
	"os"
)

type AppConfig struct {
	HTTPAddr  string
	GRPCAddr  string
	RedisPass string
	RedisAddr string
}

func Load() AppConfig {
	return AppConfig{
		HTTPAddr:  getEnv("HTTP_ADDR", ":7504"),
		GRPCAddr:  getEnv("GRPC_ADDR", ":7505"),
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

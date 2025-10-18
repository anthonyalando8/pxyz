package config

import (
	"os"
	"strings"
)

type AppConfig struct {
	GRPCAddress  string
	HTTPAddress  string
	KafkaBrokers []string
	RedisPass    string
	RedisAddr    string
}

func Load() AppConfig {
	return AppConfig{
		GRPCAddress:  getEnv("GRPC_ADDRESS", ":50051"),
		HTTPAddress:  getEnv("HTTP_ADDRESS", ":8080"),
		KafkaBrokers: getEnvSlice("KAFKA_BROKERS", []string{"kafka:9092"}),
		RedisAddr:    getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:    getEnv("REDIS_PASS", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

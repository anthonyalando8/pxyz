package config

import (
	"os"
	"strings"
)

type AppConfig struct {
	HTTPAddr  string
	GRPCAddr  string
	RedisPass string
	RedisAddr string
	KafkaBrokers     []string

}

func Load() AppConfig {
	return AppConfig{
		HTTPAddr:  getEnv("HTTP_ADDR", ":8023"),
		GRPCAddr:  getEnv("GRPC_ADDR", ":8024"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
		KafkaBrokers:   getEnvSlice("KAFKA_BROKERS", []string{"kafka:9092"}),

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

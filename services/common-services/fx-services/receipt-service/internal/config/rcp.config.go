package config

import (
	"os"
	"strings"
)

type AppConfig struct {
	HTTPAddr     string
	GRPCAddr     string
	RedisPass    string
	RedisAddr    string
	KafkaBrokers []string // list of broker addresses, e.g. ["localhost:9092","localhost:9093"]
	KafkaTopic   string
}

func Load() AppConfig {
	return AppConfig{
		HTTPAddr:     getEnv("HTTP_ADDR", ":8023"),
		GRPCAddr:     getEnv("GRPC_ADDR", ":8024"),
		RedisAddr:    getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:    getEnv("REDIS_PASS", ""),
		KafkaBrokers: parseCSVEnv("KAFKA_BROKERS", "kafka:9092"),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "receipts"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseCSVEnv(key, fallback string) []string {
	val := getEnv(key, fallback)
	parts := strings.Split(val, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

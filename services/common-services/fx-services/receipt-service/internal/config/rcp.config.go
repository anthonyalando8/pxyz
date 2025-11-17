package config

import (
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	HTTPAddr     string
	GRPCAddr     string
	RedisAddr    string
	RedisPass    string
	RedisDB      int
	KafkaBrokers []string
	KafkaTopic   string
	MachineID    int64 // For Snowflake ID generation
}

func Load() AppConfig {
	return AppConfig{
		HTTPAddr:     getEnv("HTTP_ADDR", ":8025"),
		GRPCAddr:     getEnv("GRPC_ADDR", ":8026"),
		RedisAddr:    getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:    getEnv("REDIS_PASS", ""),
		RedisDB:      getEnvInt("REDIS_DB", 0),
		KafkaBrokers: parseCSVEnv("KAFKA_BROKERS", "kafka:9092"),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "receipts"),
		MachineID:    getEnvInt64("MACHINE_ID", 19), // IMPORTANT: Unique per instance
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
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
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

	// External Services
	AuthServiceAddr         string
	PartnerServiceAddr      string
	ReceiptServiceAddr      string
	NotificationServiceAddr string
	
	// Feature flags
	SeedOnStartup bool // Enable system seeding on startup
	SeedAgents    bool // Enable agent account seeding
	
	// Environment
	Environment string // "development", "staging", "production"

}

func Load() AppConfig {
	return AppConfig{
		HTTPAddr:  getEnv("HTTP_ADDR", ":8023"),
		GRPCAddr:  getEnv("GRPC_ADDR", ":8024"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
		KafkaBrokers:   getEnvSlice("KAFKA_BROKERS", []string{"kafka:9092"}),

		AuthServiceAddr:         getEnv("AUTH_SERVICE_ADDR", "localhost:50052"),
		PartnerServiceAddr:      getEnv("PARTNER_SERVICE_ADDR", "localhost:50053"),
		ReceiptServiceAddr:      getEnv("RECEIPT_SERVICE_ADDR", "localhost:50054"),
		NotificationServiceAddr: getEnv("NOTIFICATION_SERVICE_ADDR", "localhost:50055"),
		
		// Feature flags
		SeedOnStartup: getEnvBool("SEED_ON_STARTUP", true), // Default: enabled
		SeedAgents:    getEnvBool("SEED_AGENTS", false),     // Default: disabled
		
		// Environment
		Environment: getEnv("ENVIRONMENT", "development"),

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


func getEnvBool(key string, defaultValue bool) bool {
	// Implementation to get bool from env var
	return defaultValue
}
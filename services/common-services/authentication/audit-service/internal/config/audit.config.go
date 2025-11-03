// internal/config/audit.config.go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type AppConfig struct {
	// Server Configuration
	GRPCAddress string
	HTTPAddress string
	Environment string

	// Kafka Configuration
	KafkaBrokers      []string
	KafkaRetryMax     int
	KafkaRetryBackoff time.Duration

	// Redis Configuration
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Security Configuration
	MaxFailedLogins    int
	LockoutDuration    time.Duration
	RateLimitWindow    time.Duration
	RateLimitThreshold int

	// Worker Configuration
	MaintenanceInterval      time.Duration
	SuspiciousCheckInterval  time.Duration
	CriticalEventInterval    time.Duration

	// GeoIP Configuration
	GeoIPDBPath string

	// Feature Flags
	EnableWebSocket     bool
	EnableKafka         bool
	EnableGeoIP         bool
	EnableWorkers       bool
}

func Load() AppConfig {
	return AppConfig{
		// Server
		GRPCAddress: getEnv("GRPC_ADDRESS", ":8007"),
		HTTPAddress: getEnv("HTTP_ADDRESS", ":8008"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// Kafka
		KafkaBrokers:      getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaRetryMax:     getEnvInt("KAFKA_RETRY_MAX", 5),
		KafkaRetryBackoff: getEnvDuration("KAFKA_RETRY_BACKOFF", 2*time.Second),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		// Security
		MaxFailedLogins:    getEnvInt("MAX_FAILED_LOGINS", 5),
		LockoutDuration:    getEnvDuration("LOCKOUT_DURATION", 30*time.Minute),
		RateLimitWindow:    getEnvDuration("RATE_LIMIT_WINDOW", 15*time.Minute),
		RateLimitThreshold: getEnvInt("RATE_LIMIT_THRESHOLD", 10),

		// Workers
		MaintenanceInterval:     getEnvDuration("MAINTENANCE_INTERVAL", 1*time.Hour),
		SuspiciousCheckInterval: getEnvDuration("SUSPICIOUS_CHECK_INTERVAL", 5*time.Minute),
		CriticalEventInterval:   getEnvDuration("CRITICAL_EVENT_INTERVAL", 1*time.Minute),

		// GeoIP
		GeoIPDBPath: getEnv("GEOIP_DB_PATH", "/usr/local/share/GeoIP/GeoLite2-City.mmdb"),

		// Feature Flags
		EnableWebSocket: getEnvBool("ENABLE_WEBSOCKET", true),
		EnableKafka:     getEnvBool("ENABLE_KAFKA", true),
		EnableGeoIP:     getEnvBool("ENABLE_GEOIP", false),
		EnableWorkers:   getEnvBool("ENABLE_WORKERS", true),
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

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
package config

import (
	"os"
	"strings"
)

type AppleConfig struct {
	TeamID        string // Apple Developer Team ID
	KeyID         string // Key ID for your .p8 private key
	ServiceID     string // "Client ID" (Web: Service ID, iOS/macOS: Bundle ID)
	RedirectURI   string // Must match the one configured in Apple Console
	PrivateKeyPEM string // Contents of your AuthKey_XXXXXX.p8 (keep safe!)
}

type AppConfig struct {
	HTTPAddr         string
	GRPCAddr         string
	RedisPass        string
	RedisAddr        string
	GoogleClientID   string
	Apple            AppleConfig
	TelegramBotToken string
	TelegramChatID   string
	KafkaBrokers     []string
}

func Load() AppConfig {
	return AppConfig{
		HTTPAddr:       getEnv("HTTP_ADDR", ":8001"),
		GRPCAddr:       getEnv("GRPC_ADDR", ":8006"),
		RedisAddr:      getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:      getEnv("REDIS_PASS", ""),
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		KafkaBrokers:   getEnvSlice("KAFKA_BROKERS", []string{"kafka-service:9092"}),

		Apple: AppleConfig{
			TeamID:        getEnv("APPLE_TEAM_ID", ""),
			KeyID:         getEnv("APPLE_KEY_ID", ""),
			ServiceID:     getEnv("APPLE_SERVICE_ID", ""),
			RedirectURI:   getEnv("APPLE_REDIRECT_URI", ""),
			PrivateKeyPEM: getEnv("APPLE_PRIVATE_KEY_PEM", ""),
		},
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", "8310235335:AAG40N9VHTuAdGnRkFv4388k6GgMx1sR1Os"),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
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

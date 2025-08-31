package config

import (
	"os"
	"log"
	"github.com/joho/godotenv"
)


type AppConfig struct {
	GRPCAddr  string
	RedisPass string
	RedisAddr string
	SmsKey string
    WaKey string
    SmsURL string
    WaURL string
    Sender string
	UserId string
	Password string
}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("SMS: No .env file found, relying on system env vars")
	}

	return AppConfig{
		GRPCAddr: getEnv("GRPC_ADDR", ":50059"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
		SmsKey: getEnv("SMS_KEY", ""),
		WaKey: getEnv("WA_KEY", ""),
		SmsURL: getEnv("SMS_URL", "https://smsportal.hostpinnacle.co.ke/SMSApi/send"),
		WaURL: getEnv("WA_URL", "https://whatsappprovider.com/api"),
		Sender: getEnv("SENDER", "SENDER_ID"),
		UserId: getEnv("USER_ID", "your_user_id"),
		Password: getEnv("PASSWORD", "your_password"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

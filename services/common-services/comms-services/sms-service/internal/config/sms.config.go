package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	GRPCAddr  string
	RedisPass string
	RedisAddr string
	SmsKey    string
	WaKey     string
	WaSender  string
	SmsURL    string
	WaURL     string
	Sender    string
	UserId    string
	Password  string
}

func Load() AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("SMS: No .env file found, relying on system env vars")
	}

	return AppConfig{
		GRPCAddr:  getEnv("GRPC_ADDR", ":8012"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
		SmsKey:    getEnv("SMS_KEY", ""),
		WaKey:     getEnv("WA_KEY", ""),
		WaSender:  getEnv("WA_SENDER", "254792207010"),
		SmsURL:    getEnv("SMS_URL", "https://smsportal.hostpinnacle.co.ke/SMSApi/send"),
		WaURL:     getEnv("WA_URL", "https://www.whatsupsender.co.ke/api/qr/rest/send_message"),
		Sender:    getEnv("SENDER", "DERINANCE"),
		UserId:    getEnv("USER_ID", "samderinance"),
		Password:  getEnv("PASSWORD", "Fbq75Ttz"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

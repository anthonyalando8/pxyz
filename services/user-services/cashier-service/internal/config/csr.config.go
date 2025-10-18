package config

import (
	"os"
)

type AppConfig struct {
	HTTPAddr            string
	RedisPass           string
	RedisAddr           string
	MpesaShortCode      string
	MpesaPassKey        string
	MpesaConsumerKey    string
	MpesaConsumerSecret string
	MpesaBaseURL        string
	MpesaTillNumber     string
}

func Load() AppConfig {
	return AppConfig{
		HTTPAddr:            getEnv("HTTP_ADDR", ":8021"),
		RedisAddr:           getEnv("REDIS_ADDR", "redis:6379"),
		RedisPass:           getEnv("REDIS_PASS", ""),
		MpesaShortCode:      getEnv("MPESA_SHORT_CODE", ""),
		MpesaPassKey:        getEnv("MPESA_PASS_KEY", ""),
		MpesaConsumerKey:    getEnv("MPESA_CONSUMER_KEY", ""),
		MpesaConsumerSecret: getEnv("MPESA_CONSUMER_SECRET", ""),
		MpesaBaseURL:        getEnv("MPESA_BASE_URL", "https://sandbox.safaricom.co.ke"),
		MpesaTillNumber:     getEnv("MPESA_TILL_NUMBER", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

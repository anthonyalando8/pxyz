// config/config.go
package config

import (
    "os"
)

type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Mpesa    MpesaConfig
    Partner  PartnerConfig
    Redis    RedisConfig
}

type ServerConfig struct {
    Port string
    Env  string
}

type DatabaseConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    DBName   string
    SSLMode  string
}

type MpesaConfig struct {
    Environment    string // sandbox or production
    ConsumerKey    string
    ConsumerSecret string
    Passkey        string
    ShortCode      string
    InitiatorName  string
    SecurityCredential string

    B2CShortCode      string
    B2CInitiatorName  string
    B2CPassword       string
    B2CSecurityCredential string
    B2CPasskey        string
    B2CConsumerKey    string
    B2CConsumerSecret string
}

type PartnerConfig struct {
    APIKey      string
    APISecret   string
    WebhookURL  string // Partner's webhook URL to send notifications
    CallbackURL string // Our callback URL base
}

type RedisConfig struct {
    Host     string
    Port     string
    Password string
    DB       int
}

func Load() (*Config, error) {
    return &Config{
        Server: ServerConfig{
            Port: getEnv("SERVER_PORT", "8080"),
            Env:  getEnv("ENVIRONMENT", "development"),
        },
        Database:  DatabaseConfig{
            Host:     getEnv("DB_HOST", "localhost"),
            Port:     getEnv("DB_PORT", "5432"),
            User:     getEnv("DB_USER", "postgres"),
            Password:  getEnv("DB_PASSWORD", ""),
            DBName:   getEnv("DB_NAME", "pxyz_fx"),
            SSLMode:  getEnv("DB_SSL_MODE", "disable"),
        },
        Mpesa: MpesaConfig{
            Environment:    getEnv("MPESA_ENVIRONMENT", "sandbox"),
            ConsumerKey:    getEnv("MPESA_CONSUMER_KEY", ""),
            ConsumerSecret: getEnv("MPESA_CONSUMER_SECRET", ""),
            Passkey:        getEnv("MPESA_PASSKEY", ""),
            ShortCode:      getEnv("MPESA_SHORT_CODE", ""),
            InitiatorName:  getEnv("MPESA_INITIATOR_NAME", ""),
            SecurityCredential: getEnv("MPESA_SECURITY_CREDENTIAL", ""),
            B2CShortCode:      getEnv("B2C_SHORT_CODE", ""),
            B2CInitiatorName:  getEnv("B2C_INITIATOR_NAME", ""),
            B2CPassword:       getEnv("B2C_PASSWORD", ""),
            B2CSecurityCredential: getEnv("B2C_SECURITY_CREDENTIAL", ""),
            B2CPasskey:        getEnv("B2C_PASSKEY", ""),
            B2CConsumerKey:    getEnv("B2C_CONSUMER_KEY", ""),
            B2CConsumerSecret: getEnv("B2C_CONSUMER_SECRET", ""),
        },
        Partner: PartnerConfig{
            APIKey:      getEnv("PARTNER_API_KEY", ""),
            APISecret:   getEnv("PARTNER_API_SECRET", ""),
            WebhookURL:  getEnv("PARTNER_WEBHOOK_URL", ""),
            CallbackURL: getEnv("CALLBACK_BASE_URL", "http://localhost:8080"),
        },
        Redis: RedisConfig{
            Host:     getEnv("REDIS_HOST", "redis"),
            Port:     getEnv("REDIS_PORT", "6379"),
            Password:  getEnv("REDIS_PASSWORD", ""),
            DB:       0,
        },
    }, nil
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
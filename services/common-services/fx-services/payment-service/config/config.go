// config/config.go
package config

import (
    "fmt"
    "os"
    //"path/filepath"
    
    "payment-service/pkg/security"
    
    "go.uber.org/zap"
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
    Environment           string
    ConsumerKey           string
    ConsumerSecret        string
    Passkey               string
    ShortCode             string
    InitiatorName         string
    SecurityCredential    string
    
    B2CShortCode          string
    B2CInitiatorName      string
    B2CPassword           string
    B2CSecurityCredential string
    B2CPasskey            string
    B2CConsumerKey        string
    B2CConsumerSecret     string
}

type PartnerConfig struct {
    APIKey      string
    APISecret   string
    WebhookURL  string
    CallbackURL string
}

type RedisConfig struct {
    Host     string
    Port     string
    Password string
    DB       int
}

func Load(logger *zap.Logger) (*Config, error) {
    cfg := &Config{
        Server:  ServerConfig{
            Port: getEnv("SERVER_PORT", "8027"),
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
            
            B2CShortCode:       getEnv("B2C_SHORT_CODE", ""),
            B2CInitiatorName:   getEnv("B2C_INITIATOR_NAME", ""),
            B2CPassword:        getEnv("B2C_PASSWORD", ""),
            B2CPasskey:        getEnv("B2C_PASSKEY", ""),
            B2CConsumerKey:    getEnv("B2C_CONSUMER_KEY", ""),
            B2CConsumerSecret: getEnv("B2C_CONSUMER_SECRET", ""),
        },
        Partner: PartnerConfig{
            APIKey:      getEnv("PARTNER_API_KEY", ""),
            APISecret:   getEnv("PARTNER_API_SECRET", ""),
            WebhookURL:  getEnv("PARTNER_WEBHOOK_URL", ""),
            CallbackURL:  getEnv("CALLBACK_BASE_URL", "http://localhost:8027"),
        },
        Redis:  RedisConfig{
            Host:      getEnv("REDIS_HOST", "redis"),
            Port:     getEnv("REDIS_PORT", "6379"),
            Password: getEnv("REDIS_PASSWORD", ""),
            DB:       0,
        },
    }

    // Generate security credentials from certificate
    if err := cfg.generateSecurityCredentials(logger); err != nil {
        return nil, fmt.Errorf("failed to generate security credentials: %w", err)
    }

    return cfg, nil
}

// generateSecurityCredentials generates M-Pesa security credentials from certificate
func (c *Config) generateSecurityCredentials(logger *zap.Logger) error {
    // Determine certificate path
    certPath := getEnv("MPESA_CERT_PATH", "/app/certs/cert.cer")
    
    // Check if certificate exists
    if _, err := os. Stat(certPath); os.IsNotExist(err) {
        // Try alternative paths
        altPaths := []string{
            "./certs/cert.cer",
            "./payment-service/certs/cert.cer",
            "/app/certs/cert.cer",
        }
        
        found := false
        for _, path := range altPaths {
            if _, err := os.Stat(path); err == nil {
                certPath = path
                found = true
                break
            }
        }
        
        if !found {
            logger. Warn("M-Pesa certificate not found, using environment variables",
                zap.String("searched_path", certPath),
                zap. Strings("alternative_paths", altPaths))
            
            // Use credentials from environment if certificate not found
            c. Mpesa.SecurityCredential = getEnv("MPESA_SECURITY_CREDENTIAL", "")
            c.Mpesa.B2CSecurityCredential = getEnv("B2C_SECURITY_CREDENTIAL", "")
            return nil
        }
    }

    logger.Info("generating M-Pesa security credentials from certificate",
        zap.String("cert_path", certPath))

    // Generate STK Push security credential
    if c.Mpesa.InitiatorName != "" {
        initiatorPassword := getEnv("MPESA_INITIATOR_PASSWORD", "Safaricom999!*!")
        credential, err := security.GenerateSecurityCredential(certPath, initiatorPassword)
        if err != nil {
            logger.Error("failed to generate STK security credential",
                zap.Error(err))
            return fmt.Errorf("failed to generate STK security credential: %w", err)
        }
        c.Mpesa.SecurityCredential = credential
        logger.Info("STK Push security credential generated successfully")
    }

    // Generate B2C security credential
    if c.Mpesa.B2CInitiatorName != "" && c.Mpesa.B2CPassword != "" {
        credential, err := security.GenerateSecurityCredential(certPath, c.Mpesa.B2CPassword)
        if err != nil {
            logger.Error("failed to generate B2C security credential",
                zap.Error(err))
            return fmt.Errorf("failed to generate B2C security credential: %w", err)
        }
        c. Mpesa.B2CSecurityCredential = credential
        logger. Info("B2C security credential generated successfully",
            zap.String("initiator_name", c.Mpesa.B2CInitiatorName))
    }

    return nil
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
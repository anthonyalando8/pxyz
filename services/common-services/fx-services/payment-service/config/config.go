// config/config.go
package config

import (
	"fmt"
	//"log"
	"os"
	"strconv"
	"strings"

	"payment-service/pkg/security"

	"go.uber.org/zap"
)

type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Mpesa    MpesaConfig
    Partners map[string]PartnerConfig  //  Changed to map for multiple partners
    BaseCallbackURL string
    Redis    RedisConfig
    logger   *zap.Logger
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
    
    // B2C Configuration
    B2CShortCode          string
    B2CInitiatorName      string
    B2CPassword           string
    B2CSecurityCredential string
    B2CPasskey            string
    B2CConsumerKey        string
    B2CConsumerSecret     string
    
    // ✅ B2B Configuration (NEW)
    B2BShortCode          string
    B2BInitiatorName      string
    B2BPassword           string
    B2BSecurityCredential string
    B2BConsumerKey        string
    B2BConsumerSecret     string
}

// ✅ Updated PartnerConfig
type PartnerConfig struct {
    ID          string  // Partner identifier (e.g., "safaricom", "airtel", "bank-ke")
    Name        string  // Display name
    APIKey      string  // Partner API key
    APISecret   string  // Partner API secret
    WebhookURL  string  // Partner's webhook endpoint
    CallbackURL string  // Our callback URL for this partner
    Enabled     bool    // Is this partner active
}

type RedisConfig struct {
    Host     string
    Port     string
    Password string
    DB       int
}

// Update the Load function to include B2B config
func Load(logger *zap.Logger) (*Config, error) {
    cfg := &Config{
        Server:  ServerConfig{
            Port: getEnv("SERVER_PORT", "8027"),
            Env:  getEnv("ENVIRONMENT", "development"),
        },
        Database: DatabaseConfig{
            Host:     getEnv("DB_HOST", "localhost"),
            Port:     getEnv("DB_PORT", "5432"),
            User:     getEnv("DB_USER", "postgres"),
            Password: getEnv("DB_PASSWORD", ""),
            DBName:   getEnv("DB_NAME", "pxyz_fx"),
            SSLMode:  getEnv("DB_SSL_MODE", "disable"),
        },
        Mpesa: MpesaConfig{
            Environment:      getEnv("MPESA_ENVIRONMENT", "sandbox"),
            ConsumerKey:    getEnv("MPESA_CONSUMER_KEY", ""),
            ConsumerSecret: getEnv("MPESA_CONSUMER_SECRET", ""),
            Passkey:          getEnv("MPESA_PASSKEY", ""),
            ShortCode:      getEnv("MPESA_SHORT_CODE", ""),
            InitiatorName:  getEnv("MPESA_INITIATOR_NAME", ""),
            
            // B2C
            B2CShortCode:        getEnv("B2C_SHORT_CODE", ""),
            B2CInitiatorName:   getEnv("B2C_INITIATOR_NAME", ""),
            B2CPassword:        getEnv("B2C_PASSWORD", ""),
            B2CPasskey:         getEnv("B2C_PASSKEY", ""),
            B2CConsumerKey:     getEnv("B2C_CONSUMER_KEY", ""),
            B2CConsumerSecret:  getEnv("B2C_CONSUMER_SECRET", ""),
            
            // ✅ B2B (NEW)
            B2BShortCode:       getEnv("B2B_SHORTCODE", ""),
            B2BInitiatorName:   getEnv("B2B_INITIATOR_NAME", ""),
            B2BPassword:        getEnv("B2B_INITIATOR_PASSWORD", ""),
            B2BConsumerKey:     getEnv("B2B_CONSUMER_KEY", ""),
            B2BConsumerSecret:  getEnv("B2B_CONSUMER_SECRET", ""),
        },
        Redis: RedisConfig{
            Host:     getEnv("REDIS_HOST", "redis"),
            Port:     getEnv("REDIS_PORT", "6379"),
            Password: getEnv("REDIS_PASSWORD", ""),
            DB:       0,
        },
        BaseCallbackURL: getEnv("CALLBACK_BASE_URL", "https://api.safarigari.com"),
        logger:          logger,
    }

    // Load multiple partners
    if err := cfg. loadPartners(logger); err != nil {
        return nil, fmt. Errorf("failed to load partners: %w", err)
    }

    // Generate security credentials from certificate
    if err := cfg. generateSecurityCredentials(logger); err != nil {
        return nil, fmt.Errorf("failed to generate security credentials: %w", err)
    }

    return cfg, nil
}

// ✅ loadPartners loads multiple partner configurations from environment
func (c *Config) loadPartners(logger *zap.Logger) error {
    c.Partners = make(map[string]PartnerConfig)

    // Get list of partner IDs (comma-separated)
    partnerIDsStr := getEnv("PARTNER_IDS", "")
    if partnerIDsStr == "" {
        logger. Warn("no partners configured (PARTNER_IDS is empty)")
        return nil
    }

    partnerIDs := strings.Split(partnerIDsStr, ",")
    
    for _, partnerID := range partnerIDs {
        partnerID = strings.TrimSpace(partnerID)
        if partnerID == "" {
            continue
        }

        // Load partner config
        prefix := fmt.Sprintf("PARTNER_%s_", strings.ToUpper(partnerID))
        
        partner := PartnerConfig{
            ID:          partnerID,
            Name:        getEnv(prefix+"NAME", partnerID),
            APIKey:       getEnv(prefix+"API_KEY", ""),
            APISecret:   getEnv(prefix+"API_SECRET", ""),
            WebhookURL:  getEnv(prefix+"WEBHOOK_URL", ""),
            CallbackURL: getEnv(prefix+"CALLBACK_URL", getEnv("CALLBACK_BASE_URL", "http://localhost:8027")),
            Enabled:     getEnvBool(prefix+"ENABLED", true),
        }

        // Validate required fields
        if partner.APIKey == "" {
            logger. Warn("partner API key is missing",
                zap.String("partner_id", partnerID))
            continue
        }

        if partner.APISecret == "" {
            logger.Warn("partner API secret is missing",
                zap.String("partner_id", partnerID))
            continue
        }

        c.Partners[partnerID] = partner
        
        logger.Info("partner loaded",
            zap.String("partner_id", partnerID),
            zap.String("partner_name", partner.Name),
            zap.Bool("enabled", partner.Enabled))
    }

    if len(c.Partners) == 0 {
        return fmt.Errorf("no valid partners configured")
    }

    logger.Info("partners loaded successfully",
        zap. Int("count", len(c.Partners)))

    return nil
}

// ✅ GetPartner returns a specific partner configuration with fallback matching
func (c *Config) GetPartner(partnerID string) (*PartnerConfig, error) {
	// Try exact match first
	partner, exists := c.Partners[partnerID]
	if exists {
		if !partner.Enabled {
			return nil, fmt.Errorf("partner is disabled: %s", partnerID)
		}
		return &partner, nil
	}

	// ✅ Fallback 1: Try to find partner that starts with partnerID
	for id, p := range c.Partners {
		if strings.HasPrefix(strings.ToLower(id), strings.ToLower(partnerID)) && p.Enabled {
			c.logger.Info("using fallback partner match (prefix)",
				zap.String("requested", partnerID),
				zap.String("matched", id),
				zap.String("partner_name", p.Name))
			return &p, nil
		}
	}

	// ✅ Fallback 2: Try to find partner that contains partnerID
	for id, p := range c.Partners {
		if strings.Contains(strings.ToLower(id), strings.ToLower(partnerID)) && p.Enabled {
			c.logger.Info("using fallback partner match (contains)",
				zap.String("requested", partnerID),
				zap.String("matched", id),
				zap.String("partner_name", p. Name))
			return &p, nil
		}
	}

	// ✅ Fallback 3: Try to find partner by name (case-insensitive)
	for id, p := range c.Partners {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(partnerID)) && p.Enabled {
			c.logger.Info("using fallback partner match (name)",
				zap.String("requested", partnerID),
				zap.String("matched", id),
				zap.String("partner_name", p.Name))
			return &p, nil
		}
	}

	return nil, fmt.Errorf("partner not found: %s", partnerID)
}

// ✅ GetAllPartners returns all enabled partners
func (c *Config) GetAllPartners() []PartnerConfig {
    partners := make([]PartnerConfig, 0, len(c. Partners))
    for _, partner := range c.Partners {
        if partner.Enabled {
            partners = append(partners, partner)
        }
    }
    return partners
}

// generateSecurityCredentials generates M-Pesa security credentials from certificate
// Update generateSecurityCredentials to include B2B
func (c *Config) generateSecurityCredentials(logger *zap.Logger) error {
    // Determine certificate path
    certPath := getEnv("MPESA_CERT_PATH", "/app/certs/cert. cer")
    
    // Check if certificate exists
    if _, err := os.Stat(certPath); os.IsNotExist(err) {
        // Try alternative paths
        altPaths := []string{
            "./certs/cert.cer",
            "./payment-service/certs/cert.cer",
            "/app/certs/cert. cer",
        }
        
        found := false
        for _, path := range altPaths {
            if _, err := os. Stat(path); err == nil {
                certPath = path
                found = true
                break
            }
        }
        
        if !found {
            logger. Warn("M-Pesa certificate not found, using environment variables",
                zap.String("searched_path", certPath),
                zap.Strings("alternative_paths", altPaths))
            
            // Use credentials from environment if certificate not found
            c. Mpesa.SecurityCredential = getEnv("MPESA_SECURITY_CREDENTIAL", "")
            c.Mpesa.B2CSecurityCredential = getEnv("B2C_SECURITY_CREDENTIAL", "")
            c.Mpesa.B2BSecurityCredential = getEnv("B2B_SECURITY_CREDENTIAL", "")  // ✅ NEW
            return nil
        }
    }

    logger.Info("generating M-Pesa security credentials from certificate",
        zap.String("cert_path", certPath))

    // Generate STK Push security credential
    if c.Mpesa.InitiatorName != "" {
        initiatorPassword := getEnv("MPESA_INITIATOR_PASSWORD", "Safaricom999! *!")
        credential, err := security.GenerateSecurityCredential(certPath, initiatorPassword)
        if err != nil {
            logger.Error("failed to generate STK security credential",
                zap.Error(err))
            return fmt.Errorf("failed to generate STK security credential: %w", err)
        }
        c.Mpesa.SecurityCredential = credential
        logger. Info("STK Push security credential generated successfully")
    }

    // Generate B2C security credential
    if c. Mpesa.B2CInitiatorName != "" && c. Mpesa.B2CPassword != "" {
        credential, err := security.GenerateSecurityCredential(certPath, c. Mpesa.B2CPassword)
        if err != nil {
            logger.Error("failed to generate B2C security credential",
                zap.Error(err))
            return fmt.Errorf("failed to generate B2C security credential: %w", err)
        }
        c.Mpesa.B2CSecurityCredential = credential
        logger.Info("B2C security credential generated successfully",
            zap.String("initiator_name", c.Mpesa.B2CInitiatorName))
    }

    // ✅ Generate B2B security credential (NEW)
    if c.Mpesa.B2BInitiatorName != "" && c.Mpesa.B2BPassword != "" {
        credential, err := security. GenerateSecurityCredential(certPath, c.Mpesa. B2BPassword)
        if err != nil {
            logger. Error("failed to generate B2B security credential",
                zap. Error(err))
            return fmt.Errorf("failed to generate B2B security credential: %w", err)
        }
        c.Mpesa.B2BSecurityCredential = credential
        logger.Info("B2B security credential generated successfully",
            zap.String("initiator_name", c.Mpesa.B2BInitiatorName))
    }

    return nil
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
    if value := os.Getenv(key); value != "" {
        boolVal, err := strconv.ParseBool(value)
        if err == nil {
            return boolVal
        }
    }
    return defaultValue
}
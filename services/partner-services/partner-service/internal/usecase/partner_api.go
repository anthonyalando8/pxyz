// usecase/partner_api.go
package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"partner-service/internal/domain"
	"time"
)

// GenerateAPICredentials creates new API credentials for a partner
func (uc *PartnerUsecase) GenerateAPICredentials(ctx context.Context, partnerID string) (apiKey, apiSecret string, err error) {
	if partnerID == "" {
		return "", "", errors.New("partner_id is required")
	}

	// Generate API key (32 bytes, base64 encoded)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate API key: %w", err)
	}
	apiKey = "pk_" + base64.URLEncoding.EncodeToString(keyBytes)

	// Generate API secret (48 bytes, base64 encoded)
	secretBytes := make([]byte, 48)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate API secret: %w", err)
	}
	apiSecret = "sk_" + base64.URLEncoding.EncodeToString(secretBytes)

	// Hash the secret before storing
	hash := sha256.Sum256([]byte(apiSecret))
	apiSecretHash := hex.EncodeToString(hash[:])

	// Store in database
	if err := uc.partnerRepo.GenerateAPICredentials(ctx, partnerID, apiKey, apiSecretHash); err != nil {
		return "", "", fmt.Errorf("failed to store API credentials: %w", err)
	}

	return apiKey, apiSecret, nil
}

// RevokeAPICredentials removes API access for a partner
func (uc *PartnerUsecase) RevokeAPICredentials(ctx context.Context, partnerID string) error {
	if partnerID == "" {
		return errors.New("partner_id is required")
	}
	return uc.partnerRepo.RevokeAPICredentials(ctx, partnerID)
}

// RotateAPISecret generates a new secret while keeping the same API key
func (uc *PartnerUsecase) RotateAPISecret(ctx context.Context, partnerID string) (string, error) {
	if partnerID == "" {
		return "", errors.New("partner_id is required")
	}

	// Get current partner to retrieve API key
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, partnerID)
	if err != nil {
		return "", fmt.Errorf("failed to get partner: %w", err)
	}

	if partner.APIKey == nil {
		return "", errors.New("partner does not have API credentials")
	}

	// Generate new secret
	secretBytes := make([]byte, 48)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", fmt.Errorf("failed to generate API secret: %w", err)
	}
	apiSecret := "sk_" + base64.URLEncoding.EncodeToString(secretBytes)

	// Hash the secret
	hash := sha256.Sum256([]byte(apiSecret))
	apiSecretHash := hex.EncodeToString(hash[:])

	// Update in database
	if err := uc.partnerRepo.GenerateAPICredentials(ctx, partnerID, *partner.APIKey, apiSecretHash); err != nil {
		return "", fmt.Errorf("failed to rotate API secret: %w", err)
	}

	return apiSecret, nil
}

// UpdateAPISettings updates API configuration
func (uc *PartnerUsecase) UpdateAPISettings(ctx context.Context, partnerID string, isEnabled bool, rateLimit int, allowedIPs []string) error {
	if partnerID == "" {
		return errors.New("partner_id is required")
	}

	// Validate rate limit
	if rateLimit < 10 || rateLimit > 10000 {
		return errors.New("rate_limit must be between 10 and 10000")
	}

	// Update settings (implement in repo)
	return uc.partnerRepo.UpdateAPISettings(ctx, partnerID, isEnabled, rateLimit, allowedIPs)
}

// ValidateAPICredentials checks if API key and secret are valid
func (uc *PartnerUsecase) ValidateAPICredentials(ctx context.Context, apiKey, apiSecret string) (*domain.Partner, error) {
	if apiKey == "" || apiSecret == "" {
		return nil, errors.New("API key and secret are required")
	}

	// Get partner by API key
	partner, err := uc.partnerRepo.GetPartnerByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("invalid API credentials")
	}

	if !partner.IsAPIEnabled {
		return nil, errors.New("API access is disabled for this partner")
	}

	// Hash provided secret and compare
	hash := sha256.Sum256([]byte(apiSecret))
	providedHash := hex.EncodeToString(hash[:])

	if partner.APISecretHash == nil || *partner.APISecretHash != providedHash {
		return nil, errors.New("invalid API credentials")
	}

	return partner, nil
}

// usecase/partner_api.go - ADD THESE METHODS

// GetAPILogs retrieves API request logs for a partner
func (uc *PartnerUsecase) GetAPILogs(ctx context.Context, partnerID string, limit, offset int, endpointFilter *string) ([]domain.PartnerAPILog, int64, error) {
	if partnerID == "" {
		return nil, 0, errors.New("partner_id is required")
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	return uc.partnerRepo.GetAPILogs(ctx, partnerID, limit, offset, endpointFilter)
}

// GetAPIUsageStats returns API usage statistics for a partner
func (uc *PartnerUsecase) GetAPIUsageStats(ctx context.Context, partnerID string, from, to time.Time) (map[string]interface{}, error) {
	if partnerID == "" {
		return nil, errors.New("partner_id is required")
	}

	if from.IsZero() || to.IsZero() {
		return nil, errors.New("from and to dates are required")
	}

	if from.After(to) {
		return nil, errors.New("from date must be before to date")
	}

	return uc.partnerRepo.GetAPIUsageStats(ctx, partnerID, from, to)
}
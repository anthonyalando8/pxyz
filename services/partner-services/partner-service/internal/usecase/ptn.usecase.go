// usecase/partner_usecase.go - UPDATED CreatePartner

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
	"x/shared/utils/id"
)

// ---------------- Partner ----------------

// CreatePartner business logic with automatic API credential generation
func (uc *PartnerUsecase) CreatePartner(ctx context.Context, p *domain.Partner) error {
	if p.Name == "" {
		return errors.New("partner name cannot be empty")
	}

	// Generate external ID using Snowflake
	if p.ID == "" {
		p.ID = id.GenerateID("PTN")
	}

	// Set default API settings
	p.IsAPIEnabled = false // Disabled by default, admin can enable later
	p.APIRateLimit = 1000  // Default rate limit

	// Auto-generate API credentials during partner creation
	apiKey, apiSecret, err := generateAPICredentials()
	if err != nil {
		return fmt.Errorf("failed to generate API credentials: %w", err)
	}

	// Hash the secret before storing
	hash := sha256.Sum256([]byte(apiSecret))
	apiSecretHash := hex.EncodeToString(hash[:])

	// Store credentials in partner object
	p.APIKey = &apiKey
	p.APISecretHash = &apiSecretHash

	// Create partner with API credentials
	if err := uc.partnerRepo.CreatePartner(ctx, p); err != nil {
		return fmt.Errorf("failed to create partner: %w", err)
	}

	// Store the plain secret temporarily for email notification
	// (This should be sent to partner admin via secure channel)
	p.PlainAPISecret = &apiSecret

	return nil
}

// generateAPICredentials creates new API key and secret
func generateAPICredentials() (apiKey, apiSecret string, err error) {
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

	return apiKey, apiSecret, nil
}

func (uc *PartnerUsecase) GetAllPartners(ctx context.Context) ([]*domain.Partner, error) {
	partners, err := uc.partnerRepo.GetAllPartners(ctx)
	if err != nil {
		return nil, err
	}
	return partners, nil
}

func (uc *PartnerUsecase) GetPartners(ctx context.Context, partnerIDs []string) ([]*domain.Partner, error) {
	partners, err := uc.partnerRepo.GetPartnersByIDs(ctx, partnerIDs)
	if err != nil {
		return nil, err
	}
	return partners, nil
}

func (uc *PartnerUsecase) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	if id == "" {
		return nil, errors.New("invalid partner id")
	}
	return uc.partnerRepo.GetPartnerByID(ctx, id)
}

func (uc *PartnerUsecase) UpdatePartner(ctx context.Context, p *domain.Partner) error {
	if p.ID == "" {
		return errors.New("missing partner id")
	}
	return uc.partnerRepo.UpdatePartner(ctx, p)
}

func (uc *PartnerUsecase) DeletePartner(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("invalid partner id")
	}
	return uc.partnerRepo.DeletePartner(ctx, id)
}

// GetPartnersByService returns partners offering a specific service
func (uc *PartnerUsecase) GetPartnersByService(ctx context.Context, service string) ([]*domain.Partner, error) {
	if service == "" {
		return nil, errors.New("service cannot be empty")
	}
	return uc.partnerRepo.GetPartnersByService(ctx, service)
}
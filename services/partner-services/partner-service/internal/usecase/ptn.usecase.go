// usecase/partner_usecase. go
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
	"strings"
	"x/shared/utils/id"

	"go.uber.org/zap"
)

// ---------------- Partner ----------------

// CreatePartner business logic with automatic API credential generation
func (uc *PartnerUsecase) CreatePartner(ctx context.Context, p *domain.Partner) error {
	// ✅ Validate required fields
	if err := uc.validatePartnerCreate(p); err != nil {
		return err
	}

	// Generate external ID using Snowflake
	if p.ID == "" {
		p.ID = id. GenerateID("PTN")
	}

	// ✅ Set default status
	if p.Status == "" {
		p.Status = domain. PartnerStatusActive
	}

	// Set default API settings
	if p.APIRateLimit == 0 {
		p.APIRateLimit = 1000 // Default rate limit
	}
	p.  IsAPIEnabled = false // Disabled by default, admin can enable later

	// Auto-generate API credentials during partner creation
	apiKey, apiSecret, err := generateAPICredentials()
	if err != nil {
		uc.logger.Error("failed to generate API credentials",
			zap.String("partner_name", p.Name),
			zap.Error(err))
		return fmt.Errorf("failed to generate API credentials: %w", err)
	}

	// Hash the secret before storing
	hash := sha256.Sum256([]byte(apiSecret))
	apiSecretHash := hex.EncodeToString(hash[:])

	// Store credentials in partner object
	p. APIKey = &apiKey
	p.APISecretHash = &apiSecretHash

	// Create partner with API credentials
	if err := uc.partnerRepo. CreatePartner(ctx, p); err != nil {
		uc.logger.Error("failed to create partner",
			zap.String("partner_name", p.Name),
			zap.Error(err))
		return fmt.Errorf("failed to create partner:  %w", err)
	}

	// Store the plain secret temporarily for email notification
	// (This should be sent to partner admin via secure channel)
	p.PlainAPISecret = &apiSecret

	uc.logger.Info("partner created successfully",
		zap.String("partner_id", p.ID),
		zap.String("partner_name", p.Name),
		zap.String("service", p.Service),
		zap.String("currency", p. Currency),
		zap.String("local_currency", p.LocalCurrency),
		zap.Float64("rate", p.Rate))

	return nil
}

// ✅ validatePartnerCreate validates required fields for partner creation
func (uc *PartnerUsecase) validatePartnerCreate(p *domain. Partner) error {
	if p.Name == "" {
		return errors.New("partner name is required")
	}
	
	if p.Service == "" {
		return errors.New("service is required")
	}
	
	if p.Currency == "" {
		return errors.New("currency is required")
	}
	
	if p.LocalCurrency == "" {
		return errors.New("local_currency is required")
	}
	
	if p.Rate <= 0 {
		return errors.New("rate must be greater than 0")
	}

	// ✅ Validate currency format (3-letter code)
	p.LocalCurrency = strings.ToUpper(strings.TrimSpace(p.LocalCurrency))
	if len(p.LocalCurrency) != 3 {
		return errors.New("local_currency must be a 3-letter code (e.g., KES, USD, EUR)")
	}

	// ✅ Validate currency format
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if len(p.Currency) < 3 || len(p.  Currency) > 8 {
		return errors.New("currency must be 3-8 characters")
	}

	// ✅ Validate rate precision (max 8 decimal places)
	if p.Rate > 999999999.99999999 {
		return errors.New("rate exceeds maximum allowed value")
	}

	// ✅ Validate commission rate if provided
	if p.CommissionRate < 0 || p.CommissionRate > 1 {
		return errors.New("commission_rate must be between 0 and 1 (e.g., 0.005 for 0.5%)")
	}

	// ✅ Validate contact info
	if p.ContactEmail == "" && p.ContactPhone == "" {
		return errors.New("at least one contact method (email or phone) is required")
	}

	return nil
}

// generateAPICredentials creates new API key and secret
func generateAPICredentials() (apiKey, apiSecret string, err error) {
	// Generate API key (32 bytes, base64 encoded)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt. Errorf("failed to generate API key: %w", err)
	}
	apiKey = "pk_" + base64.URLEncoding.EncodeToString(keyBytes)

	// Generate API secret (48 bytes, base64 encoded)
	secretBytes := make([]byte, 48)
	if _, err := rand. Read(secretBytes); err != nil {
		return "", "", fmt. Errorf("failed to generate API secret: %w", err)
	}
	apiSecret = "sk_" + base64.URLEncoding.EncodeToString(secretBytes)

	return apiKey, apiSecret, nil
}

func (uc *PartnerUsecase) GetAllPartners(ctx context.Context) ([]*domain.Partner, error) {
	partners, err := uc.partnerRepo.GetAllPartners(ctx)
	if err != nil {
		uc.logger.Error("failed to get all partners", zap.Error(err))
		return nil, fmt.Errorf("failed to get partners: %w", err)
	}
	return partners, nil
}

func (uc *PartnerUsecase) GetPartners(ctx context.Context, partnerIDs []string) ([]*domain.Partner, error) {
	partners, err := uc. partnerRepo.GetPartnersByIDs(ctx, partnerIDs)
	if err != nil {
		uc.logger. Error("failed to get partners by IDs",
			zap.Strings("partner_ids", partnerIDs),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get partners: %w", err)
	}
	return partners, nil
}

func (uc *PartnerUsecase) GetPartnerByID(ctx context. Context, id string) (*domain.Partner, error) {
	if id == "" {
		return nil, errors.New("partner id is required")
	}
	
	partner, err := uc. partnerRepo.GetPartnerByID(ctx, id)
	if err != nil {
		uc.logger.Error("failed to get partner by ID",
			zap.String("partner_id", id),
			zap.Error(err))
		return nil, err
	}
	
	return partner, nil
}

func (uc *PartnerUsecase) UpdatePartner(ctx context.Context, p *domain.Partner) error {
	if p.ID == "" {
		return errors.New("partner id is required")
	}

	// ✅ Validate update fields if provided
	if p.LocalCurrency != "" {
		p.LocalCurrency = strings.ToUpper(strings.TrimSpace(p.LocalCurrency))
		if len(p.LocalCurrency) != 3 {
			return errors.New("local_currency must be a 3-letter code")
		}
	}

	if p.Rate < 0 {
		return errors.New("rate cannot be negative")
	}

	if p.Rate > 999999999.99999999 {
		return errors. New("rate exceeds maximum allowed value")
	}

	if p.CommissionRate < 0 || p.CommissionRate > 1 {
		return errors. New("commission_rate must be between 0 and 1")
	}

	if err := uc.partnerRepo. UpdatePartner(ctx, p); err != nil {
		uc.logger.Error("failed to update partner",
			zap.String("partner_id", p.ID),
			zap.Error(err))
		return fmt.Errorf("failed to update partner:  %w", err)
	}

	uc.logger.Info("partner updated successfully",
		zap.String("partner_id", p.ID),
		zap.String("partner_name", p.Name))

	return nil
}

func (uc *PartnerUsecase) DeletePartner(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("partner id is required")
	}

	// ✅ Check if partner exists before deleting
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, id)
	if err != nil {
		return fmt.Errorf("partner not found: %w", err)
	}

	if err := uc.partnerRepo.DeletePartner(ctx, id); err != nil {
		uc.logger.Error("failed to delete partner",
			zap. String("partner_id", id),
			zap.Error(err))
		return fmt.Errorf("failed to delete partner: %w", err)
	}

	uc.logger.Info("partner deleted successfully",
		zap.String("partner_id", id),
		zap.String("partner_name", partner.Name))

	return nil
}

// GetPartnersByService returns partners offering a specific service
func (uc *PartnerUsecase) GetPartnersByService(ctx context. Context, service string) ([]*domain.Partner, error) {
	if service == "" {
		return nil, errors.New("service is required")
	}

	partners, err := uc. partnerRepo.GetPartnersByService(ctx, service)
	if err != nil {
		uc.logger.Error("failed to get partners by service",
			zap.String("service", service),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get partners: %w", err)
	}

	return partners, nil
}

// ✅ NEW: GetPartnersByCurrency returns partners by currency
func (uc *PartnerUsecase) GetPartnersByCurrency(ctx context.Context, currency string) ([]*domain.Partner, error) {
	if currency == "" {
		return nil, errors.New("currency is required")
	}

	currency = strings.ToUpper(strings.TrimSpace(currency))
	
	partners, err := uc.partnerRepo.GetPartnersByCurrency(ctx, currency)
	if err != nil {
		uc.logger.Error("failed to get partners by currency",
			zap.String("currency", currency),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get partners: %w", err)
	}

	return partners, nil
}

// ✅ NEW: UpdatePartnerRate updates partner exchange rate
func (uc *PartnerUsecase) UpdatePartnerRate(ctx context.Context, partnerID string, newRate float64) error {
	if partnerID == "" {
		return errors.New("partner id is required")
	}

	if newRate <= 0 {
		return errors.New("rate must be greater than 0")
	}

	if newRate > 999999999.99999999 {
		return errors.New("rate exceeds maximum allowed value")
	}

	// ✅ Get partner first to log the change
	partner, err := uc.partnerRepo.GetPartnerByID(ctx, partnerID)
	if err != nil {
		return fmt.Errorf("partner not found: %w", err)
	}

	oldRate := partner.Rate

	if err := uc.partnerRepo.UpdatePartnerRate(ctx, partnerID, newRate); err != nil {
		uc. logger.Error("failed to update partner rate",
			zap.String("partner_id", partnerID),
			zap.Float64("old_rate", oldRate),
			zap.Float64("new_rate", newRate),
			zap.Error(err))
		return fmt.Errorf("failed to update rate: %w", err)
	}

	uc.logger. Info("partner rate updated",
		zap.String("partner_id", partnerID),
		zap.String("partner_name", partner.Name),
		zap.String("currency", partner.Currency),
		zap.String("local_currency", partner.LocalCurrency),
		zap.Float64("old_rate", oldRate),
		zap.Float64("new_rate", newRate),
		zap.Float64("change_percent", ((newRate-oldRate)/oldRate)*100))

	return nil
}

func (uc *PartnerUsecase) StreamAllPartners(
	ctx context.Context,
	batchSize int,
	sendFn func(*domain.Partner) error,
) error {
	return uc.partnerRepo.StreamAllPartners(ctx, batchSize, func(p *domain.Partner) error {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		return sendFn(p)
	})
}
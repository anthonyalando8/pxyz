package usecase

import (
	"context"
	"errors"
	"partner-service/internal/domain"
	"x/shared/utils/id"
)
// ---------------- Partner ----------------

// CreatePartner business logic
func (uc *PartnerUsecase) CreatePartner(ctx context.Context, p *domain.Partner) error {
	if p.Name == "" {
		return errors.New("partner name cannot be empty")
	}

	// Example: generate external ID using Snowflake
	if p.ID == "" {
		p.ID = id.GenerateID("PTN")
	}

	return uc.partnerRepo.CreatePartner(ctx, p)
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

func (uc *PartnerUsecase) UpdatePartnerUser(ctx context.Context, u *domain.PartnerUser) error {
	if u.ID == ""{
		return errors.New("missing partner_user id")
	}
	return uc.partnerUserRepo.UpdatePartnerUser(ctx, u)
}

// GetPartnersByService returns partners offering a specific service
func (uc *PartnerUsecase) GetPartnersByService(ctx context.Context, service string) ([]*domain.Partner, error) {
	if service == "" {
		return nil, errors.New("service cannot be empty")
	}
	return uc.partnerRepo.GetPartnersByService(ctx, service)
}


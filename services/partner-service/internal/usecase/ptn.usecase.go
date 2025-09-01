package usecase

import (
	"context"
	"errors"
	"fmt"
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

// ---------------- Partner Users ----------------

func (uc *PartnerUsecase) CreatePartnerUser(ctx context.Context, u *domain.PartnerUser) error {
	if u.PartnerID == "" || u.UserID == "" {
		return errors.New("partner_id and user_id are required")
	}

	if u.Role != "admin" && u.Role != "user" {
		return fmt.Errorf("invalid role: %s", u.Role)
	}

	if u.ID == ""{
		u.ID = uc.sf.Generate()
	}

	return uc.partnerUserRepo.CreatePartnerUser(ctx, u)
}

func (uc *PartnerUsecase) GetPartnerUserByID(ctx context.Context, id string) (*domain.PartnerUser, error) {
	if id == "" {
		return nil, errors.New("invalid partner_user id")
	}
	return uc.partnerUserRepo.GetPartnerUserByID(ctx, id)
}

func (uc *PartnerUsecase) UpdatePartnerUser(ctx context.Context, u *domain.PartnerUser) error {
	if u.ID == ""{
		return errors.New("missing partner_user id")
	}
	return uc.partnerUserRepo.UpdatePartnerUser(ctx, u)
}

func (uc *PartnerUsecase) DeletePartnerUser(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("invalid partner_user id")
	}
	return uc.partnerUserRepo.DeletePartnerUser(ctx, id)
}

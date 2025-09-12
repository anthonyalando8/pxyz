package usecase

import (
	"partner-service/internal/repository"
	"x/shared/utils/id"
)

type PartnerUsecase struct {
	partnerRepo     *repository.PartnerRepo
	partnerUserRepo *repository.PartnerUserRepo
	sf              *id.Snowflake
}

func NewPartnerUsecase(partnerRepo *repository.PartnerRepo, partnerUserRepo *repository.PartnerUserRepo, sf *id.Snowflake) *PartnerUsecase {
	return &PartnerUsecase{
		partnerRepo:     partnerRepo,
		partnerUserRepo: partnerUserRepo,
		sf:              sf,
	}
}

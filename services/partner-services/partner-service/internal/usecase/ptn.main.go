package usecase

import (
	"partner-service/internal/repository"
	"x/shared/utils/id"
	"go.uber.org/zap"
)

type PartnerUsecase struct {
	partnerRepo     *repository.PartnerRepo
	partnerUserRepo *repository.PartnerUserRepo
	sf              *id.Snowflake
	logger		 *zap.Logger
}

func NewPartnerUsecase(partnerRepo *repository.PartnerRepo, partnerUserRepo *repository.PartnerUserRepo, sf *id.Snowflake, logger *zap.Logger) *PartnerUsecase {
	return &PartnerUsecase{
		partnerRepo:     partnerRepo,
		partnerUserRepo: partnerUserRepo,
		sf:              sf,
		logger:		  logger,
	}
}

package usecase

import (
	"context"
	"fmt"
	"time"
	"strconv"

	"u-rbac-service/internal/domain"
	"u-rbac-service/internal/repository"
	"x/shared/utils/errors"
	"x/shared/utils/id"
)

type ModuleUsecase struct {
	rbacRepo repository.RBACRepository
	sf       *id.Snowflake
}

// NewModuleUsecase initializes ModuleUsecase
func NewModuleUsecase(rbacRepo repository.RBACRepository, sf *id.Snowflake) *ModuleUsecase {
	return &ModuleUsecase{
		rbacRepo: rbacRepo,
		sf:       sf,
	}
}

// CreateModules handles module creation (one or many)
func (uc *ModuleUsecase) CreateModules(ctx context.Context, mods []*domain.Module) ([]*domain.Module, []*xerrors.RepoError, error) {
	now := time.Now().UTC()

	// inject IDs + timestamps if missing
	for _, m := range mods {
		idStr := uc.sf.Generate()
		idInt, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("usecase: failed to parse generated ID: %w", err)
		}
		m.ID = idInt
		m.CreatedAt = now
		m.UpdatedAt = &now
	}
	created, perr, err := uc.rbacRepo.CreateModules(ctx, mods)
	if err != nil {
		return nil, perr, fmt.Errorf("usecase: failed to create modules: %w", err)
	}
	return created, perr, nil
		
}



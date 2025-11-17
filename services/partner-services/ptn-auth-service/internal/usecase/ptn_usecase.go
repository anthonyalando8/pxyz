// usecase/auth_usecase.go
package usecase

import (
	"context"
	"fmt"
	"ptn-auth-service/internal/domain"
)

func (uc *UserUsecase) GetUsersByPartnerID(ctx context.Context, partnerID string) ([]domain.User, error) {
	return uc.userRepo.GetUsersByPartnerID(ctx, partnerID)
}

func (uc *UserUsecase) GetPartnerUserStats(ctx context.Context, partnerID string) (*domain.PartnerUserStats, error) {
	return uc.userRepo.GetPartnerUserStats(ctx, partnerID)
}

func (uc *UserUsecase) GetUsersByPartnerIDPaginated(ctx context.Context, partnerID string, limit, offset int) ([]domain.User, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return uc.userRepo.GetUsersByPartnerIDWithPagination(ctx, partnerID, limit, offset)
}

func (uc *UserUsecase) UpdateUserStatus(ctx context.Context, userID, partnerID, status string) error {
	validStatuses := map[string]bool{"active": true, "suspended": true, "inactive": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}
	return uc.userRepo.UpdateUserStatus(ctx, userID, partnerID, status)
}

func (uc *UserUsecase) UpdateUserRole(ctx context.Context, userID, partnerID, role string) error {
	validRoles := map[string]bool{"partner_admin": true, "partner_user": true}
	if !validRoles[role] {
		return fmt.Errorf("invalid role: %s", role)
	}
	return uc.userRepo.UpdateUserRole(ctx, userID, partnerID, role)
}

func (uc *UserUsecase) SearchPartnerUsers(ctx context.Context, partnerID, searchTerm string, limit, offset int) ([]domain.User, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return uc.userRepo.SearchPartnerUsers(ctx, partnerID, searchTerm, limit, offset)
}

func (uc *UserUsecase) GetPartnerUserByEmail(ctx context.Context, partnerID, email string) (*domain.User, error) {
	return uc.userRepo.GetPartnerUserByEmail(ctx, partnerID, email)
}

func (uc *UserUsecase) BulkUpdateUserStatus(ctx context.Context, partnerID string, userIDs []string, status string) error {
	validStatuses := map[string]bool{"active": true, "suspended": true, "inactive": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}
	return uc.userRepo.BulkUpdateUserStatus(ctx, partnerID, userIDs, status)
}
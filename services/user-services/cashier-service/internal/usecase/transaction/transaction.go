// usecase/deposit. go
package usecase

import (
    "context"
    "cashier-service/internal/domain"
	"cashier-service/internal/repository"
)
type UserUsecase struct {
	repo *repository.UserRepo
}

func NewUserUsecase(repo *repository.UserRepo) *UserUsecase {
	return &UserUsecase{repo: repo}
}

func (uc *UserUsecase) CreateDepositRequest(ctx context.Context, req *domain.DepositRequest) error {
    return uc.repo.CreateDepositRequest(ctx, req)
}

func (uc *UserUsecase) UpdateDepositStatus(ctx context.Context, id int64, status string, errorMsg *string) error {
    return uc.repo.UpdateDepositStatus(ctx, id, status, errorMsg)
}

func (uc *UserUsecase) GetDepositByRef(ctx context. Context, requestRef string) (*domain.DepositRequest, error) {
    return uc.repo.GetDepositByRef(ctx, requestRef)
}

func (uc *UserUsecase) ListDeposits(ctx context.Context, userID int64, limit, offset int) ([]domain.DepositRequest, int64, error) {
    return uc.repo.ListDeposits(ctx, userID, limit, offset)
}

func (uc *UserUsecase) CreateWithdrawalRequest(ctx context.Context, req *domain.WithdrawalRequest) error {
    return uc.repo.CreateWithdrawalRequest(ctx, req)
}

func (uc *UserUsecase) UpdateWithdrawalStatus(ctx context.Context, id int64, status string, errorMsg *string) error {
    return uc.repo.UpdateWithdrawalStatus(ctx, id, status, errorMsg)
}

func (uc *UserUsecase) UpdateWithdrawalWithReceipt(ctx context.Context, id int64, receiptCode string, journalID int64) error {
    return uc.repo.UpdateWithdrawalWithReceipt(ctx, id, receiptCode, journalID)
}

func (uc *UserUsecase) ListWithdrawals(ctx context.Context, userID int64, limit, offset int) ([]domain.WithdrawalRequest, int64, error) {
    return uc.repo.ListWithdrawals(ctx, userID, limit, offset)
}
package usecase

import (
	"context"
	"errors"

	"accounting-service/internal/domain"
	"accounting-service/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

)

// TransactionFeeUsecase handles business logic for transaction fees
type TransactionFeeUsecase struct {
	feeRepo repository.TransactionFeeRepository
	redisClient        *redis.Client

}

// NewTransactionFeeUsecase initializes a new TransactionFeeUsecase
func NewTransactionFeeUsecase(feeRepo repository.TransactionFeeRepository, 	redisClient *redis.Client) *TransactionFeeUsecase {
	return &TransactionFeeUsecase{
		feeRepo: feeRepo,
		redisClient: redisClient,
	}
}

// BeginTx starts a db transaction
func (uc *TransactionFeeUsecase) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return uc.feeRepo.BeginTx(ctx)
}

// BatchCreate inserts multiple transaction fees at once
func (uc *TransactionFeeUsecase) BatchCreate(ctx context.Context, fees []*domain.TransactionFee, tx pgx.Tx) map[int]error {
	if len(fees) == 0 {
		return map[int]error{0: errors.New("transaction fees list is empty")}
	}
	return uc.feeRepo.CreateBatch(ctx, fees, tx)
}

// GetByReceipt fetches all fees for a given receipt
func (uc *TransactionFeeUsecase) GetByReceipt(ctx context.Context, receiptCode string) ([]*domain.TransactionFee, error) {
	if receiptCode == "" {
		return nil, errors.New("receipt code cannot be empty")
	}
	return uc.feeRepo.GetByReceipt(ctx, receiptCode)
}

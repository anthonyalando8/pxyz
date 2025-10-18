// --- transaction/service.go ---
package transaction

import (
	"context"
	"time"

	"wallet-service/internal/domain"
	"wallet-service/internal/repository"
)

type Service struct {
	repo repository.WalletTransactionRepository
}

func New(repo repository.WalletTransactionRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTransaction(ctx context.Context, tx *domain.WalletTransaction) error {
	tx.CreatedAt = time.Now()
	if tx.TxStatus == "" {
		tx.TxStatus = "pending"
	}
	return s.repo.CreateTransaction(ctx, tx)
}

func (s *Service) ListByUser(ctx context.Context, userID string) ([]*domain.WalletTransaction, error) {
	return s.repo.ListTransactionsByUserID(ctx, userID)
}
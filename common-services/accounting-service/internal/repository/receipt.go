package repository

import (
	"accounting-service/internal/domain"
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReceiptRepository interface {
	Create(ctx context.Context, r *domain.Receipt, tx pgx.Tx) error
	GetByCode(ctx context.Context, code string) (*domain.Receipt, error)
	ListByAccount(ctx context.Context, accountID int64) ([]*domain.Receipt, error)
}

type receiptRepo struct {
	db *pgxpool.Pool
}

// Create implements ReceiptRepository.
func (*receiptRepo) Create(ctx context.Context, r *domain.Receipt, tx pgx.Tx) error {
	panic("unimplemented")
}

// GetByCode implements ReceiptRepository.
func (r *receiptRepo) GetByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	panic("unimplemented")
}

// ListByAccount implements ReceiptRepository.
func (r *receiptRepo) ListByAccount(ctx context.Context, accountID int64) ([]*domain.Receipt, error) {
	panic("unimplemented")
}

func NewReceiptRepo(db *pgxpool.Pool) ReceiptRepository {
	return &receiptRepo{db: db}
}

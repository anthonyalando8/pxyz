package usecase

import (
	"context"
	"payment-service/internal/domain"
)

func (uc *PaymentUsecase) SetError(ctx context.Context, id int64, errorMsg string) error {
	return uc.paymentRepo.SetError(ctx, id, errorMsg)
}

func (uc *PaymentUsecase) UpdateStatus(ctx context.Context, id int64, status domain.PaymentStatus) error {
	return uc.paymentRepo.UpdateStatus(ctx, id, status)
}

func (uc *PaymentUsecase) GetByPaymentRef(ctx context.Context, paymentRef string) (*domain.Payment, error) {
	return uc.paymentRepo.GetByPaymentRef(ctx, paymentRef)
}
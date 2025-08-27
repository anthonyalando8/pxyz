// usecase/payment_uc.go
package mpesausecase

import "cashier-service/internal/domain"

type PaymentUsecase struct {
    providers map[string]domain.Provider
}

func NewPaymentUsecase(p []domain.Provider) *PaymentUsecase {
    m := make(map[string]domain.Provider)
    for _, prov := range p {
        m[prov.Name()] = prov
    }
    return &PaymentUsecase{providers: m}
}

func (uc *PaymentUsecase) Deposit(provider string, req domain.DepositRequest) (domain.DepositResponse, error) {
    prov, ok := uc.providers[provider]
    if !ok {
        return domain.DepositResponse{}, domain.ErrProviderNotFound
    }
    return prov.Deposit(req)
}

func (uc *PaymentUsecase) Withdraw(provider string, req domain.WithdrawRequest) (domain.WithdrawResponse, error) {
    prov, ok := uc.providers[provider]
    if !ok {
        return domain.WithdrawResponse{}, domain.ErrProviderNotFound
    }
    return prov.Withdraw(req)
}

// handler/payment_handler.go
package handler

import (
    "encoding/json"
    "net/http"
    "cashier-service/internal/usecase/mpesa"
    "cashier-service/internal/domain"
)

type PaymentHandler struct {
    uc *mpesausecase.PaymentUsecase
}

func NewPaymentHandler(uc *mpesausecase.PaymentUsecase) *PaymentHandler {
    return &PaymentHandler{uc: uc}
}

func (h *PaymentHandler) Deposit(w http.ResponseWriter, r *http.Request) {
    var req domain.DepositRequest
    json.NewDecoder(r.Body).Decode(&req)

    resp, err := h.uc.Deposit(req.Provider, req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    json.NewEncoder(w).Encode(resp)
}

func (h *PaymentHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
    var req domain.WithdrawRequest
    json.NewDecoder(r.Body).Decode(&req)

    resp, err := h.uc.Withdraw(req.Provider, req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    json.NewEncoder(w).Encode(resp)
}

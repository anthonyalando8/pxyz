// handler/payment_handler.go
package handler

import (
	"cashier-service/internal/domain"
	"cashier-service/internal/usecase/mpesa"
	"encoding/json"
	"fmt"
	"net/http"
    partnerclient "x/shared/partner"
	accountingclient "x/shared/common/accounting"

	"x/shared/response"

    notificationclient "x/shared/notification"

	// notificationpb "x/shared/genproto/shared/notificationpb"

	// "github.com/google/uuid"
	// "google.golang.org/protobuf/types/known/structpb"
)

type PaymentHandler struct {
    uc *mpesausecase.PaymentUsecase
    partnerClient    *partnerclient.PartnerService
	accountingClient *accountingclient.AccountingClient
    notificationClient *notificationclient.NotificationService
}

func NewPaymentHandler(
    uc *mpesausecase.PaymentUsecase,
    partnerClient    *partnerclient.PartnerService,
	accountingClient *accountingclient.AccountingClient,
    notificationClient *notificationclient.NotificationService,

) *PaymentHandler {
    return &PaymentHandler{
        uc: uc,
        partnerClient:    partnerClient,
		accountingClient: accountingClient,
        notificationClient: notificationClient,
    }
}

func (h *PaymentHandler) Deposit(w http.ResponseWriter, r *http.Request) {
    var req domain.DepositRequest
    json.NewDecoder(r.Body).Decode(&req)

    resp, err := h.uc.Deposit(req.Provider, req)
    if err != nil {
        response.Error(w, http.StatusBadRequest, err.Error(),)
        return
    }
	response.JSON(w, http.StatusOK, resp)
    //json.NewEncoder(w).Encode(resp)
}

func (h *PaymentHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
    var req domain.WithdrawRequest
    json.NewDecoder(r.Body).Decode(&req)

    resp, err := h.uc.Withdraw(req.Provider, req)
    if err != nil {
        response.Error(w,  http.StatusBadRequest,err.Error(),)
        return
    }
	response.JSON(w, http.StatusOK, resp)
    //json.NewEncoder(w).Encode(resp)
}


func (h *PaymentHandler) MpesaCallback(w http.ResponseWriter, r *http.Request) {
    var callbackData map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&callbackData); err != nil {
        response.Error(w,http.StatusBadRequest, "invalid callback payload", )
        return
    }

    // TODO: call usecase to process callback (update transaction status, etc.)
    fmt.Printf("Received Mpesa callback: %+v\n", callbackData)

    // Respond with 200 so Safaricom stops retrying
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"ResultCode":0, "ResultDesc":"Success"}`))
}
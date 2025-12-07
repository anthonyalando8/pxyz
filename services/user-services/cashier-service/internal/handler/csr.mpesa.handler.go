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
    "cashier-service/internal/usecase/transaction"
    notificationclient "x/shared/notification"

    	"x/shared/auth/otp"
    "x/shared/account"
    //"cashier-service/internal/usecase"
    "github.com/redis/go-redis/v9"
    "go.uber.org/zap"

    "x/shared/utils/profile"

	// notificationpb "x/shared/genproto/shared/notificationpb"

	// "github.com/google/uuid"
	// "google.golang.org/protobuf/types/known/structpb"
)

type PaymentHandler struct {
	uc                 *mpesausecase.PaymentUsecase
	partnerClient      *partnerclient.PartnerService
	accountingClient   *accountingclient. AccountingClient
	notificationClient *notificationclient. NotificationService
	userUc             *usecase.UserUsecase
	hub                *Hub
	otp                *otpclient.OTPService
	accountClient      *accountclient.  AccountClient
	rdb                *redis.Client //  Add Redis client
	logger             *zap.Logger   //  Add logger
    profileFetcher     *helpers.ProfileFetcher

}

func NewPaymentHandler(
	uc *mpesausecase.PaymentUsecase,
	partnerClient *partnerclient.PartnerService,
	accountingClient *accountingclient.AccountingClient,
	notificationClient *notificationclient.NotificationService,
	userUc *usecase.UserUsecase,
	hub *Hub,
	otp *otpclient.OTPService,
	accountClient *accountclient.AccountClient,
	rdb *redis.Client, //  Add Redis
	logger *zap.Logger, //  Add logger
    profileFetcher     *helpers.ProfileFetcher,

) *PaymentHandler {
	return &PaymentHandler{
		uc:                 uc,
		partnerClient:      partnerClient,
		accountingClient:   accountingClient,
		notificationClient: notificationClient,
		userUc:             userUc,
		hub:                hub,
		otp:                otp,
		accountClient:      accountClient,
		rdb:                rdb,
		logger:             logger,
        profileFetcher:     profileFetcher,
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
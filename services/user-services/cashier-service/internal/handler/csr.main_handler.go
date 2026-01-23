// handler/payment_handler.go
package handler

import (
    partnerclient "x/shared/partner"
	accountingclient "x/shared/common/accounting"

    "cashier-service/internal/usecase/transaction"
    notificationclient "x/shared/notification"

    	"x/shared/auth/otp"
    "x/shared/account"
    //"cashier-service/internal/usecase"
    "github.com/redis/go-redis/v9"
    "go.uber.org/zap"

    "x/shared/utils/profile"
	cryptoclient "x/shared/common/crypto"


	// notificationpb "x/shared/genproto/shared/notificationpb"

	// "github.com/google/uuid"
	// "google.golang.org/protobuf/types/known/structpb"
)

type PaymentHandler struct {
	partnerClient      *partnerclient.PartnerService
	accountingClient   *accountingclient. AccountingClient
	notificationClient *notificationclient. NotificationService
	userUc             *usecase.UserUsecase
	hub                *Hub
	otp                *otpclient.OTPService
	accountClient      *accountclient.  AccountClient
	cryptoClient  *cryptoclient.CryptoClient
	rdb                *redis.Client //  Add Redis client
	logger             *zap.Logger   //  Add logger
    profileFetcher     *helpers.ProfileFetcher

}

func NewPaymentHandler(
	partnerClient *partnerclient.PartnerService,
	accountingClient *accountingclient.AccountingClient,
	notificationClient *notificationclient.NotificationService,
	userUc *usecase.UserUsecase,
	hub *Hub,
	otp *otpclient.OTPService,
	accountClient *accountclient.AccountClient,
	cryptoClient  *cryptoclient.CryptoClient,
	rdb *redis.Client, //  Add Redis
	logger *zap.Logger, //  Add logger
    profileFetcher     *helpers.ProfileFetcher,

) *PaymentHandler {
	return &PaymentHandler{
		partnerClient:      partnerClient,
		accountingClient:   accountingClient,
		notificationClient: notificationClient,
		userUc:             userUc,
		hub:                hub,
		otp:                otp,
		accountClient:      accountClient,
		cryptoClient:  cryptoClient,
		rdb:                rdb,
		logger:             logger,
        profileFetcher:     profileFetcher,
	}
}
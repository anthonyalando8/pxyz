package handler

import (
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	coreclient "x/shared/core"
	partnerclient "x/shared/partner"
	accountingclient "x/shared/common/accounting" // 👈 added
	"github.com/redis/go-redis/v9"
)

type AdminHandler struct {
	auth             *middleware.MiddlewareWithClient
	otp              *otpclient.OTPService
	emailClient      *emailclient.EmailClient
	smsClient        *smsclient.SMSClient
	redisClient      *redis.Client
	coreClient       *coreclient.CoreService
	partnerClient    *partnerclient.PartnerService
	accountingClient *accountingclient.AccountingClient // 👈 added
}

func NewAdminHandler(
	auth *middleware.MiddlewareWithClient,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	redisClient *redis.Client,
	coreClient *coreclient.CoreService,
	partnerClient *partnerclient.PartnerService,
	accountingClient *accountingclient.AccountingClient, // 👈 added
) *AdminHandler {
	return &AdminHandler{
		auth:             auth,
		otp:              otp,
		emailClient:      emailClient,
		smsClient:        smsClient,
		redisClient:      redisClient,
		coreClient:       coreClient,
		partnerClient:    partnerClient,
		accountingClient: accountingClient, // 👈 added
	}
}
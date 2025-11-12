package handler

import (
	"ptn-auth-service/internal/usecase"
	"x/shared/auth/middleware"
	otpclient "x/shared/auth/otp"
	coreclient "x/shared/core"
	emailclient "x/shared/email"
	sessionpb "x/shared/genproto/partner/sessionpb"
	smsclient "x/shared/sms"
	"github.com/redis/go-redis/v9"
	notificationclient "x/shared/notification" // ✅ added
	urbacservice "x/shared/factory/partner/urbac/utils"

	//"x/shared/genproto/emailpb"
)

type AuthHandler struct {
	uc            *usecase.UserUsecase
	auth          *middleware.MiddlewareWithClient
	otp           *otpclient.OTPService
	emailClient   *emailclient.EmailClient
	smsClient     *smsclient.SMSClient
	redisClient   *redis.Client // <- added
	coreClient    *coreclient.CoreService
	sessionClient sessionpb.PartnerSessionServiceClient
	notificationClient *notificationclient.NotificationService // ✅ added
	urbacservice  *urbacservice.Service

}

func NewAuthHandler(
	uc *usecase.UserUsecase,
	auth *middleware.MiddlewareWithClient,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	redisClient *redis.Client, // <- added
	coreClient *coreclient.CoreService,
	sessionClient sessionpb.PartnerSessionServiceClient,
	notificationClient *notificationclient.NotificationService, // ✅ added
	urbacservice *urbacservice.Service,

) *AuthHandler {
	return &AuthHandler{
		uc:            uc,
		auth:          auth,
		otp:           otp,
		emailClient:   emailClient,
		smsClient:     smsClient,
		redisClient:   redisClient, // <- assigned
		coreClient:    coreClient,
		sessionClient: sessionClient,
		notificationClient: notificationClient, //
		urbacservice:  urbacservice,
	}
}

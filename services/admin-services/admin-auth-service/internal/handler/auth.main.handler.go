package handler

import (
	"admin-auth-service/internal/usecase"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	coreclient "x/shared/core"
	"github.com/redis/go-redis/v9"
	sessionpb "x/shared/genproto/admin/sessionpb"
	notificationclient "x/shared/notification" // ✅ added
	urbacservice "x/shared/factory/admin/urbac/utils"

	//"x/shared/genproto/emailpb"
)


type AuthHandler struct {
	uc             *usecase.UserUsecase
	auth           *middleware.MiddlewareWithClient
	otp            *otpclient.OTPService
	emailClient    *emailclient.EmailClient
	smsClient      *smsclient.SMSClient
	redisClient    *redis.Client      // <- added
	coreClient		*coreclient.CoreService
	sessionClient sessionpb.AdminSessionServiceClient
	notificationClient *notificationclient.NotificationService // ✅ added
	urbacservice  *urbacservice.Service

}

func NewAuthHandler(
	uc *usecase.UserUsecase,
	auth *middleware.MiddlewareWithClient,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	redisClient *redis.Client,           // <- added
	coreClient		*coreclient.CoreService,
	sessionClient sessionpb.AdminSessionServiceClient,
	notificationClient *notificationclient.NotificationService, // ✅ added
	urbacservice *urbacservice.Service,

) *AuthHandler {
	return &AuthHandler{
		uc:             uc,
		auth:           auth,
		otp:            otp,
		emailClient:    emailClient,
		smsClient:      smsClient,
		redisClient:    redisClient,      // <- assigned
		coreClient:		coreClient,
		sessionClient: sessionClient,
		notificationClient: notificationClient, // ✅ assigned
		urbacservice:  urbacservice,
	}
}



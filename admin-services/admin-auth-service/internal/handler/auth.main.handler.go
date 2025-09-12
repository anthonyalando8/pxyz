package handler

import (
	"admin-auth-service/internal/usecase"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	coreclient "x/shared/core"
	urbacservice "x/shared/urbac/utils"
	"github.com/redis/go-redis/v9"
	sessionpb "x/shared/genproto/sessionpb"

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
	urbacservice  *urbacservice.Service
	sessionClient sessionpb.AuthServiceClient
}


func NewAuthHandler(
	uc *usecase.UserUsecase,
	auth *middleware.MiddlewareWithClient,
	otp *otpclient.OTPService,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	redisClient *redis.Client,           // <- added
	coreClient		*coreclient.CoreService,
	urbacservice  *urbacservice.Service,
	sessionClient sessionpb.AuthServiceClient,
) *AuthHandler {
	return &AuthHandler{
		uc:             uc,
		auth:           auth,
		otp:            otp,
		emailClient:    emailClient,
		smsClient:      smsClient,
		redisClient:    redisClient,      // <- assigned
		coreClient:		coreClient,
		urbacservice: urbacservice,
		sessionClient: sessionClient,
	}
}



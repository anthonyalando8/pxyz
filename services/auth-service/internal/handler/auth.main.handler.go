package handler

import (
	"auth-service/internal/usecase"
	"auth-service/internal/config"
	accountclient "x/shared/account"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	coreclient "x/shared/core"
	telegramclient "auth-service/internal/service/telegram"
	urbacservice "x/shared/urbac/utils"
	"github.com/redis/go-redis/v9"
	sessionpb "x/shared/genproto/sessionpb"

	//"x/shared/genproto/emailpb"

)

type Config struct {
    GoogleClientID string
	Apple config.AppleConfig
}

type AuthHandler struct {
	uc             *usecase.UserUsecase
	auth           *middleware.MiddlewareWithClient
	otp            *otpclient.OTPService
	accountClient  *accountclient.AccountClient
	emailClient    *emailclient.EmailClient
	smsClient      *smsclient.SMSClient
	redisClient    *redis.Client      // <- added
	coreClient		*coreclient.CoreService
	urbacservice  *urbacservice.Service
	sessionClient sessionpb.AuthServiceClient
	config         *Config
	telegramClient *telegramclient.TelegramClient
	
}


func NewAuthHandler(
	uc *usecase.UserUsecase,
	auth *middleware.MiddlewareWithClient,
	otp *otpclient.OTPService,
	accountClient *accountclient.AccountClient,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	redisClient *redis.Client,           // <- added
	coreClient		*coreclient.CoreService,
	urbacservice  *urbacservice.Service,
	sessionClient sessionpb.AuthServiceClient,

	config *Config,
	telegramClient *telegramclient.TelegramClient,
) *AuthHandler {
	return &AuthHandler{
		uc:             uc,
		auth:           auth,
		otp:            otp,
		accountClient:  accountClient,
		emailClient:    emailClient,
		smsClient:      smsClient,
		redisClient:    redisClient,      // <- assigned
		coreClient:		coreClient,
		urbacservice: urbacservice,
		sessionClient: sessionClient,
		config:         config,
		telegramClient: telegramClient,
	}
}



package handler

import (
	"auth-service/internal/usecase"
	"auth-service/internal/config"
	telegramclient "auth-service/internal/service/telegram"

	accountclient "x/shared/account"
	coreclient "x/shared/core"
	notificationclient "x/shared/notification" // ✅ added
	urbacservice "x/shared/urbac/utils"

	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"

	sessionpb "x/shared/genproto/sessionpb"
	"auth-service/internal/ws"

	"github.com/redis/go-redis/v9"
	oauth2s "auth-service/internal/service/app_oauth2_client"

)

type Config struct {
	GoogleClientID string
	Apple          config.AppleConfig
}

type AuthHandler struct {
	uc                 *usecase.UserUsecase
	auth               *middleware.MiddlewareWithClient
	otp                *otpclient.OTPService
	accountClient      *accountclient.AccountClient
	emailClient        *emailclient.EmailClient
	smsClient          *smsclient.SMSClient
	redisClient        *redis.Client
	coreClient         *coreclient.CoreService
	notificationClient *notificationclient.NotificationService // ✅ added
	urbacservice       *urbacservice.Service
	sessionClient      sessionpb.AuthServiceClient
	config             *Config
	telegramClient     *telegramclient.TelegramClient
	oauth2Svc *oauth2s.OAuth2Service
	publisher *ws.AuthEventPublisher
}

func NewAuthHandler(
	uc *usecase.UserUsecase,
	auth *middleware.MiddlewareWithClient,
	otp *otpclient.OTPService,
	accountClient *accountclient.AccountClient,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
	redisClient *redis.Client,
	coreClient *coreclient.CoreService,
	notificationClient *notificationclient.NotificationService, // added
	urbacservice *urbacservice.Service,
	sessionClient sessionpb.AuthServiceClient,
	config *Config,
	telegramClient *telegramclient.TelegramClient,
	oauth2Svc *oauth2s.OAuth2Service,
	publisher *ws.AuthEventPublisher,
) *AuthHandler {
	return &AuthHandler{
		uc:                 uc,
		auth:               auth,
		otp:                otp,
		accountClient:      accountClient,
		emailClient:        emailClient,
		smsClient:          smsClient,
		redisClient:        redisClient,
		coreClient:         coreClient,
		notificationClient: notificationClient, // ✅ assigned
		urbacservice:       urbacservice,
		sessionClient:      sessionClient,
		config:             config,
		telegramClient:     telegramClient,
		oauth2Svc: oauth2Svc,
		publisher: 		publisher,
	}
}




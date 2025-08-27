package handler

import (
	"auth-service/internal/usecase"
	"auth-service/internal/config"
	accountclient "x/shared/account"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	telegramclient "auth-service/internal/service/telegram"
	//"x/shared/genproto/emailpb"

)

type Config struct {
    GoogleClientID string
	Apple config.AppleConfig
}

type AuthHandler struct {
	uc     *usecase.UserUsecase
	auth   *middleware.MiddlewareWithClient
	otp *otpclient.OTPService
	accountClient *accountclient.AccountClient 
	emailClient *emailclient.EmailClient
	//config Config
	config *Config
	telegramClient *telegramclient.TelegramClient

}

func NewAuthHandler(uc *usecase.UserUsecase, auth *middleware.MiddlewareWithClient, otp *otpclient.OTPService, accountClient *accountclient.AccountClient, emailClient *emailclient.EmailClient, config *Config, telegramClient *telegramclient.TelegramClient, ) *AuthHandler {
	return &AuthHandler{uc: uc, auth: auth, otp: otp, accountClient: accountClient, emailClient: emailClient, config: config, telegramClient: telegramClient,}
}

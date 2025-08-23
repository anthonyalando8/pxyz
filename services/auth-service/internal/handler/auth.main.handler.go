package handler

import (
	"auth-service/internal/usecase"
	accountclient "x/shared/account"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
	emailclient "x/shared/email"
	//"x/shared/genproto/emailpb"
)

type AuthHandler struct {
	uc     *usecase.UserUsecase
	auth   *middleware.MiddlewareWithClient
	otp *otpclient.OTPService
	accountClient *accountclient.AccountClient 
	emailClient *emailclient.EmailClient
}

func NewAuthHandler(uc *usecase.UserUsecase, auth *middleware.MiddlewareWithClient, otp *otpclient.OTPService, accountClient *accountclient.AccountClient, emailClient *emailclient.EmailClient ) *AuthHandler {
	return &AuthHandler{uc: uc, auth: auth, otp: otp, accountClient: accountClient, emailClient: emailClient,}
}

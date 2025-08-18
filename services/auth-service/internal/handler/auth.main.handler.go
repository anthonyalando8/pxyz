package handler

import (
	"auth-service/internal/usecase"
	accountclient "x/shared/account"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
)

type AuthHandler struct {
	uc     *usecase.UserUsecase
	auth   *middleware.MiddlewareWithClient
	otp *otpclient.OTPService
	accountClient *accountclient.AccountClient 
}

func NewAuthHandler(uc *usecase.UserUsecase, auth *middleware.MiddlewareWithClient, otp *otpclient.OTPService, accountClient *accountclient.AccountClient ) *AuthHandler {
	return &AuthHandler{uc: uc, auth: auth, otp: otp, accountClient: accountClient,}
}

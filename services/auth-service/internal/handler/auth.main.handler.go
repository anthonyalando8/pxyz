package handler

import (
	"auth-service/internal/usecase"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
)

type AuthHandler struct {
	uc     *usecase.UserUsecase
	auth   *middleware.MiddlewareWithClient
	otp *otpclient.OTPService
}

func NewAuthHandler(uc *usecase.UserUsecase, auth *middleware.MiddlewareWithClient, otp *otpclient.OTPService) *AuthHandler {
	return &AuthHandler{uc: uc, auth: auth, otp: otp}
}

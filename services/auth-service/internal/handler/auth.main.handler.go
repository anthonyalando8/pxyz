package handler

import (
	"auth-service/internal/usecase"
	"auth-service/pkg/jwtutil"
	"x/shared/auth/middleware"
	"x/shared/auth/otp"
)

type AuthHandler struct {
	uc     *usecase.UserUsecase
	jwtGen *jwtutil.Generator
	auth   *middleware.MiddlewareWithClient
	otp *otpclient.OTPService
}

func NewAuthHandler(uc *usecase.UserUsecase, jwtGen *jwtutil.Generator, auth *middleware.MiddlewareWithClient, otp *otpclient.OTPService) *AuthHandler {
	return &AuthHandler{uc: uc, jwtGen: jwtGen, auth: auth, otp: otp}
}

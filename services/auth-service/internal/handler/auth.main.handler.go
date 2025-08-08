package handler

import (
	"auth-service/internal/usecase"
	"auth-service/pkg/jwtutil"
	"x/shared/auth/middleware"
)

type AuthHandler struct {
	uc     *usecase.UserUsecase
	jwtGen *jwtutil.Generator
	auth   *middleware.MiddlewareWithClient
}

func NewAuthHandler(uc *usecase.UserUsecase, jwtGen *jwtutil.Generator, auth *middleware.MiddlewareWithClient) *AuthHandler {
	return &AuthHandler{uc: uc, jwtGen: jwtGen, auth: auth}
}

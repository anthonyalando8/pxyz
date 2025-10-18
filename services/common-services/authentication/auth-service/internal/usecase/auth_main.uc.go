// usecase/user_usecase.go
package usecase

import (
	"auth-service/internal/repository"
	"x/shared/utils/cache"

	"x/shared/utils/id"
		"x/shared/auth/otp"

)

type UserUsecase struct {
	userRepo      *repository.UserRepository
	Sf            *id.Snowflake
	cache         *cache.Cache
	kafkaProducer UserEventProducer
	otp                *otpclient.OTPService

}

func NewUserUsecase(
	userRepo *repository.UserRepository,
	sf *id.Snowflake,
	cache *cache.Cache,
	kafkaProducer UserEventProducer,
	otp                *otpclient.OTPService,
) *UserUsecase {
	return &UserUsecase{
		userRepo:      userRepo,
		Sf:            sf,
		cache:         cache,
		kafkaProducer: kafkaProducer,
		otp:           otp,
	}
}
package server

import (
	"log"
	"net/http"

	"partner-service/internal/config"
	"partner-service/internal/handler"
	"partner-service/internal/repository"
	"partner-service/internal/router"
	"partner-service/internal/usecase"
	"x/shared/auth/middleware"
	otp "x/shared/auth/otp"
	email "x/shared/email"
	sms "x/shared/sms"
	"x/shared/utils/id"
	authclient "x/shared/auth"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func NewServer(cfg config.AppConfig) *http.Server {
	// DB connection
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}

	// Repositories
	partnerRepo := repository.NewPartnerRepo(db)
	partnerUserRepo := repository.NewPartnerUserRepo(db)

	// Snowflake for ID generation
	sf, err := id.NewSnowflake(11)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	// Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	// Usecase
	partnerUC := usecase.NewPartnerUsecase(partnerRepo, partnerUserRepo, sf)

	// Middleware and service clients
	authMiddleware := middleware.RequireAuth()
	otpSvc := otp.NewOTPService()
	emailCli := email.NewEmailClient()
	smsCli := sms.NewSMSClient()

	// Auth-service client
	authClient, err := authclient.DialAuthService(authclient.PartnerAuthService)
	if err != nil {
        log.Fatalf("failed to dial auth service: %v", err)
    }

	// Handler
	partnerHandler := handler.NewPartnerHandler(
		partnerUC,
		authClient,
		otpSvc,
		emailCli,
		smsCli,
	)



	// HTTP router
	r := chi.NewRouter()
	r = router.SetupRoutes(r, partnerHandler, authMiddleware, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}

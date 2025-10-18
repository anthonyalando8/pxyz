package server

import (
	"log"
	"net/http"

	"cashier-service/internal/config"
	"cashier-service/internal/domain"
	"cashier-service/internal/handler"
	"cashier-service/internal/provider/mpesa"
	"cashier-service/internal/router"
	mpesausecase "cashier-service/internal/usecase/mpesa"
	partnerclient "x/shared/partner"
	accountingclient "x/shared/common/accounting"
	notificationclient "x/shared/notification"

	"x/shared/auth/middleware"
	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func NewServer(cfg config.AppConfig) *http.Server {
	// --- Connect Postgres ---
	db, err := config.ConnectDB()
	_ = db
	if err != nil {
		log.Fatalf("failed to connect DB: %v", err)
	}

	// --- Init ID generator ---
	sf, err := id.NewSnowflake(9)
	_ = sf
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	// --- Init Redis ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	partnerSvc := partnerclient.NewPartnerService()
	accountingClient := accountingclient.NewAccountingClient()
	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client

	// --- Init Payment Providers ---
	mpesaClient := mpesa.NewMpesaClient(
		cfg.MpesaBaseURL,
		cfg.MpesaConsumerKey,
		cfg.MpesaConsumerSecret,
		cfg.MpesaPassKey,
		cfg.MpesaShortCode,
	)
	mpesaProvider := mpesa.NewMpesaProvider(mpesaClient)

	// Register providers in usecase
	paymentUC := mpesausecase.NewPaymentUsecase([]domain.Provider{mpesaProvider})


	// --- Init Middleware ---
	auth := middleware.RequireAuth()

	// --- Handlers ---
	paymentHandler := handler.NewPaymentHandler(paymentUC, partnerSvc, accountingClient,notificationCli,)

	// --- Router ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, paymentHandler, auth, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}

package server

import (
	"context"
	"log"
	"net/http"

	"cashier-service/internal/config"
	"cashier-service/internal/domain"
	"cashier-service/internal/handler"
	"cashier-service/internal/provider/mpesa"
	"cashier-service/internal/repository"
	"cashier-service/internal/router"
	mpesausecase "cashier-service/internal/usecase/mpesa"
	usecase "cashier-service/internal/usecase/transaction"

	"cashier-service/internal/sub"
	"x/shared/auth/middleware"
	accountingclient "x/shared/common/accounting"
	notificationclient "x/shared/notification"
	partnerclient "x/shared/partner"
	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func NewServer(cfg config.AppConfig) *http.Server {
	// --- Connect Postgres ---
	db, err := config.ConnectDB()
	if err != nil {
		log. Fatalf("failed to connect DB: %v", err)
	}

	// --- Init ID generator ---
	sf, err := id.NewSnowflake(9)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}
	_ = sf

	// --- Init Redis ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	// Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	log.Println("[Redis] ✅ Connected successfully")
	// --- Init gRPC Clients ---
	partnerSvc := partnerclient. NewPartnerService()
	accountingClient := accountingclient. NewAccountingClient()
	notificationCli := notificationclient.NewNotificationService()

	// --- Init Payment Providers ---
	mpesaClient := mpesa.NewMpesaClient(
		cfg.MpesaBaseURL,
		cfg.MpesaConsumerKey,
		cfg.MpesaConsumerSecret,
		cfg.MpesaPassKey,
		cfg.MpesaShortCode,
	)
	mpesaProvider := mpesa.NewMpesaProvider(mpesaClient)

	// --- Init Repositories ---
	userRepo := repository.NewUserRepository(db)

	// --- Init Usecases ---
	paymentUC := mpesausecase.NewPaymentUsecase([]domain.Provider{mpesaProvider})
	userUC := usecase.NewUserUsecase(userRepo)

	// --- Init WebSocket Hub ---
	hub := handler.NewHub()
	go hub.Run() // Start hub in background goroutine
	log. Println("[WebSocket] Hub started")

	transactionSub := subscriber.NewTransactionEventSubscriber(rdb, hub)
	if err := transactionSub.Start(ctx); err != nil {
		log.Fatalf("failed to start transaction subscriber: %v", err)
	}
	log.Println("[TransactionSubscriber] ✅ Started")

	// --- Init Middleware ---
	auth := middleware.RequireAuth()

	// --- Init Handlers ---
	paymentHandler := handler.NewPaymentHandler(
		paymentUC,partnerSvc,
		accountingClient,
		notificationCli,
		userUC,
		hub,
		//sf,
	)

	// --- Router ---
	r := chi. NewRouter()
	r = router.SetupRoutes(r, paymentHandler, auth, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg. HTTPAddr,
		Handler: r,
	}
}
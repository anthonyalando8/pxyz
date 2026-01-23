package server

import (
	"context"
	"log"
	"net/http"

	"cashier-service/internal/config"
	"cashier-service/internal/handler"
	"cashier-service/internal/repository"
	"cashier-service/internal/router"
	"cashier-service/internal/sub"
	transaction "cashier-service/internal/event_handler"
	usecase "cashier-service/internal/usecase/transaction"
	authclient "x/shared/auth"


	"x/shared/auth/middleware"
	accountingclient "x/shared/common/accounting"
	notificationclient "x/shared/notification"
	partnerclient "x/shared/partner"
	"x/shared/utils/id"
	"x/shared/auth/otp"
	accountclient "x/shared/account"
	cryptoclient "x/shared/common/crypto"

	"x/shared/utils/profile"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewServer(cfg config.AppConfig) *http. Server {
	// --- Init Logger ---
	logger, err := zap. NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// --- Connect Postgres ---
	db, err := config. ConnectDB()
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
		log.  Fatalf("failed to connect to Redis: %v", err)
	}
	log.Println("[Redis] ✅ Connected successfully")

	authClient, err := authclient.DialAuthService(authclient.AllAuthServices)
	if err != nil {
        log.Fatalf("failed to dial auth service: %v", err)
    }
	profileFetcher := helpers.NewProfileFetcher(authClient)


	otpSvc := otpclient.NewOTPService()
	accountClient := accountclient.NewAccountClient()

	cryptoClient := cryptoclient.NewCryptoClient()
	if cryptoClient == nil {
		log.Println("⚠️  Crypto service unavailable - wallets will not be created")
	}

	// --- Init gRPC Clients ---
	partnerSvc := partnerclient. NewPartnerService()
	accountingClient := accountingclient. NewAccountingClient()
	notificationCli := notificationclient.NewNotificationService()


	// --- Init Repositories ---
	userRepo := repository.NewUserRepository(db)

	// --- Init Usecases ---
	userUC := usecase.NewUserUsecase(userRepo)

	// --- Init WebSocket Hub ---
	hub := handler.NewHub()
	go hub.Run() // Start hub in background goroutine
	log. Println("[WebSocket] Hub started")

	combinedHandler := transaction.NewCombinedEventHandler(userRepo, hub, logger)

	// ✅ --- Start Transaction Event Subscriber (your existing one) ---
	transactionSub := subscriber.NewTransactionEventSubscriber(rdb, hub)
	go func() {
		if err := transactionSub.Start(ctx); err != nil {
			log. Fatalf("transaction subscriber failed: %v", err)
		}
	}()
	log.Println("[TransactionSubscriber] ✅ Started")

	// ✅ --- Start Deposit Event Subscriber (new one for partner events) ---
	depositSub := subscriber.NewEventSubscriber(rdb, logger, combinedHandler)
	go func() {
		if err := depositSub.Start(ctx); err != nil {
			log.Fatalf("deposit subscriber failed: %v", err)
		}
	}()
	log.Println("[DepositSubscriber] ✅ Started - listening to partner:deposit:completed, partner:deposit:failed")

	// --- Init Middleware ---
	auth := middleware.RequireAuth()

	// --- Init Handlers ---
	paymentHandler := handler.NewPaymentHandler(
		partnerSvc,
		accountingClient,
		notificationCli,
		userUC,
		hub,
		otpSvc,
		accountClient,
		cryptoClient,
		rdb,
		logger,
		profileFetcher,
	)

	// --- Router ---
	r := chi. NewRouter()
	r = router.SetupRoutes(r, paymentHandler, auth, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg. HTTPAddr,
		Handler: r,
	}
}
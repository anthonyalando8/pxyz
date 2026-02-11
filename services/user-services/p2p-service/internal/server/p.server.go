// internal/server/server.go
package server

import (
	"context"
	"log"
	"net/http"

	"p2p-service/internal/config"
	rh "p2p-service/internal/handler/rest"
	wsh "p2p-service/internal/handler/websocket"
	"p2p-service/internal/repository"
	"p2p-service/internal/router"
	"p2p-service/internal/usecase"
	"x/shared/auth/middleware"

	// accountclient "x/shared/account"
	// authclient "x/shared/auth"
	// otpclient "x/shared/auth/otp"
	// accountingclient "x/shared/common/accounting"
	// cryptoclient "x/shared/common/crypto"
	// notificationclient "x/shared/notification"
	// "x/shared/utils/id"
	// helpers "x/shared/utils/profile"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewServer(cfg config.AppConfig) *http.Server {
	// --- Init Logger ---
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// --- Connect Postgres ---
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect DB: %v", err)
	}

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
	log.Println("[Redis] Connected successfully")

	// // --- Init ID generator ---
	// sf, err := id.NewSnowflake(9)
	// if err != nil {
	// 	log.Fatalf("failed to init snowflake: %v", err)
	// }
	// _ = sf

	// authClient, err := authclient.DialAuthService(authclient.AllAuthServices)
	// if err != nil {
	// 	log.Fatalf("failed to dial auth service: %v", err)
	// }
	// profileFetcher := helpers.NewProfileFetcher(authClient)

	// otpSvc := otpclient.NewOTPService()
	// accountClient := accountclient.NewAccountClient()

	// cryptoClient := cryptoclient.NewCryptoClientOrNil()
	// if cryptoClient == nil {
	// 	log.Println("⚠️  Crypto service unavailable - wallets will not be created")
	// }

	// // --- Init gRPC Clients ---
	// accountingClient := accountingclient.NewAccountingClient()
	// notificationCli := notificationclient.NewNotificationService()


	

	// --- Init P2P Repositories ---
	p2pProfileRepo := repository.NewP2PProfileRepository(db, logger)

	// --- Init P2P Usecases ---
	p2pProfileUsecase := usecase.NewP2PProfileUsecase(p2pProfileRepo, logger)

	// --- Init Middleware ---
	auth := middleware.RequireAuth()

	// --- Init P2P Handlers ---
	p2pRestHandler := rh.NewP2PRestHandler(p2pProfileUsecase, logger)
	p2pWSHandler := wsh.NewP2PWebSocketHandler(p2pProfileUsecase, logger)

	log.Println("[P2P] Handlers initialized")

	// --- Router ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, p2pRestHandler, p2pWSHandler, auth, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}
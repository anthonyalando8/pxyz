package server

import (
	"log"
	"net/http"

	"admin-service/internal/config"
	"admin-service/internal/handler"
	"admin-service/internal/router"

	partnerclient "x/shared/partner"
	coreclient "x/shared/core"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	accountingclient "x/shared/common/accounting"

	"x/shared/auth/middleware"
	"go.uber.org/zap"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func NewServer(cfg config.AppConfig) *http.Server {
	// --- Connect Postgres (if needed later) ---
	db, err := config.ConnectDB()
	_ = db
	if err != nil {
		log.Fatalf("failed to connect DB: %v", err)
	}

	// --- Init Redis ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// --- Init Clients ---
	coreSvc := coreclient.NewCoreService()
	emailSvc := emailclient.NewEmailClient()
	smsSvc := smsclient.NewSMSClient()
	partnerSvc := partnerclient.NewPartnerService()
	accountingClient := accountingclient.NewAccountingClient()

	// --- Init Middleware ---
	auth := middleware.RequireAuth()

	// --- Handlers ---
	adminHandler := handler.NewAdminHandler(
		auth,
		nil,        // otp client if/when you wire it up
		emailSvc,
		smsSvc,
		rdb,
		coreSvc,
		partnerSvc,
		accountingClient,
		logger,
	)

	// --- Router ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, adminHandler, auth, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}

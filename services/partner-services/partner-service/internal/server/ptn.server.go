package server

import (
	"log"
	"net"
	"net/http"

	"partner-service/internal/config"
	"partner-service/internal/handler"
	"partner-service/internal/repository"
	"partner-service/internal/router"
	"partner-service/internal/usecase"
	"x/shared/auth/middleware"
	partnerMiddleware "partner-service/pkg/auth"
	otp "x/shared/auth/otp"
	email "x/shared/email"
	sms "x/shared/sms"
	"x/shared/utils/id"
	authclient "x/shared/auth"
	accountingclient "x/shared/common/accounting" //
	"partner-service/internal/events"


	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
		"go.uber.org/zap"


	partnersvcpb "x/shared/genproto/partner/svcpb"
)

type Server struct {
	HTTP *http.Server
	GRPC *grpc.Server
}

func NewServer(cfg config.AppConfig) *Server {
	// --- DB connection ---
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	// --- Repositories ---
	partnerRepo := repository.NewPartnerRepo(db)
	partnerUserRepo := repository.NewPartnerUserRepo(db)

	// --- Snowflake for ID generation ---
	sf, err := id.NewSnowflake(11)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	// --- Redis client ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	// --- Event Publisher ---
	eventPublisher := events.NewEventPublisher(rdb, logger)
	// --- Usecase ---
	partnerUC := usecase.NewPartnerUsecase(partnerRepo, partnerUserRepo, sf, logger, )
	apiKeyAuth := partnerMiddleware.NewAPIKeyAuthMiddleware(partnerRepo, logger)

	// --- Middleware and service clients ---
	authMiddleware := middleware.RequireAuth()
	otpSvc := otp.NewOTPService()
	emailCli := email.NewEmailClient()
	smsCli := sms.NewSMSClient()

	// --- Auth-service client ---
	authClient, err := authclient.DialAuthService(authclient.PartnerAuthService)
	if err != nil {
		log.Fatalf("failed to dial auth service: %v", err)
	}

	accountingClient := accountingclient.NewAccountingClient()


	// --- Handlers ---
	partnerHandler := handler.NewPartnerHandler(
		partnerUC,
		authClient,
		otpSvc,
		emailCli,
		smsCli,
		accountingClient,
		logger,
		eventPublisher,
	)

	grpcPartnerHandler := handler.NewGRPCPartnerHandler(partnerUC,
		authClient,
		otpSvc,
		emailCli,
		smsCli,
		accountingClient,
	)
		

	// --- HTTP router ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, partnerHandler, authMiddleware,apiKeyAuth, rdb).(*chi.Mux)

	// --- HTTP server ---
	httpSrv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}

	// --- gRPC server ---
	grpcSrv := grpc.NewServer()
	partnersvcpb.RegisterPartnerServiceServer(grpcSrv, grpcPartnerHandler)

	// enable reflection for testing (grpcurl / evans)
	reflection.Register(grpcSrv)

	return &Server{
		HTTP: httpSrv,
		GRPC: grpcSrv,
	}
}

// StartGRPC runs the gRPC server on cfg.GRPCAddr
func (s *Server) StartGRPC(grpcAddr string) error {
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}
	log.Printf("🚀 Partner gRPC service listening on %s", grpcAddr)
	return s.GRPC.Serve(lis)
}

// StartHTTP runs the HTTP server
func (s *Server) StartHTTP() error {
	log.Printf("🚀 Partner HTTP service listening on %s", s.HTTP.Addr)
	return s.HTTP.ListenAndServe()
}

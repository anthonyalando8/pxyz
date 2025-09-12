package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"core-service/internal/config"
	hrest "core-service/internal/handler/rest"
	hgrpc "core-service/internal/handler/grpc"
	"core-service/internal/repository"
	"core-service/internal/router"
	"core-service/internal/service"
	"core-service/internal/usecase"
	corepb "x/shared/genproto/corepb"
	"x/shared/auth/middleware"
	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewServer(cfg config.AppConfig) *http.Server {
	// --- DB connection ---
	dbpool, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	// don’t defer dbpool.Close() here → will close before server starts
	// instead close in main.go during shutdown

	// --- Init repos & usecases ---
	countryRepo := repository.NewCountryRepo(dbpool)
	sf, err := id.NewSnowflake(2) // Node ID for core-service
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	auth := middleware.RequireAuth()

	countryUC := usecase.NewCountryUsecase(countryRepo, sf)
	countryHandler := hrest.NewCountryHandler(countryUC)
	countryGRPCHandler := hgrpc.NewCountryGRPCHandler(countryUC)

	// --- Auto sync countries on startup ---
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		sync := service.NewCountrySync(countryRepo)
		if err := sync.Sync(ctx); err != nil {
			log.Printf("⚠️  failed to sync countries on startup: %v", err)
		} else {
			log.Println("✅ countries synced successfully on startup")
		}
	}()

	// --- HTTP routes ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, countryHandler, auth, rdb).(*chi.Mux)

	// --- gRPC server ---
	grpcServer := grpc.NewServer()
	corepb.RegisterCoreServiceServer(grpcServer, countryGRPCHandler) // register handler
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
		}
		log.Printf("gRPC server listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// --- HTTP server ---
	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}

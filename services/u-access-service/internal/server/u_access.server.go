package server

import (
	"log"
	"net"
	"net/http"

	"u-rbac-service/internal/config"
	hgrpc "u-rbac-service/internal/handler/grpc"
	hrest "u-rbac-service/internal/handler/rest"
	"u-rbac-service/internal/repository"
	"u-rbac-service/internal/router"
	"u-rbac-service/internal/usecase"
	"x/shared/auth/middleware"
	rbacpb "x/shared/genproto/urbacpb"
	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// NewServer builds and returns the HTTP server
func NewServer(cfg config.AppConfig) *http.Server {
	// --- DB connection ---
	dbpool, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// --- Redis + Auth ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	auth := middleware.RequireAuth()

	// --- Init repos & usecases ---
	rbacRepo := repository.NewRBACRepo(dbpool)
	sf, err := id.NewSnowflake(11) // node ID for RBAC service
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	moduleUC := usecase.NewModuleUsecase(rbacRepo, sf)
	moduleHandler := hrest.NewModuleHandler(moduleUC)
	moduleGRPCHandler := hgrpc.NewModuleGRPCHandler(moduleUC)

	// --- HTTP routes ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, moduleHandler, auth, rdb).(*chi.Mux)

	// --- gRPC server ---
	grpcServer := grpc.NewServer()
	rbacpb.RegisterRBACServiceServer(grpcServer, moduleGRPCHandler)
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
		}
		log.Printf("RBAC gRPC server listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("RBAC gRPC server failed: %v", err)
		}
	}()

	// --- HTTP server ---
	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}

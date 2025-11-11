package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"ptn-rbac-service/internal/config"
	hgrpc "ptn-rbac-service/internal/handler/grpc"
	hrest "ptn-rbac-service/internal/handler/rest"
	"ptn-rbac-service/internal/repository"
	"ptn-rbac-service/internal/router"
	"ptn-rbac-service/internal/service"
	"ptn-rbac-service/internal/usecase"
	"x/shared/auth/middleware"
	rbacpb "x/shared/genproto/partner/ptnrbacpb"
	"x/shared/utils/id"

	"x/shared/utils/cache"

	"github.com/go-chi/chi/v5"

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
	cache := cache.NewCache([]string{cfg.RedisAddr}, cfg.RedisPass, false)

	auth := middleware.RequireAuth()

	// --- Init repos & usecases ---
	rbacRepo := repository.NewRBACRepo(dbpool)
	sf, err := id.NewSnowflake(13) // node ID for RBAC service
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		sync := service.NewRBACSeedService(rbacRepo)
		if err := sync.SeedDefaults(ctx); err != nil {
			log.Printf("⚠️  failed to sync data on startup: %v", err)
		} else {
			log.Println("✅ data synced successfully on startup")
		}
	}()

	moduleUC := usecase.NewRBACUsecase(rbacRepo, sf, cache)
	moduleHandler := hrest.NewModuleHandler(moduleUC)
	moduleGRPCHandler := hgrpc.NewRBACGRPCHandler(moduleUC)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		err := moduleUC.BatchAssignRolesToUnassignedUsers(ctx, 0)
		if err != nil {
			log.Printf("⚠️  batch role assignment failed: %v", err)
		} else {
			log.Println("✅ batch role assignment completed successfully")
		}
	}()

	// --- HTTP routes ---
	r := chi.NewRouter()
	r = router.SetupRoutes(r, moduleHandler, auth, cache).(*chi.Mux)

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

package main

import (
	"admin-session-service/internal/config"
	"admin-session-service/internal/handler"
	"admin-session-service/internal/repository"
	"admin-session-service/internal/usecase"
	"admin-session-service/pkg/jwtutil"
	"log"
	"net"
	"os"
	"os/signal"
	authclient "x/shared/auth"
	pb "x/shared/genproto/admin/sessionpb"
	urbacservice "x/shared/factory/admin/urbac/utils"
	"x/shared/utils/cache"
	urbac "x/shared/factory/admin/urbac"

	"syscall"
	"x/shared/utils/id"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	// Load environment variables
	cfg := config.Load()

	// Connect to the database
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	cache := cache.NewCache([]string{cfg.RedisAddr}, cfg.RedisPass, false)

	// Initialize Snowflake ID generator
	sf, err := id.NewSnowflake(3)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	jwtGen := jwtutil.LoadAndBuild(cfg.JWT)

	authClient, err := authclient.DialAuthService(authclient.AdminAuthService)
	if err != nil {
        log.Fatalf("failed to dial auth service: %v", err)
    }
	
	urbacCli := urbac.NewRBACService()
	urbacSvc :=	urbacservice.NewService(urbacCli.Client, rdb)
	// Initialize session repository and gRPC handler
	sessionRepo := repository.NewSessionRepository(db)
	sessionUC := usecase.NewSessionUsecase(sessionRepo, sf, jwtGen, authClient, urbacSvc, cache,)
	authHandler := handler.NewAuthHandler(sessionUC)

	// Create a gRPC server
	grpcServer := grpc.NewServer()

	// Register the gRPC service
	pb.RegisterAdminSessionServiceServer(grpcServer, authHandler)

	// Start listening on configured address
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
	}

	// Start gRPC server in a goroutine
	go func() {
		log.Printf("Session gRPC server started at %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// Graceful shutdown on interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down gRPC server...")
	grpcServer.GracefulStop()
	log.Println("Session service stopped")
}

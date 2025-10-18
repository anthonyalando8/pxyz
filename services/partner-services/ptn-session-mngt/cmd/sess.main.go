package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"ptn-session-service/internal/config"
	"ptn-session-service/internal/handler"
	"ptn-session-service/internal/repository"
	"ptn-session-service/internal/usecase"
	"ptn-session-service/pkg/jwtutil"
	authclient "x/shared/auth"
	pb "x/shared/genproto/partner/sessionpb"

	"syscall"
	"x/shared/utils/id"

	//"github.com/redis/go-redis/v9"
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
	// rdb := redis.NewClient(&redis.Options{
	// 	Addr:     cfg.RedisAddr,
	// 	Password: cfg.RedisPass,
	// 	DB:       0,
	// })
	// Initialize Snowflake ID generator
	sf, err := id.NewSnowflake(6)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	jwtGen := jwtutil.LoadAndBuild(cfg.JWT)

	authClient, err := authclient.DialAuthService(authclient.PartnerAuthService)
	if err != nil {
        log.Fatalf("failed to dial auth service: %v", err)
    }
    defer authClient.Close()

	// Initialize session repository and gRPC handler
	sessionRepo := repository.NewSessionRepository(db)
	sessionUC := usecase.NewSessionUsecase(sessionRepo, sf, jwtGen, authClient)
	authHandler := handler.NewAuthHandler(sessionUC)

	// Create a gRPC server
	grpcServer := grpc.NewServer()

	// Register the gRPC service
	pb.RegisterPartnerSessionServiceServer(grpcServer, authHandler)

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

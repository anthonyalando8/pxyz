package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"session-service/internal/config"
	"session-service/internal/repository"
	"session-service/internal/handler"
	"session-service/internal/usecase"
	pb "x/shared/genproto/sessionpb"
	"x/shared/utils/id"
	"syscall"

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
	// Initialize Snowflake ID generator
	sf, err := id.NewSnowflake(1)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	// Initialize session repository and gRPC handler
	sessionRepo := repository.NewSessionRepository(db)
	sessionUC := usecase.NewSessionUsecase(sessionRepo, sf)
	authHandler := handler.NewAuthHandler(sessionUC)

	// Create a gRPC server
	grpcServer := grpc.NewServer()

	// Register the gRPC service
	pb.RegisterAuthServiceServer(grpcServer, authHandler)

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

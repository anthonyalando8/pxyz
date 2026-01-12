package main

import (
	"fmt"
	"log"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"email-service/internal/config"
	"email-service/internal/handler"
	"email-service/internal/repository"
	"email-service/internal/service"
	pb "x/shared/genproto/emailpb"
	"x/shared/utils/id"
)

func main() {
	// Load config
	cfg := config.Load()

	// Connect to DB
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Init dependencies
	emailRepo := repository.NewEmailLogRepo(db)
	emailSvc := service.NewEmailSender(service.EmailConfig{
		SMTPHost:   cfg.SMTPHost,
		SMTPPort:   cfg.SMTPPort,
		Username:   cfg.SMTPUser,
		Password:   cfg.SMTPPass,
		FromName:   cfg.FromName,
		ReplyTo:    cfg.ReplyTo,
		DomainName: cfg.DomainName,
	})
	logger, err := zap.NewProduction()
    if err != nil {
        panic(fmt.Sprintf("failed to initialize logger: %v", err))
    }
    defer logger. Sync()

	// snowflake
	sf, err := id.NewSnowflake(4)
	if err != nil { log.Fatalf("sf: %v", err) }
	// Init handler
	emailHandler := handler.NewEmailHandler(emailSvc, emailRepo, sf, logger)

	// Start gRPC server
	listener, err := net.Listen("tcp",cfg.GRPCPort)
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterEmailServiceServer(grpcServer, emailHandler)

	log.Printf("Email service running on port %s", cfg.GRPCPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

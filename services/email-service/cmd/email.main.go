package main

import (
	"log"
	"net"

	"google.golang.org/grpc"

	pb "x/shared/genproto/emailpb"
	"email-service/internal/handler"
	"email-service/internal/repository"
	"email-service/internal/service"
	"email-service/internal/config"
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
	emailSvc := service.NewEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)

	// Init handler
	emailHandler := handler.NewEmailHandler(emailSvc, emailRepo)

	// Start gRPC server
	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
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

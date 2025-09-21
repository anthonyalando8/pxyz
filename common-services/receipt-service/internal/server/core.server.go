package server

import (
	//"context"
	"log"
	"net"

	"receipt-service/internal/config"
	hgrpc "receipt-service/internal/handler/grpc"
	"receipt-service/internal/repository"
	"receipt-service/internal/usecase"
	"receipt-service/pkg/generator"
	receiptpb "x/shared/genproto/shared/accounting/receiptpb"
	notificationclient "x/shared/notification" // ✅ added


	//"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// NewReceiptGRPCServer starts the gRPC server for receipt service
func NewReceiptGRPCServer(cfg config.AppConfig) {
	// --- DB connection ---
	dbpool, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	defer dbpool.Close()

	// --- Redis client ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	
	_ = rdb
	// --- Repositories ---
	receiptRepo := repository.NewReceiptRepo(dbpool)

	// --- Generator for receipt codes ---
	codeGen := generator.NewGenerator() // configure length etc in constructor

	// --- Notification client ---
	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client


	// --- Usecases ---
	receiptUC := usecase.NewReceiptUsecase(receiptRepo, codeGen, notificationCli)

	// --- gRPC Handler ---
	receiptHandler := hgrpc.NewReceiptGRPCHandler(receiptUC)

	// --- gRPC Server ---
	grpcServer := grpc.NewServer()
	receiptpb.RegisterReceiptServiceServer(grpcServer, receiptHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
	}

	log.Printf("Receipt gRPC server listening on %s", cfg.GRPCAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

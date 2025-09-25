package server

import (
	"context"
	"log"
	"net"

	"receipt-service/internal/config"
	hgrpc "receipt-service/internal/handler/grpc"
	"receipt-service/internal/repository"
	"receipt-service/internal/usecase"
	"receipt-service/pkg/generator"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v2"
	notificationclient "x/shared/notification"

	service "receipt-service/internal/service"

	"github.com/segmentio/kafka-go"
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
	_ = rdb // keep for caching later

	// --- Repositories ---
	receiptRepo := repository.NewReceiptRepo(dbpool)

	// --- Generator for receipt codes ---
	codeGen := generator.NewGenerator()

	// --- Notification client ---
	notificationCli := notificationclient.NewNotificationService()

	// --- Kafka writer ---
	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.KafkaBrokers...), // slice of brokers from config
		Topic:    "receipts",
		Balancer: &kafka.LeastBytes{},
	}

	// --- Usecases ---
	receiptUC := usecase.NewReceiptUsecase(receiptRepo, codeGen, notificationCli, writer)

	// --- gRPC Handler ---
	receiptHandler := hgrpc.NewReceiptGRPCHandler(receiptUC)

	// --- Start Kafka worker in background ---
	go func() {
		log.Println("starting receipt worker...")
		service.StartReceiptWorker(context.Background(), cfg.KafkaBrokers, receiptRepo)
	}()

	// --- gRPC Server ---
	grpcServer := grpc.NewServer()
	receiptpb.RegisterReceiptServiceV2Server(grpcServer, receiptHandler)
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

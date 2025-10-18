package server

import (
	"log"
	"net"
	"time"
	"context"

	"accounting-service/internal/config"
	hgrpc "accounting-service/internal/handler/grpc"
	"accounting-service/internal/repository"
	"accounting-service/internal/usecase"
	"accounting-service/internal/service"
	accountingpb "x/shared/genproto/shared/accounting/accountingpb"
	authclient "x/shared/auth"
	receiptclient "x/shared/common/receipt"
	partnerclient "x/shared/partner"

	"x/shared/utils/id"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewAccountingGRPCServer(cfg config.AppConfig) {
	// --- DB connection ---
	dbpool, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sf, err := id.NewSnowflake(15)
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	// --- Redis client ---
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	// --- External service clients ---
	authClient, err := authclient.DialAuthService(authclient.AllAuthServices)
	if err != nil {
		log.Fatalf("failed to dial auth service: %v", err)
	}
	receiptCli := receiptclient.NewReceiptClientV2()
	partnerSvc := partnerclient.NewPartnerService()

	// --- Repositories ---
	accountRepo := repository.NewAccountRepo(dbpool)
	journalRepo := repository.NewJournalRepo(dbpool)
	postingRepo := repository.NewPostingRepo(dbpool)
	balanceRepo := repository.NewBalanceRepo(dbpool)
	ledgerRepo := repository.NewLedgerRepo(dbpool, accountRepo, journalRepo, postingRepo, balanceRepo)
	statementRepo := repository.NewStatementRepo(dbpool, postingRepo)
	currencyRepo := repository.NewCurrencyRepo(dbpool) // for FXService
	feeRepo := repository.NewTransactionFeeRepo(dbpool)
	ruleRepo := repository.NewTransactionFeeRuleRepo(dbpool)

	// --- Usecases ---
	accountUC := usecase.NewAccountUsecase(accountRepo, rdb)
	ruleUC := usecase.NewTransactionFeeRuleUsecase(ruleRepo, rdb, 30*time.Minute)

	ledgerUC := usecase.NewLedgerUsecase(ledgerRepo, balanceRepo, sf, authClient, receiptCli, partnerSvc, rdb, accountUC, ruleUC,)
	statementUC := usecase.NewStatementUsecase(statementRepo, rdb)
	feeUC := usecase.NewTransactionFeeUsecase(feeRepo, rdb)
	_ = feeUC

	// --- Services ---
	fxService := service.NewFXService(currencyRepo)
	systemSeeder := service.NewSystemSeeder(
		fxService,
		accountUC,
		ruleUC,
		dbpool,
	)

	// --- Seed system in a goroutine (non-blocking) ---
	go func() {
		if err := systemSeeder.SeedSystem(context.Background()); err != nil {
			log.Printf("⚠️  System seeding failed: %v", err)
		} else {
			log.Println("✅ System seeding completed successfully")
		}
	}()

	// --- gRPC Handler ---
	accountingHandler := hgrpc.NewAccountingGRPCHandler(accountUC, ledgerUC, statementUC, rdb)

	// --- gRPC Server ---
	grpcServer := grpc.NewServer()
	accountingpb.RegisterAccountingServiceServer(grpcServer, accountingHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.GRPCAddr, err)
	}

	log.Printf("Accounting gRPC server listening on %s", cfg.GRPCAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

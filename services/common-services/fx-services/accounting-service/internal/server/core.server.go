package server

import (
	"context"
	"log"
	"net"
	"time"

	"accounting-service/internal/config"
	hgrpc "accounting-service/internal/handler/grpc"
	"accounting-service/internal/repository"
	"accounting-service/internal/service"
	"accounting-service/internal/usecase"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	"accounting-service/internal/pub"
	authclient "x/shared/auth"
	receiptclient "x/shared/common/receipt"
	notificationclient "x/shared/notification"
	partnerclient "x/shared/partner"
	feecalculator "accounting-service/internal/pkg"

	"x/shared/utils/id"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"go.uber.org/zap"
)

// NewAccountingGRPCServer initializes and starts the accounting gRPC server
// with all required dependencies
func NewAccountingGRPCServer(cfg config.AppConfig) {
	ctx := context.Background()
	
	// ===============================
	// DATABASE CONNECTION
	// ===============================
	dbpool, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer dbpool.Close()
	log.Println("✅ Database connected successfully")
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// ===============================
	// SNOWFLAKE ID GENERATOR
	// ===============================
	// Used for generating unique IDs for journals, ledgers, etc.
	// Node ID should be unique per instance (0-1023)
	sf, err := id.NewSnowflake(15) // Use your node ID
	if err != nil {
		log.Fatalf("❌ Failed to initialize snowflake: %v", err)
	}
	log.Println("✅ Snowflake ID generator initialized")

	// ===============================
	// REDIS CLIENT
	// ===============================
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPass,
		DB:           0,
		PoolSize:     100,              // Max connections
		MinIdleConns: 10,               // Min idle connections
		MaxRetries:   3,                // Retry attempts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	defer rdb.Close()

	// Test Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Failed to connect to redis: %v", err)
	}
	log.Println("✅ Redis connected successfully")

	// ===============================
	// KAFKA WRITER (Event Streaming)
	// ===============================
	// Used for publishing transaction events
	kafkaWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Topic:        "accounting.transactions",
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        true, // Async for better performance
		BatchSize:    100,  // Messages per batch
		BatchTimeout: 10 * time.Millisecond,
	}
	defer kafkaWriter.Close()
	log.Println("✅ Kafka writer initialized")

	// ===============================
	// EXTERNAL SERVICE CLIENTS
	// ===============================
	
	// Auth Service - For user/agent verification
	authClient, err := authclient.DialAuthService(authclient.AllAuthServices)
	if err != nil {
		log.Fatalf("❌ Failed to dial auth service: %v", err)
	}
	log.Println("✅ Auth service client connected")

	// Receipt Service - For generating transaction receipts
	receiptCli := receiptclient.NewReceiptClientV3()
	log.Println("✅ Receipt service client initialized")

	// Notification Service - For sending transaction notifications
	notificationCli := notificationclient.NewNotificationService()
	log.Println("✅ Notification service client initialized")

	// Partner Service - For partner-specific operations
	partnerSvc := partnerclient.NewPartnerService()
	log.Println("✅ Partner service client initialized")

	// ===============================
	// REPOSITORIES (Data Layer)
	// ===============================
	// Initialize all repositories for database access
	
	accountRepo := repository.NewAccountRepo(dbpool)
	journalRepo := repository.NewJournalRepo(dbpool)
	ledgerRepo := repository.NewLedgerRepo(dbpool)
	balanceRepo := repository.NewBalanceRepo(dbpool)
	currencyRepo := repository.NewCurrencyRepo(dbpool)
	feeRepo := repository.NewTransactionFeeRepo(dbpool)
	feeRuleRepo := repository.NewTransactionFeeRuleRepo(dbpool)
	statementRepo := repository.NewStatementRepo(dbpool, ledgerRepo)
	agentRepo := repository.NewAgentRepository(dbpool)
	_ = currencyRepo  // Currently unused, but initialized for completeness

	transactionRepo := repository.NewTransactionRepo(dbpool, accountRepo, journalRepo, ledgerRepo, balanceRepo, currencyRepo, feeRepo, agentRepo, logger)

	log.Println("✅ All repositories initialized")

	// ===============================
	// USECASES (Business Logic Layer)
	// ===============================
	// Initialize in dependency order to avoid nil references
	feeCal := feecalculator.NewTransactionFeeCalculator(
		feeRepo,
		feeRuleRepo,
		rdb,
	)
	log.Println("✅ Fee calculator initialized")
	
	// 1. Account Usecase - No dependencies on other usecases
	accountUC := usecase.NewAccountUsecase(
		accountRepo,  // Repository for account operations
		balanceRepo,
		sf,		  // Snowflake for ID generation
		rdb,          // Redis for caching
	)
	log.Println("✅ Account usecase initialized")
	
	// 2. Fee Rule Usecase - No dependencies on other usecases
	feeRuleUC := usecase.NewTransactionFeeRuleUsecase(
		feeRuleRepo,  // Repository for fee rules
		//balanceRepo,
		rdb,          // Redis for caching
	)
	log.Println("✅ Fee rule usecase initialized")
	
	// 3. Fee Usecase - Depends on fee rule usecase
	feeUC := usecase.NewTransactionFeeUsecase(
		feeRepo,      // Repository for fee records
		feeRuleRepo,  // Repository for fee rules
		rdb,          // Redis for caching
		feeCal,
	)
	log.Println("✅ Fee usecase initialized")
	
	// 4. Journal Usecase - For transaction journals
	journalUC := usecase.NewJournalUsecase(
		journalRepo,  // Repository for journals
		ledgerRepo,   // Repository for ledgers
		rdb,          // Redis for caching
	)
	log.Println("✅ Journal usecase initialized")
	
	// 5. Ledger Usecase - For ledger entries
	ledgerUC := usecase.NewLedgerUsecase(
		ledgerRepo,   // Repository for ledgers
		accountRepo,  // Repository for accounts
		rdb,          // Redis for caching
	)
	log.Println("✅ Ledger usecase initialized")
	
	// 6. Statement Usecase - For generating statements
	statementUC := usecase.NewStatementUsecase(
		statementRepo,  // Repository for statement queries
		accountRepo,    // Repository for account data
		balanceRepo,    // Repository for balance data
		rdb,            // Redis for caching
	)
	log.Println("✅ Statement usecase initialized")

	agentUc := usecase.NewAgentUsecase(
		agentRepo,
		accountUC,
		sf,	
	)
	log.Println("✅ Agent usecase initialized")
	
	// 7. Transaction Usecase - Main transaction processing
	// This is the most complex usecase with many dependencies
	pub := publisher.NewTransactionEventPublisher(rdb)
	transactionUC := usecase.NewTransactionUsecase(
		transactionRepo,    // Repository for transactions (or journalRepo)
		accountRepo,        // Repository for account operations
		balanceRepo,        // Repository for balance updates
		journalRepo,        // Repository for journal entries
		ledgerRepo,         // Repository for ledger entries
		feeRepo,            // Repository for fee records
		accountUC,          // Account usecase for account operations
		feeUC,              // Fee usecase for fee calculations
		feeRuleUC,          // Fee rule usecase for fee rules
		authClient,         // Auth service for user verification
		receiptCli,         // Receipt service for receipt generation
		notificationCli,    // Notification service for notifications
		partnerSvc,         // Partner service for partner operations
		rdb,                // Redis for caching and status tracking
		kafkaWriter,        // Kafka for event streaming
		pub,
		feeCal,
		//sf,                 // Snowflake for ID generation
	)
	log.Println("✅ Transaction usecase initialized")

	log.Println("✅ All 7 usecases initialized successfully")

	// ===============================
	// SYSTEM SEEDER (Optional)
	// ===============================
	// Seed system accounts, user accounts, and partner accounts
	// Set cfg.SeedOnStartup = true in config to enable
	if cfg.SeedOnStartup {
		log.Println("")
		log.Println("🌱 Starting system seeding...")
		log.Println("════════════════════════════════════════════════════════════")
		
		seeder := service.NewSystemSeeder(
			accountUC,
			authClient,
			partnerSvc,
			dbpool,
		)

		// Step 1: Seed system accounts (run first, always)
		log.Println("Step 1/3: Creating system accounts...")
		if err := seeder.SeedSystemAccounts(ctx); err != nil {
			log.Printf("⚠️  Warning: System account seeding failed (may already exist): %v", err)
			// Don't fatal - accounts might already exist
		} else {
			log.Println("✅ System accounts seeded successfully")
		}

		// Step 2: Seed user and partner accounts
		log.Println("Step 2/3: Seeding user and partner accounts...")
		if err := seeder.SeedSystem(ctx); err != nil {
			log.Printf("⚠️  Warning: User/partner seeding failed: %v", err)
			// Don't fatal - continue with server startup
		} else {
			log.Println("✅ User and partner accounts seeded successfully")
		}

		// Step 3 (Optional): Seed agent accounts
		if cfg.SeedAgents {
			log.Println("Step 3/3: Seeding agent accounts...")
			if err := seeder.SeedAgentAccounts(ctx); err != nil {
				log.Printf("⚠️  Warning: Agent seeding failed: %v", err)
			} else {
				log.Println("✅ Agent accounts seeded successfully")
			}
		}

		log.Println("════════════════════════════════════════════════════════════")
		log.Println("✅ System seeding completed!")
		log.Println("")
	} else {
		log.Println("ℹ️  System seeding skipped (cfg.SeedOnStartup = false)")
	}

	// ===============================
	// GRPC HANDLER
	// ===============================
	// Initialize the gRPC handler with all usecases
	accountingHandler := hgrpc.NewAccountingHandler(
		accountUC,        // Account management (8 RPCs)
		transactionUC,    // Transaction execution (5 RPCs)
		statementUC,      // Statements & reports (6 RPCs)
		journalUC,        // Journal queries (4 RPCs)
		ledgerUC,         // Ledger queries (4 RPCs)
		feeUC,            // Fee management (3 RPCs)
		feeRuleUC,        // Fee rule management (covered in fee RPCs)
		agentUc,
		rdb,              // Redis for health checks
	)

	log.Println("✅ gRPC handler initialized with 28 RPCs")

	// ===============================
	// GRPC SERVER OPTIONS
	// ===============================
	grpcOpts := []grpc.ServerOption{
		// Message size limits
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB max receive
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB max send
		
		// Connection settings
		grpc.ConnectionTimeout(30 * time.Second),
		
		// Concurrency
		grpc.NumStreamWorkers(32), // Number of workers for streaming
	}

	// ===============================
	// GRPC SERVER
	// ===============================
	grpcServer := grpc.NewServer(grpcOpts...)
	
	// Register accounting service
	accountingpb.RegisterAccountingServiceServer(grpcServer, accountingHandler)
	
	// Enable reflection for grpcurl/grpcui testing
	reflection.Register(grpcServer)

	// ===============================
	// START SERVER
	// ===============================
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("❌ Failed to listen on %s: %v", cfg.GRPCAddr, err)
	}

	// Pretty print startup information
	log.Println("")
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║         Accounting Service - gRPC Server Started          ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")
	log.Printf("🚀 Server listening on: %s", cfg.GRPCAddr)
	log.Println("")
	log.Println("📡 Available RPCs (28 total):")
	log.Println("   ├─ Account Management (8 RPCs)")
	log.Println("   │  ├─ CreateAccount")
	log.Println("   │  ├─ CreateAccounts")
	log.Println("   │  ├─ GetAccount")
	log.Println("   │  ├─ GetAccountsByOwner")
	log.Println("   │  ├─ GetOrCreateUserAccounts")
	log.Println("   │  ├─ UpdateAccount")
	log.Println("   │  ├─ GetBalance")
	log.Println("   │  └─ BatchGetBalances")
	log.Println("   ├─ Transaction Execution (5 RPCs)")
	log.Println("   │  ├─ ExecuteTransaction")
	log.Println("   │  ├─ ExecuteTransactionSync")
	log.Println("   │  ├─ BatchExecuteTransactions")
	log.Println("   │  ├─ GetTransactionStatus")
	log.Println("   │  └─ GetTransactionByReceipt")
	log.Println("   ├─ Journal & Ledger (4 RPCs)")
	log.Println("   │  ├─ GetJournal")
	log.Println("   │  ├─ ListJournals")
	log.Println("   │  ├─ ListLedgersByJournal")
	log.Println("   │  └─ ListLedgersByAccount")
	log.Println("   ├─ Statements & Reports (6 RPCs)")
	log.Println("   │  ├─ GetAccountStatement")
	log.Println("   │  ├─ GetOwnerStatement")
	log.Println("   │  ├─ GetOwnerSummary")
	log.Println("   │  ├─ GenerateDailyReport")
	log.Println("   │  ├─ GetTransactionSummary")
	log.Println("   │  └─ GetSystemHoldings")
	log.Println("   ├─ Fee Management (3 RPCs)")
	log.Println("   │  ├─ CalculateFee")
	log.Println("   │  ├─ GetFeesByReceipt")
	log.Println("   │  └─ GetAgentCommissionSummary")
	log.Println("   └─ Monitoring (2 RPCs)")
	log.Println("      ├─ HealthCheck")
	log.Println("      └─ StreamTransactionEvents")
	log.Println("")
	log.Println("💡 Quick Test Commands:")
	log.Println("   Health Check:")
	log.Printf("   $ grpcurl -plaintext %s accounting.v1.AccountingService/HealthCheck\n", cfg.GRPCAddr)
	log.Println("")
	log.Println("   List Services:")
	log.Printf("   $ grpcurl -plaintext %s list\n", cfg.GRPCAddr)
	log.Println("")
	log.Println("   List Methods:")
	log.Printf("   $ grpcurl -plaintext %s list accounting.v1.AccountingService\n", cfg.GRPCAddr)
	log.Println("")
	log.Println("📊 Performance Targets:")
	log.Println("   ├─ Async Transactions: 4000+ req/sec")
	log.Println("   ├─ Sync Transactions:  1500+ req/sec")
	log.Println("   ├─ Balance Queries:    10,000+ req/sec")
	log.Println("   └─ Statement Queries:  2000+ req/sec")
	log.Println("")
	log.Println("✨ Ready to accept connections!")
	log.Println("════════════════════════════════════════════════════════════")
	log.Println("")

	// Start serving (blocking call)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ gRPC server failed: %v", err)
	}
}
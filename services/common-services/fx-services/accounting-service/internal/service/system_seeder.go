package service
// accounting-service/internal/service/system_seeder.go

import (
	"context"
	"fmt"
	"io"
	"log"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"

	authclient "x/shared/auth"
	partnerclient "x/shared/partner"
	cryptoclient "x/shared/common/crypto" //  Add crypto client
	
	authpb "x/shared/genproto/authpb"
	partnerpb "x/shared/genproto/partner/svcpb"
	cryptopb "x/shared/genproto/shared/accounting/cryptopb" //  Add crypto proto

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SystemSeeder handles initial setup of user and partner accounts
type SystemSeeder struct {
	accountUC     *usecase.AccountUsecase
	authClient    *authclient.AuthService
	partnerClient *partnerclient.PartnerService
	cryptoClient  *cryptoclient.CryptoClient //  Add crypto client
	db            *pgxpool.Pool
}

func NewSystemSeeder(
	accountUC *usecase. AccountUsecase,
	authClient *authclient.AuthService,
	partnerClient *partnerclient.PartnerService,
	cryptoClient *cryptoclient.CryptoClient, //  Add parameter
	db *pgxpool. Pool,
) *SystemSeeder {
	return &SystemSeeder{
		accountUC:     accountUC,
		authClient:    authClient,
		partnerClient: partnerClient,
		cryptoClient:  cryptoClient, //  Store it
		db:            db,
	}
}

// SeedSystem seeds user and partner accounts from auth and partner services
func (s *SystemSeeder) SeedSystem(ctx context.Context) error {
	log.Println("🚀 Starting system seeding...")

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt. Errorf("failed to begin tx:  %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Seed user accounts
	if err := s. seedUserAccounts(ctx, tx); err != nil {
		return err
	}

	// 2. Seed partner accounts
	if err := s.seedPartnerAccounts(ctx, tx); err != nil {
		return err
	}

	// Commit changes
	if err := tx. Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit:  %w", err)
	}

	log.Println(" System seeding completed!")
	return nil
}

func (s *SystemSeeder) seedUserAccounts(ctx context.Context, tx pgx.Tx) error {
	log.Println("📥 Streaming users from auth-service...")

	stream, err := s.authClient. UserClient.StreamAllUsers(ctx, &authpb.StreamAllUsersRequest{
		BatchSize: 2500,
	})
	if err != nil {
		return fmt. Errorf("failed to stream users: %w", err)
	}

	var accountBatch []*domain.CreateAccountRequest
	var walletBatch []string // User IDs for wallet creation
	userCount := 0
	const BATCH_SIZE = 500

	for {
		user, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Create accounting accounts (BTC, USDT, USD Demo, USD Real)
		accountBatch = append(accountBatch, 
			&domain.CreateAccountRequest{
				OwnerType:       domain.OwnerTypeUser,
				OwnerID:         user.Id,
				Currency:       "BTC",
				Purpose:        domain.PurposeWallet,
				AccountType:    domain.AccountTypeReal,
				InitialBalance: 0,
			},
			&domain.CreateAccountRequest{
				OwnerType:       domain.OwnerTypeUser,
				OwnerID:         user.Id,
				Currency:       "TRX",
				Purpose:        domain.PurposeWallet,
				AccountType:    domain.AccountTypeReal,
				InitialBalance: 0,
			},
			&domain.CreateAccountRequest{
				OwnerType:      domain.OwnerTypeUser,
				OwnerID:        user.Id,
				Currency:       "USDT",
				Purpose:        domain.PurposeWallet,
				AccountType:    domain.AccountTypeReal,
				InitialBalance:  0,
			},
			&domain.CreateAccountRequest{
				OwnerType:      domain.OwnerTypeUser,
				OwnerID:        user.Id,
				Currency:       "USD",
				Purpose:        domain.PurposeWallet,
				AccountType:    domain.AccountTypeDemo,
				InitialBalance: 1000000, // $10,000 demo
			},
			&domain.CreateAccountRequest{
				OwnerType:      domain.OwnerTypeUser,
				OwnerID:        user.Id,
				Currency:       "USD",
				Purpose:        domain.PurposeWallet,
				AccountType:    domain.AccountTypeReal,
				InitialBalance: 0,
			},
		)

		//  Collect user ID for wallet creation
		walletBatch = append(walletBatch, user.Id)

		userCount++

		// Flush every BATCH_SIZE users
		if userCount%BATCH_SIZE == 0 {
			// Create accounting accounts
			errs := s.accountUC. CreateAccounts(ctx, accountBatch, tx)
			if hasErrors(errs) {
				log.Printf("⚠️  Errors in account batch (continuing): %v", errs)
			}
			
			//  Create crypto wallets (only if crypto service is available)
			if s.cryptoClient != nil {
				s.createCryptoWalletsForUsers(ctx, walletBatch)
			}
			
			log.Printf("✔️  Processed %d users (%d accounts, %d wallets)...", 
				userCount, len(accountBatch), len(walletBatch)*3) // 3 wallets per user
			
			accountBatch = accountBatch[:0]
			walletBatch = walletBatch[:0]
		}
	}

	// 🔥 Flush remaining
	if len(accountBatch) > 0 {
		errs := s.accountUC. CreateAccounts(ctx, accountBatch, tx)
		if hasErrors(errs) {
			log.Printf("⚠️  Errors in final account batch: %v", errs)
		}
		
		if s.cryptoClient != nil {
			s.createCryptoWalletsForUsers(ctx, walletBatch)
		}
		
		log.Printf("✔️  Processed final batch:  %d users", len(walletBatch))
	}

	log.Printf(" User accounts seeded: %d users total", userCount)
	return nil
}

//  NEW: Create crypto wallets for users
func (s *SystemSeeder) createCryptoWalletsForUsers(ctx context.Context, userIDs []string) {
	log.Printf("💰 Creating crypto wallets for %d users...", len(userIDs))

	successCount := 0
	failCount := 0

	for _, userID := range userIDs {
		// Call crypto service to initialize wallets
		resp, err := s.cryptoClient.WalletClient.InitializeUserWallets(ctx, &cryptopb.InitializeUserWalletsRequest{
			UserId:        userID,
			SkipExisting: true, // Idempotent - don't recreate if exists
			// Leave chains/assets empty to create all default wallets
		})

		if err != nil {
			log. Printf("⚠️  Failed to create wallets for user %s: %v", userID, err)
			failCount++
			continue
		}

		if resp.TotalFailed > 0 {
			log.Printf("⚠️  User %s:  %d wallets created, %d failed", 
				userID, resp.TotalCreated, resp.TotalFailed)
		}

		successCount++
	}

	log.Printf(" Crypto wallets created:  %d users successful, %d failed", successCount, failCount)
}

// accounting-service/internal/service/system_seeder.go

func (s *SystemSeeder) seedPartnerAccounts(ctx context.Context, tx pgx.Tx) error {
	log. Println("📥 Streaming partners from partner-service...")

	stream, err := s.partnerClient.Client.StreamAllPartners(ctx, &partnerpb.StreamAllPartnersRequest{
		BatchSize: 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to stream partners: %w", err)
	}

	var accountBatch []*domain.CreateAccountRequest
	var partnerWalletBatch []*PartnerWalletSpec //  Store partner ID + currency
	partnerCount := 0
	const BATCH_SIZE = 500

	for {
		partner, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("partner stream error: %w", err)
		}

		currency := "USD"
		if partner.Currency != "" {
			currency = partner.Currency
		}

		// Create settlement account
		accountBatch = append(accountBatch, &domain. CreateAccountRequest{
			OwnerType:      domain.OwnerTypePartner,
			OwnerID:        partner.Id,
			Currency:       currency,
			Purpose:        domain.PurposeSettlement,
			AccountType:    domain.AccountTypeReal,
			InitialBalance:  0,
		})

		//  Collect partner info for wallet creation
		partnerWalletBatch = append(partnerWalletBatch, &PartnerWalletSpec{
			PartnerID: partner.Id,
			Currency:  currency,
		})

		partnerCount++

		if len(accountBatch) >= BATCH_SIZE {
			errs := s.accountUC.CreateAccounts(ctx, accountBatch, tx)
			if hasErrors(errs) {
				log.Printf("⚠️  Errors in partner batch:  %v", errs)
			}
			
			//  Create crypto wallets for partners
			if s.cryptoClient != nil {
				s.createCryptoWalletsForPartners(ctx, partnerWalletBatch)
			}
			
			log.Printf("✔️  Processed %d partners.. .", partnerCount)
			accountBatch = accountBatch[:0]
			partnerWalletBatch = partnerWalletBatch[: 0]
		}
	}

	// Flush remaining
	if len(accountBatch) > 0 {
		errs := s.accountUC.CreateAccounts(ctx, accountBatch, tx)
		if hasErrors(errs) {
			log.Printf("⚠️  Errors in final partner batch: %v", errs)
		}
		
		if s.cryptoClient != nil {
			s.createCryptoWalletsForPartners(ctx, partnerWalletBatch)
		}
	}

	log. Printf(" Partner accounts seeded: %d partners", partnerCount)
	return nil
}

// PartnerWalletSpec holds partner wallet creation info
type PartnerWalletSpec struct {
	PartnerID string
	Currency  string
}

//  UPDATED: Create crypto wallets based on partner's currency
func (s *SystemSeeder) createCryptoWalletsForPartners(ctx context.Context, partners []*PartnerWalletSpec) {
	log.Printf("💰 Creating crypto wallets for %d partners...", len(partners))

	successCount := 0
	failCount := 0
	skippedCount := 0

	for _, partner := range partners {
		// Map currency to wallet specs
		walletSpecs := s.getCryptoWalletsForCurrency(partner.Currency)
		
		if len(walletSpecs) == 0 {
			log.Printf("⏭️  Partner %s currency %s - no crypto wallets needed", 
				partner.PartnerID, partner.Currency)
			skippedCount++
			continue
		}

		// Create wallets for this partner
		resp, err := s.cryptoClient.WalletClient.CreateWallets(ctx, &cryptopb.CreateWalletsRequest{
			UserId:  partner. PartnerID,
			Wallets: walletSpecs,
		})

		if err != nil {
			log.Printf("⚠️  Failed to create wallets for partner %s (%s): %v", 
				partner.PartnerID, partner.Currency, err)
			failCount++
			continue
		}

		if resp.FailedCount > 0 {
			log.Printf("⚠️  Partner %s (%s): %d wallets created, %d failed", 
				partner. PartnerID, partner.Currency, resp.SuccessCount, resp.FailedCount)
		}

		successCount++
	}

	log.Printf(" Partner crypto wallets:  %d successful, %d failed, %d skipped", 
		successCount, failCount, skippedCount)
}

// getCryptoWalletsForCurrency returns wallet specs based on currency
func (s *SystemSeeder) getCryptoWalletsForCurrency(currency string) []*cryptopb.WalletSpec {
	// Map fiat/crypto currencies to their crypto wallet requirements
	currencyMap := map[string][]*cryptopb.WalletSpec{
		// USDT partners need TRON USDT wallet
		"USDT": {
			{Chain: cryptopb.Chain_CHAIN_TRON, Asset: "USDT", Label:  "Partner USDT Settlement"},
		},
		
		// BTC partners need Bitcoin wallet
		"BTC": {
			{Chain: cryptopb. Chain_CHAIN_BITCOIN, Asset: "BTC", Label:  "Partner BTC Settlement"},
		},
		
		// TRX partners need TRON TRX wallet
		"TRX": {
			{Chain: cryptopb.Chain_CHAIN_TRON, Asset: "TRX", Label: "Partner TRX Settlement"},
		},
		
		// USD partners might need both USDT and BTC for settlements
		// "USD": {
		// 	{Chain: cryptopb.Chain_CHAIN_TRON, Asset: "USDT", Label: "Partner USDT Settlement"},
		// 	{Chain: cryptopb.Chain_CHAIN_BITCOIN, Asset: "BTC", Label: "Partner BTC Settlement"},
		// },
		
		// EUR partners - same as USD (crypto settlement options)
		// "EUR": {
		// 	{Chain: cryptopb. Chain_CHAIN_TRON, Asset: "USDT", Label: "Partner USDT Settlement"},
		// 	{Chain: cryptopb.Chain_CHAIN_BITCOIN, Asset: "BTC", Label: "Partner BTC Settlement"},
		// },
		
		// Add more currency mappings as needed
		// "ETH": {
		// 	{Chain: cryptopb.Chain_CHAIN_ETHEREUM, Asset: "ETH", Label: "Partner ETH Settlement"},
		// },
	}

	if wallets, ok := currencyMap[currency]; ok {
		return wallets
	}

	// No crypto wallets for this currency
	return nil
}

// hasErrors checks if error map has any errors
func hasErrors(errs map[int]error) bool {
	return len(errs) > 0
}

// SeedAgentAccounts seeds accounts for agents (if you have agents)
// This is optional - only use if you have an agent service
func (s *SystemSeeder) SeedAgentAccounts(ctx context.Context) error {
	log.Println("📥 Seeding agent accounts...")

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Example: If you have a list of agents
	// For now, this is just a placeholder
	var batch []*domain.CreateAccountRequest

	// Example agent IDs (replace with actual agent data source)
	agentIDs := []string{
		"agent-001",
		"agent-002",
		"agent-003",
	}

	for _, agentID := range agentIDs {
		// Create commission account for each agent
		commissionRate := "0.15" // 15% commission rate
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeAgent,
			OwnerID:        agentID,
			Currency:       "USD",
			Purpose:        domain.PurposeCommission,
			AccountType:    domain.AccountTypeReal,
			CommissionRate: &commissionRate,
			InitialBalance: 0,
		})
	}

	if len(batch) > 0 {
		errs := s.accountUC.CreateAccounts(ctx, batch, tx)
		if hasErrors(errs) {
			log.Printf("⚠️  Errors creating agent accounts: %v", errs)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit agent accounts: %w", err)
	}

	log.Printf(" Agent accounts seeded: %d agents", len(agentIDs))
	return nil
}

// SeedSystemAccounts creates system operational accounts
// Run this before seeding user/partner accounts
func (s *SystemSeeder) SeedSystemAccounts(ctx context.Context) error {
	log.Println("🏦 Creating system accounts...")

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Define currencies to create system accounts for
	currencies := []string{"USD", "BTC", "USDT", "TRX"}

	var batch []*domain.CreateAccountRequest

	for _, currency := range currencies {
		// 1. System Liquidity Account
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       currency,
			Purpose:        domain.PurposeLiquidity,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 100000000000, // $1,000,000.00 initial liquidity
			OverdraftLimit: 0,
		})

		// 2. System Fee Account
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       currency,
			Purpose:        domain.PurposeFees,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})

		// 3. System Clearing Account
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       currency,
			Purpose:        domain.PurposeClearing,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})

		// 4. System Settlement Account
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       currency,
			Purpose:        domain.PurposeSettlement,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})

		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeSystem,
			OwnerID:        "system",
			Currency:       currency,
			Purpose:        domain.PurposeRevenue,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})
	}

	// Create all system accounts
	errs := s.accountUC.CreateAccounts(ctx, batch, tx)
	if hasErrors(errs) {
		// Log errors but continue (accounts might already exist)
		log.Printf("⚠️  Some system accounts already exist or failed: %v", errs)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit system accounts: %w", err)
	}

	log.Printf(" System accounts created for %d currencies", len(currencies))
	return nil
}
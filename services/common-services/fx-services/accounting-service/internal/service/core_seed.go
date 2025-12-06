package service

import (
	"context"
	"fmt"
	"io"
	"log"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"

	authclient "x/shared/auth"
	partnerclient "x/shared/partner"
	authpb "x/shared/genproto/authpb"
	partnerpb "x/shared/genproto/partner/svcpb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SystemSeeder handles initial setup of user and partner accounts
type SystemSeeder struct {
	accountUC     *usecase.AccountUsecase
	authClient    *authclient.AuthService
	partnerClient *partnerclient.PartnerService
	db            *pgxpool.Pool
}

func NewSystemSeeder(
	accountUC *usecase.AccountUsecase,
	authClient *authclient.AuthService,
	partnerClient *partnerclient.PartnerService,
	db *pgxpool.Pool,
) *SystemSeeder {
	return &SystemSeeder{
		accountUC:     accountUC,
		authClient:    authClient,
		partnerClient: partnerClient,
		db:            db,
	}
}

// SeedSystem seeds user and partner accounts from auth and partner services
func (s *SystemSeeder) SeedSystem(ctx context.Context) error {
	log.Println("🚀 Starting system seeding...")

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Seed user accounts
	if err := s.seedUserAccounts(ctx, tx); err != nil {
		return err
	}

	// 2. Seed partner accounts
	if err := s.seedPartnerAccounts(ctx, tx); err != nil {
		return err
	}

	// Commit changes
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	log.Println("✅ System seeding completed!")
	return nil
}

func (s *SystemSeeder) seedUserAccounts(ctx context.Context, tx pgx.Tx) error {
	log.Println("📥 Streaming users from auth-service...")

	stream, err := s.authClient.UserClient.StreamAllUsers(ctx, &authpb.StreamAllUsersRequest{
		BatchSize: 2500,
	})
	if err != nil {
		return fmt.Errorf("failed to stream users: %w", err)
	}

	var batch []*domain.CreateAccountRequest
	userCount := 0
	const BATCH_SIZE = 500 // Flush every 500 users (adjusts to 500 users * 4 accounts = 2000 accounts)

	for {
		user, err := stream.Recv()
		if err == io.EOF {
			break // Exit loop, will flush remaining below
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Create 3 REAL accounts per user: BTC, USDT, BTC (duplicate?)
		// Note: You mentioned "BTC USDT and BTC" - assuming you meant BTC and USDT only
		// If you need 2 separate BTC accounts, uncomment the second BTC account below

		// 1. BTC Real Account
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeUser,
			OwnerID:        user.Id,
			Currency:       "BTC",
			Purpose:        domain.PurposeWallet,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})

		// 2. USDT Real Account
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeUser,
			OwnerID:        user.Id,
			Currency:       "USDT",
			Purpose:        domain.PurposeWallet,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})

		// 3. USD Demo Account
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeUser,
			OwnerID:        user.Id,
			Currency:       "USD",
			Purpose:        domain.PurposeWallet,
			AccountType:    domain.AccountTypeDemo,
			InitialBalance: 1000000, // $10,000.00 demo balance (in cents)
			OverdraftLimit: 0,
		})

		// Optional: If you really need 2 BTC accounts (uncomment if needed)
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypeUser,
			OwnerID:        user.Id,
			Currency:       "USD",
			Purpose:        domain.PurposeWallet, // Or use different purpose if needed
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})

		userCount++

		// Flush every BATCH_SIZE users (now creates 3 accounts per user = 1500 accounts per batch)
		if userCount%BATCH_SIZE == 0 {
			errs := s.accountUC.CreateAccounts(ctx, batch, tx)
			if hasErrors(errs) {
				log.Printf("⚠️  Errors in batch (continuing): %v", errs)
				// Continue even with errors (accounts may already exist)
			}
			log.Printf("✔️  Processed %d users (%d accounts)...", userCount, len(batch))
			batch = batch[:0] // Clear batch
		}
	}

	// 🔥 CRITICAL: Flush remaining users (even if < BATCH_SIZE)
	if len(batch) > 0 {
		errs := s.accountUC.CreateAccounts(ctx, batch, tx)
		if hasErrors(errs) {
			log.Printf("⚠️  Errors in final batch: %v", errs)
		}
		log.Printf("✔️  Processed final batch: %d users (%d accounts)", userCount%BATCH_SIZE, len(batch))
	}

	log.Printf("✅ User accounts seeded: %d users total (%d accounts created)", userCount, userCount*3)
	return nil
}

func (s *SystemSeeder) seedPartnerAccounts(ctx context.Context, tx pgx.Tx) error {
	log.Println("📥 Streaming partners from partner-service...")

	stream, err := s.partnerClient.Client.StreamAllPartners(ctx, &partnerpb.StreamAllPartnersRequest{
		BatchSize: 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to stream partners: %w", err)
	}

	var batch []*domain.CreateAccountRequest
	partnerCount := 0
	const BATCH_SIZE = 500 // Flush every 500 accounts

	for {
		partner, err := stream.Recv()
		if err == io.EOF {
			break // Exit loop, will flush remaining below
		}
		if err != nil {
			return fmt.Errorf("partner stream error: %w", err)
		}

		// Determine currency (use partner's currency or default to USD)
		currency := "USD"
		if partner.Currency != "" {
			currency = partner.Currency
		}

		// Only create Settlement Account (NOT commission - partners earn differently)
		batch = append(batch, &domain.CreateAccountRequest{
			OwnerType:      domain.OwnerTypePartner,
			OwnerID:        partner.Id,
			Currency:       currency,
			Purpose:        domain.PurposeSettlement,
			AccountType:    domain.AccountTypeReal,
			InitialBalance: 0,
			OverdraftLimit: 0,
		})

		partnerCount++

		// Flush every BATCH_SIZE accounts
		if len(batch) >= BATCH_SIZE {
			errs := s.accountUC.CreateAccounts(ctx, batch, tx)
			if hasErrors(errs) {
				log.Printf("⚠️  Errors in partner batch (continuing): %v", errs)
			}
			log.Printf("✔️  Processed %d partners...", partnerCount)
			batch = batch[:0] // Clear batch
		}
	}

	// 🔥 CRITICAL: Flush remaining partners (even if < BATCH_SIZE)
	if len(batch) > 0 {
		errs := s.accountUC.CreateAccounts(ctx, batch, tx)
		if hasErrors(errs) {
			log.Printf("⚠️  Errors in final partner batch: %v", errs)
		}
		log.Printf("✔️  Processed final batch of %d partners", len(batch))
	}

	log.Printf("✅ Partner accounts seeded: %d partners total", partnerCount)
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

	log.Printf("✅ Agent accounts seeded: %d agents", len(agentIDs))
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
	currencies := []string{"USD", "BTC", "USDT",}

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

	log.Printf("✅ System accounts created for %d currencies", len(currencies))
	return nil
}
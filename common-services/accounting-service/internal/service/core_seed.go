package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"accounting-service/internal/domain"
	"accounting-service/internal/usecase"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SystemSeeder handles initial setup of currencies, FX rates, and system account
type SystemSeeder struct {
	fxService   *FXService
	accountUC   *usecase.AccountUsecase
	ledgerUC    *usecase.LedgerUsecase
	statementUC *usecase.StatementUsecase
	db          *pgxpool.Pool
}

func NewSystemSeeder(
	fx *FXService,
	accountUC *usecase.AccountUsecase,
	ledgerUC *usecase.LedgerUsecase,
	statementUC *usecase.StatementUsecase,
	db *pgxpool.Pool,
) *SystemSeeder {
	return &SystemSeeder{
		fxService:   fx,
		accountUC:   accountUC,
		ledgerUC:    ledgerUC,
		statementUC: statementUC,
		db:          db,
	}
}


// SeedSystem seeds currencies, FX rates, system account, and initial balance
func (s *SystemSeeder) SeedSystem(ctx context.Context) error {
	log.Println("🚀 Starting system seeding...")

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Seed common currencies
	if errs := s.fxService.FetchCommonCurrencies(ctx, tx); len(errs) > 0 {
		for _, e := range errs {
			log.Printf("⚠️ currency seed error: %v", e)
		}
	}

	// Seed FX rates for USD
	if errs := s.fxService.FetchFXRates(ctx, "USD", tx); len(errs) > 0 {
		for _, e := range errs {
			log.Printf("⚠️ FX rate seed error: %v", e)
		}
	}

	// Create system account
	systemAccount := &domain.Account{
		OwnerType:   "system",
		OwnerID:     "SYSTEM",
		Currency:    "USD",
		Purpose:     "wallet",
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	errMap := s.accountUC.CreateAccounts(ctx, []*domain.Account{systemAccount}, tx)
	if err, exists := errMap[0]; exists {
		return fmt.Errorf("failed to create system account: %w", err)
	}

	// Create seed capital account
	seedCapitalAccount := &domain.Account{
		OwnerType:   "system",
		OwnerID:     "SEED_CAPITAL",
		Currency:    "USD",
		Purpose:     "wallet",
		AccountType: "real",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	errMap = s.accountUC.CreateAccounts(ctx, []*domain.Account{seedCapitalAccount}, tx)
	if err, exists := errMap[0]; exists {
		return fmt.Errorf("failed to create seed capital account: %w", err)
	}

	// Post initial credit transaction only if accounts have 0 balance
	balance, err := s.statementUC.GetAccountBalance(ctx, systemAccount.ID)

	if err != nil || balance == 0 {
		log.Println("💰 System account has 0 balance, seeding initial capital...")

		postings := []*domain.Posting{
			{AccountID: systemAccount.ID, DrCr: "CR", Amount: 10_000_000, Currency: "USD"},
			{AccountID: seedCapitalAccount.ID, DrCr: "DR", Amount: 10_000_000, Currency: "USD"},
		}

		journal := &domain.Journal{
			Description:   "Initial seed funding",
			CreatedByType: "system",
		}

		if _, err := s.ledgerUC.CreateTransactionMulti(ctx, journal, postings, tx); err != nil {
			return fmt.Errorf("failed to create initial seed transaction: %w", err)
		}
	} else {
		log.Printf("⚠️ Skipping seed transaction: system account already has balance = %.2f\n", balance)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit seeding transaction: %w", err)
	}
	tx = nil // prevent rollback in defer

	log.Println("🎉 System seeding completed successfully")
	return nil
}


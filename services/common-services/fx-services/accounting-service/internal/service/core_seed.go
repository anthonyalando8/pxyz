package service

import (
	"context"
	//"fmt"
	"log"

	//"accounting-service/internal/domain"
	"accounting-service/internal/usecase"

	//"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SystemSeeder handles initial setup of currencies, FX rates, system accounts, and fee rules
type SystemSeeder struct {
	fxService   *FXService
	accountUC   *usecase.AccountUsecase
	ruleUC      *usecase.TransactionFeeRuleUsecase
	db          *pgxpool.Pool
}

func NewSystemSeeder(
	fx *FXService,
	accountUC *usecase.AccountUsecase,
	ruleUC *usecase.TransactionFeeRuleUsecase,
	db *pgxpool.Pool,
) *SystemSeeder {
	return &SystemSeeder{
		fxService: fx,
		accountUC: accountUC,
		ruleUC:    ruleUC,
		db:        db,
	}
}

// SeedSystem seeds currencies, FX rates, system accounts, and fee rules
func (s *SystemSeeder) SeedSystem(ctx context.Context) error {
	log.Println("🚀 Starting system seeding...")

	// // Begin transaction
	// tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	// if err != nil {
	// 	return fmt.Errorf("failed to begin transaction: %w", err)
	// }
	// defer func() {
	// 	if tx != nil {
	// 		_ = tx.Rollback(ctx)
	// 	}
	// }()

	// // 1. Seed currencies (USD, BTC, USDT)
	// if errs := s.fxService.FetchCommonCurrencies(ctx, tx); len(errs) > 0 {
	// 	for _, e := range errs {
	// 		log.Printf("⚠️ currency seed error: %v", e)
	// 	}
	// }

	// // 2. Seed FX rates for USD
	// if errs := s.fxService.FetchFXRates(ctx, "USD", tx); len(errs) > 0 {
	// 	for _, e := range errs {
	// 		log.Printf("⚠️ FX rate seed error: %v", e)
	// 	}
	// }

	// // 3. Create system accounts with balances
	// systemAccounts := domain.DefaultSystemAccounts

	// if errMap := s.accountUC.CreateAccounts(ctx, systemAccounts, tx); len(errMap) > 0 {
	// 	for i, e := range errMap {
	// 		log.Printf("⚠️ system account insert error #%d: %v", i, e)
	// 	}
	// 	return fmt.Errorf("failed to create system accounts")
	// }

	// // Ensure account numbers are set
	// for _, acc := range systemAccounts {
	// 	if acc.AccountNumber == "" {
	// 		acc.AccountNumber = fmt.Sprintf("WL-%d", acc.ID)
	// 	}
	// }
	// log.Println("✅ Created system accounts with initial balances")

	// // 4. Seed transaction fee rules
	// if errs := s.ruleUC.CreateBatch(ctx, domain.DefaultTransactionFeeRules, tx); len(errs) > 0 {
	// 	for i, e := range errs {
	// 		log.Printf("⚠️ fee rule insert error #%d: %v", i, e)
	// 	}
	// 	return fmt.Errorf("failed to create transaction fee rules")
	// }
	// log.Println("✅ Created default transaction fee rules")

	// // Commit transaction
	// if err := tx.Commit(ctx); err != nil {
	// 	return fmt.Errorf("failed to commit seeding transaction: %w", err)
	// }
	// tx = nil // prevent rollback in defer

	// log.Println("🎉 System seeding completed successfully")
	return nil
}

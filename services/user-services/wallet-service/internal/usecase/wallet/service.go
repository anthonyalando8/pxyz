// --- service.go ---
package wallet

import (
	"context"
	"log"
	"time"

	"wallet-service/internal/domain"
	"wallet-service/internal/repository"
)

type Service struct {
	repo      *repository.WalletRepository
	converter CurrencyConverter
	Notifier *Notifier // Assuming Notifier is defined elsewhere

}

func New(repo *repository.WalletRepository, converter CurrencyConverter, notifier *Notifier) *Service {
	return &Service{
		repo:      repo,
		converter: converter,
		Notifier:  notifier,
	}
}

func (s *Service) GetOrCreateWallet(ctx context.Context, userID, currency string) (*domain.Wallet, error) {
	wallet, err := s.repo.GetWalletByUserIDAndCurrency(ctx, userID, currency)
	if err == nil && wallet != nil {
		return wallet, nil
	}

	newWallet := &domain.Wallet{
		UserID:    userID,
		Currency:  currency,
		Balance:   0.0,
		Available: 0.0,
		Locked:    0.0,
		Type:      "main",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.repo.CreateWallet(ctx, newWallet)
	if err != nil {
		return nil, err
	}
	return newWallet, nil
}

func (s *Service) CreateDefaultUserWallets(ctx context.Context, userID string) error {
	supportedCurrencies := []string{
		"USD", "EUR", "KES", "XAF", "XOF", "NGN", "ZAR",
		"BTC", "ETH", "USDT", "USDC", "BNB", "SOL",
	}

	for _, currency := range supportedCurrencies {
		existing, err := s.repo.GetWalletByUserIDAndCurrency(ctx, userID, currency)
		if err == nil && existing != nil {
			continue
		}

		wallet := &domain.Wallet{
			UserID:   userID,
			Currency: currency,
			Balance:  0,
		}

		if err := s.repo.CreateWallet(ctx, wallet); err != nil {
			log.Printf("failed to create wallet for %s (%s): %v", userID, currency, err)
		}
	}
	return nil
}

func (s *Service) UpdateWalletBalance(ctx context.Context, userID, currency string, balance float64) error {
	wallet := &domain.Wallet{
		UserID:    userID,
		Currency:  currency,
		Balance:   balance,
		UpdatedAt: time.Now(),
	}
	return s.repo.UpdateWalletBalance(ctx, wallet)
}

func (s *Service) GetWalletByUserIDAndCurrency(ctx context.Context, userID, currency string) (*domain.Wallet, error) {
	return s.repo.GetWalletByUserIDAndCurrency(ctx, userID, currency)
}

func (s *Service) ListUserWallets(ctx context.Context, userID string) ([]*domain.Wallet, error) {
	return s.repo.ListUserWallets(ctx, userID)
}

func (s *Service) CalculateUserNetWorthInUSD(ctx context.Context, userID string) ([]*domain.Wallet, float64, error) {
	wallets, err := s.repo.ListUserWallets(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	var netWorthUSD float64
	for _, wallet := range wallets {
		converted, err := s.converter.ConvertToUSD(ctx, wallet.Balance, wallet.Currency)
		if err != nil {
			return nil, 0, err
		}
		netWorthUSD += converted
	}

	return wallets, netWorthUSD, nil
}
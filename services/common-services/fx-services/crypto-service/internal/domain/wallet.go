// internal/domain/crypto_wallet.go
package domain

import (
	"math/big"
	"time"
)

// CryptoWallet represents a user's blockchain wallet
type CryptoWallet struct {
	ID                   int64
	UserID               string
	Chain                string
	Asset                string
	
	// Credentials
	Address              string
	PublicKey            *string
	EncryptedPrivateKey  string
	EncryptionVersion    string
	
	// Metadata
	Label                *string
	IsPrimary            bool
	IsActive             bool
	
	// Balance (cached)
	Balance              *big. Int
	LastBalanceUpdate    *time.Time
	
	// Monitoring
	LastDepositCheck     *time.Time
	LastTransactionBlock *int64
	
	// Timestamps
	CreatedAt            time. Time
	UpdatedAt            time.Time
}

// DecryptedWallet contains decrypted credentials for transactions
type DecryptedWallet struct {
	*CryptoWallet
	PrivateKey string  // Decrypted private key (never persisted)
}

// WalletBalance represents wallet balance with formatting
type WalletBalance struct {
	WalletID      int64
	Address       string
	Chain         string
	Asset         string
	Balance       *big.Int
	BalanceFormatted string  // Human-readable (e.g., "100. 50 USDT")
	Decimals      int
	UpdatedAt     time.Time
}
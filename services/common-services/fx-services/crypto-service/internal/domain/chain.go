// internal/domain/chain.go
package domain

import (
	"context"
	"math/big"
	"time"
)

// Chain represents a blockchain network
type Chain interface {
	// Name returns the chain name (TRON, ETH, BTC)
	Name() string
	
	// Symbol returns native coin symbol (TRX, ETH, BTC)
	Symbol() string
	
	// GenerateWallet creates a new wallet
	GenerateWallet(ctx context.Context) (*Wallet, error)
	
	// ImportWallet imports wallet from private key
	ImportWallet(ctx context.Context, privateKey string) (*Wallet, error)
	
	// GetBalance gets balance for address
	GetBalance(ctx context.Context, address string, walletID string, asset *Asset) (*Balance, error)
	
	// EstimateFee estimates transaction fee
	EstimateFee(ctx context.Context, req *TransactionRequest) (*Fee, error)
	
	// Send sends transaction
	Send(ctx context.Context, req *TransactionRequest) (*TransactionResult, error)
	
	// GetTransaction gets transaction status
	GetTransaction(ctx context. Context, txHash string) (*Transaction, error)
	
	// ValidateAddress validates address format
	ValidateAddress(address string) error
}

// Wallet represents a blockchain wallet
type Wallet struct {
	Address    string
	PrivateKey string // Encrypted in production
	PublicKey  string
	Chain      string
	CreatedAt  time.Time
}

// Asset represents a crypto asset
type Asset struct {
	Chain        string  // TRON, ETH, BTC
	Symbol       string  // TRX, USDT, ETH, BTC
	ContractAddr *string // For tokens (TRC20, ERC20)
	Decimals     int
	Type         AssetType
}

type AssetType string

const (
	AssetTypeNative AssetType = "native"  // TRX, ETH, BTC
	AssetTypeToken  AssetType = "token"   // USDT, USDC
)

// Balance represents account balance
type Balance struct {
	Address  string
	Asset    *Asset
	Amount   *big.Int
	Decimals int
}

// TransactionRequest represents a send request
type TransactionRequest struct {
	From        string
	To          string
	Asset       *Asset
	Amount      *big.Int
	PrivateKey  string
	Memo        *string
	Priority    TxPriority
}

type TxPriority string

const (
	TxPriorityLow    TxPriority = "low"
	TxPriorityNormal TxPriority = "normal"
	TxPriorityHigh   TxPriority = "high"
)

// TransactionResult represents send result
type TransactionResult struct {
	TxHash    string
	Status    TxStatus
	Fee       *big.Int
	Timestamp time. Time
}

// Transaction represents blockchain transaction
type Transaction struct {
	Hash          string
	Chain         string
	From          string
	To            string
	Asset         *Asset
	Amount        *big.Int
	Fee           *big.Int
	Status        TxStatus
	Confirmations int
	BlockNumber   *int64
	Timestamp     time.Time
}

type TxStatus string

const (
	TxStatusPending   TxStatus = "pending"
	TxStatusConfirmed TxStatus = "confirmed"
	TxStatusFailed    TxStatus = "failed"
)

// Fee represents transaction fee
type Fee struct {
	Amount   *big.Int
	Currency string
	GasLimit *int64
	GasPrice *big.Int
}
# 🚀 Crypto Service - Complete Implementation Plan

---

## 📋 Service Overview

```
crypto-service/
├── Handles:  TRX, USDT(TRC20), ETH, BTC
├── Protocol: gRPC
├── Language: Go
├── Database: PostgreSQL (wallet metadata, tx history)
├── Cache: Redis (balance caching, nonce management)
└── External: TronGrid, Infura, Bitcoin RPC
```

---

## 🏗️ Complete Project Structure

```
crypto-service/
│
├── cmd/
│   └── server/
│       └── main. go
│
├── api/
│   └── proto/
│       ├── crypto.proto
│       └── wallet.proto
│
├── internal/
│   ├── server/
│   │   └── grpc_server.go
│   │
│   ├── handler/
│   │   ├── wallet_handler.go
│   │   ├── transaction_handler.go
│   │   └── balance_handler.go
│   │
│   ├── usecase/
│   │   ├── wallet_usecase.go
│   │   ├── transaction_usecase.go
│   │   └── balance_usecase.go
│   │
│   ├── domain/
│   │   ├── chain.go          # Core interface
│   │   ├── wallet.go
│   │   ├── transaction.go
│   │   ├── asset.go
│   │   └── errors.go
│   │
│   ├── chains/
│   │   ├── registry.go       # Chain registry
│   │   │
│   │   ├── tron/
│   │   │   ├── tron.go       # Chain implementation
│   │   │   ├── wallet.go     # Address generation
│   │   │   ├── trc20.go      # USDT handling
│   │   │   ├── client.go     # TronGrid API
│   │   │   └── signer.go     # Transaction signing
│   │   │
│   │   ├── ethereum/
│   │   │   ├── ethereum.go
│   │   │   ├── wallet.go
│   │   │   ├── erc20.go
│   │   │   └── client.go
│   │   │
│   │   └── bitcoin/
│   │       ├── bitcoin.go
│   │       ├── wallet.go
│   │       └── client.go
│   │
│   ├── repository/
│   │   ├── wallet_repo.go
│   │   └── transaction_repo.go
│   │
│   ├── security/
│   │   ├── encryption.go     # Key encryption
│   │   └── vault.go          # Key storage
│   │
│   └── config/
│       └── config.go
│
├── pkg/
│   └── utils/
│       └── crypto.go
│
├── migrations/
│   ├── 001_wallets.sql
│   └── 002_transactions.sql
│
├── docker-compose.yml
├── Dockerfile
└── go. mod
```

---

## 🧩 Phase 1: Core Domain (Start Here)

### 1. Domain Interfaces

```go
// internal/domain/chain.go
package domain

import (
	"context"
	"math/big"
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
	GetBalance(ctx context.Context, address string, asset *Asset) (*Balance, error)
	
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
```

### 2. Chain Registry

```go
// internal/chains/registry.go
package chains

import (
	"crypto-service/internal/domain"
	"fmt"
	"sync"
)

type Registry struct {
	chains map[string]domain.Chain
	mu     sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		chains: make(map[string]domain.Chain),
	}
}

// Register adds a chain to registry
func (r *Registry) Register(chain domain.Chain) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chains[chain.Name()] = chain
}

// Get retrieves a chain by name
func (r *Registry) Get(name string) (domain.Chain, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	chain, ok := r.chains[name]
	if !ok {
		return nil, fmt. Errorf("chain not supported: %s", name)
	}
	
	return chain, nil
}

// List returns all registered chains
func (r *Registry) List() []string {
	r.mu. RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.chains))
	for name := range r.chains {
		names = append(names, name)
	}
	
	return names
}
```

---

## 🔷 Phase 2: TRON Implementation (This Week)

### 1. TRON Chain Implementation

```go
// internal/chains/tron/tron. go
package tron

import (
	"context"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"go.uber.org/zap"
)

type TronChain struct {
	client  *client.GrpcClient
	network string // mainnet, shasta, nile
	logger  *zap. Logger
}

func NewTronChain(apiKey, network string, logger *zap.Logger) (*TronChain, error) {
	var grpcURL string
	
	switch network {
	case "mainnet":
		grpcURL = "grpc.trongrid.io:50051"
	case "shasta":
		grpcURL = "grpc. shasta.trongrid.io:50051"
	case "nile":
		grpcURL = "grpc.nile.trongrid.io:50051"
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}
	
	c := client.NewGrpcClient(grpcURL)
	c.SetAPIKey(apiKey)
	
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("failed to start TRON client: %w", err)
	}
	
	return &TronChain{
		client:  c,
		network: network,
		logger:  logger,
	}, nil
}

func (t *TronChain) Name() string {
	return "TRON"
}

func (t *TronChain) Symbol() string {
	return "TRX"
}

// GenerateWallet creates new TRON wallet
func (t *TronChain) GenerateWallet(ctx context.Context) (*domain.Wallet, error) {
	t.logger.Info("generating TRON wallet")
	
	wallet, err := generateTronWallet()
	if err != nil {
		return nil, fmt.Errorf("failed to generate wallet: %w", err)
	}
	
	t.logger.Info("TRON wallet generated",
		zap.String("address", wallet. Address))
	
	return wallet, nil
}

// GetBalance gets TRX or TRC20 balance
func (t *TronChain) GetBalance(ctx context.Context, address string, asset *domain.Asset) (*domain.Balance, error) {
	if asset.Type == domain.AssetTypeNative {
		return t.getTRXBalance(ctx, address, asset)
	}
	
	return t.getTRC20Balance(ctx, address, asset)
}

// Send sends TRX or TRC20
func (t *TronChain) Send(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	if req.Asset.Type == domain. AssetTypeNative {
		return t.sendTRX(ctx, req)
	}
	
	return t.sendTRC20(ctx, req)
}

// More methods in next file...
```

### 2. TRON Wallet Generation

```go
// internal/chains/tron/wallet.go
package tron

import (
	"crypto-service/internal/domain"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"time"
	
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

func generateTronWallet() (*domain.Wallet, error) {
	// Generate private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt. Errorf("failed to generate private key: %w", err)
	}
	
	// Get public key
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	
	// Generate TRON address
	addr := address.PubkeyToAddress(*publicKey)
	
	// Convert to hex strings
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	publicKeyHex := hex.EncodeToString(crypto.FromECDSAPub(publicKey))
	
	return &domain.Wallet{
		Address:    addr.String(),
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Chain:      "TRON",
		CreatedAt:  time.Now(),
	}, nil
}

func importTronWallet(privateKeyHex string) (*domain.Wallet, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	addr := address.PubkeyToAddress(*publicKey)
	publicKeyHex := hex.EncodeToString(crypto.FromECDSAPub(publicKey))
	
	return &domain.Wallet{
		Address:    addr.String(),
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Chain:      "TRON",
		CreatedAt:  time.Now(),
	}, nil
}
```
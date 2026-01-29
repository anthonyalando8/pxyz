// internal/chains/ethereum/ethereum.go
package ethereum

import (
	"context"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

type EthereumChain struct {
	client *ethclient.Client
	logger *zap.Logger
	config *Config
}

type Config struct {
	RPCURL          string
	ChainID         *big.Int
	GasLimitETH     uint64
	GasLimitERC20   uint64
	MaxGasPrice     *big.Int
	Confirmations   int
}

func NewEthereumChain(rpcURL string, logger *zap.Logger) (*EthereumChain, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum: %w", err)
	}

	// Get chain ID
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	config := &Config{
		RPCURL:        rpcURL,
		ChainID:       chainID,
		GasLimitETH:   21000,    // Standard ETH transfer
		GasLimitERC20: 65000,    // ERC-20 transfer
		MaxGasPrice:   big.NewInt(100e9), // 100 Gwei
		Confirmations: 12,       // ~3 minutes
	}

	logger.Info("Ethereum chain initialized",
		zap.String("rpc", rpcURL),
		zap.String("chain_id", chainID.String()))

	return &EthereumChain{
		client: client,
		logger: logger,
		config: config,
	}, nil
}

// Name returns chain name
func (c *EthereumChain) Name() string {
	return "ETHEREUM"
}

// Symbol returns native coin symbol
func (c *EthereumChain) Symbol() string {
	return "ETH"
}

// GenerateWallet creates a new Ethereum wallet
func (c *EthereumChain) GenerateWallet(ctx context.Context) (*domain.Wallet, error) {
	return generateEthereumWallet()
}

// ImportWallet imports wallet from private key
func (c *EthereumChain) ImportWallet(ctx context.Context, privateKey string) (*domain.Wallet, error) {
	return importEthereumWallet(privateKey)
}

// ValidateAddress validates Ethereum address
func (c *EthereumChain) ValidateAddress(address string) error {
	if !common.IsHexAddress(address) {
		return fmt.Errorf("invalid Ethereum address format")
	}
	
	// Validate checksum
	addr := common.HexToAddress(address)
	if addr.Hex() != address && !strings.EqualFold(addr.Hex(), address) {
		return fmt.Errorf("invalid address checksum")
	}
	
	return nil
}

// GetBalance gets balance for address and asset
func (c *EthereumChain) GetBalance(ctx context.Context, address string, asset *domain.Asset) (*domain.Balance, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is required")
	}

	addr := common.HexToAddress(address)

	// Native ETH balance
	if asset.Type == domain.AssetTypeNative {
		balance, err := c.client.BalanceAt(ctx, addr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get ETH balance: %w", err)
		}

		return &domain.Balance{
			Address:  address,
			Asset:    asset,
			Amount:   balance,
			Decimals: 18, // ETH has 18 decimals
		}, nil
	}

	// ERC-20 token balance
	if asset.Type == domain.AssetTypeToken {
		balance, err := c.getERC20Balance(ctx, address, asset)
		if err != nil {
			return nil, err
		}

		return &domain.Balance{
			Address:  address,
			Asset:    asset,
			Amount:   balance,
			Decimals: asset.Decimals,
		}, nil
	}

	return nil, fmt.Errorf("unsupported asset type: %s", asset.Type)
}

// EstimateFee estimates transaction fee
func (c *EthereumChain) EstimateFee(ctx context.Context, req *domain.TransactionRequest) (*domain.Fee, error) {
	if req.Asset == nil {
		return nil, fmt.Errorf("asset is required")
	}

	// Get current gas price
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	// Cap gas price
	if gasPrice.Cmp(c.config.MaxGasPrice) > 0 {
		gasPrice = c.config.MaxGasPrice
	}

	// Determine gas limit
	var gasLimit uint64
	if req.Asset.Type == domain.AssetTypeNative {
		gasLimit = c.config.GasLimitETH
	} else {
		gasLimit = c.config.GasLimitERC20
	}

	// Calculate fee
	fee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	gasLimitInt := int64(gasLimit)

	return &domain.Fee{
		Amount:   fee,
		Currency: "ETH", // Always ETH for gas
		GasLimit: &gasLimitInt,
		GasPrice: gasPrice,
	}, nil
}

// Send sends a transaction
func (c *EthereumChain) Send(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	if req.Asset == nil {
		return nil, fmt.Errorf("asset is required")
	}

	c.logger.Info("Sending Ethereum transaction",
		zap.String("from", req.From),
		zap.String("to", req.To),
		zap.String("asset", req.Asset.Symbol),
		zap.String("amount", req.Amount.String()))

	// Native ETH transfer
	if req.Asset.Type == domain.AssetTypeNative {
		return c.sendETH(ctx, req)
	}

	// ERC-20 token transfer
	if req.Asset.Type == domain.AssetTypeToken {
		return c.sendERC20(ctx, req)
	}

	return nil, fmt.Errorf("unsupported asset type: %s", req.Asset.Type)
}

// GetTransaction gets transaction status

func (c *EthereumChain) GetTransaction(ctx context.Context, txHash string) (*domain.Transaction, error) {
	hash := common.HexToHash(txHash)

	// Get transaction
	tx, isPending, err := c.client.TransactionByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	// Build transaction object
	transaction := &domain.Transaction{
		Hash:   txHash,
		Chain:  c.Name(),
		Amount: tx.Value(),
		Fee:    new(big.Int).Mul(tx.GasPrice(), big.NewInt(int64(tx.Gas()))),
	}

	// Set To address (might be nil for contract creation)
	if tx.To() != nil {
		transaction.To = tx.To().Hex()
	}

	//  Get sender using Sender() instead of AsMessage()
	signer := types.LatestSignerForChainID(c.config.ChainID)
	sender, err := types.Sender(signer, tx)
	if err == nil {
		transaction.From = sender.Hex()
	} else {
		c.logger.Warn("Failed to recover sender",
			zap.String("tx_hash", txHash),
			zap.Error(err))
	}

	// If still pending
	if isPending {
		transaction.Status = domain.TxStatusPending
		transaction.Confirmations = 0
		return transaction, nil
	}

	// Get receipt
	receipt, err := c.client.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %w", err)
	}

	// Set block details
	blockNum := receipt.BlockNumber.Int64()
	transaction.BlockNumber = &blockNum

	// Get confirmations
	currentBlock, err := c.client.BlockNumber(ctx)
	if err == nil {
		transaction.Confirmations = int(currentBlock - receipt.BlockNumber.Uint64())
	}

	// Set status based on receipt status and confirmations
	if receipt.Status == types.ReceiptStatusSuccessful {
		if transaction.Confirmations >= c.config.Confirmations {
			transaction.Status = domain.TxStatusConfirmed
		} else {
			transaction.Status = domain.TxStatusPending
		}
	} else {
		transaction.Status = domain.TxStatusFailed
	}

	// Get block timestamp
	if block, err := c.client.BlockByNumber(ctx, receipt.BlockNumber); err == nil {
		transaction.Timestamp = time.Unix(int64(block.Time()), 0)
	}

	// Update fee with actual gas used
	transaction.Fee = new(big.Int).Mul(tx.GasPrice(), big.NewInt(int64(receipt.GasUsed)))

	c.logger.Info("Transaction retrieved",
		zap.String("tx_hash", txHash),
		zap.String("from", transaction.From),
		zap.String("to", transaction.To),
		zap.String("status", string(transaction.Status)),
		zap.Int("confirmations", transaction.Confirmations))

	return transaction, nil
}
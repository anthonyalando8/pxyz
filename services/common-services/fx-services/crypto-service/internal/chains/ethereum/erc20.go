// internal/chains/ethereum/erc20.go
package ethereum

import (
	"context"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"
)

// ERC-20 ABI for balanceOf and transfer functions
const erc20ABI = `[
	{
		"constant": true,
		"inputs": [{"name": "_owner", "type": "address"}],
		"name": "balanceOf",
		"outputs": [{"name": "balance", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{"name": "_to", "type": "address"},
			{"name": "_value", "type": "uint256"}
		],
		"name": "transfer",
		"outputs": [{"name": "", "type": "bool"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [{"name": "", "type": "uint8"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "symbol",
		"outputs": [{"name": "", "type": "string"}],
		"type": "function"
	}
]`

// getERC20Balance gets ERC-20 token balance
// internal/chains/ethereum/erc20.go

// getERC20Balance gets ERC-20 token balance
func (c *EthereumChain) getERC20Balance(ctx context.Context, address string, asset *domain.Asset) (*big.Int, error) {
	if asset.ContractAddr == nil {
		return nil, fmt.Errorf("contract address required for token")
	}

	// Parse ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Pack balanceOf function call
	data, err := parsedABI.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf: %w", err)
	}

	// Call contract
	contractAddr := common.HexToAddress(*asset.ContractAddr)
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %w", err)
	}

	// Check if result is empty (address has no balance/never interacted with token)
	if len(result) == 0 {
		c.logger.Debug("Empty result from balanceOf call (address likely has no tokens)",
			zap.String("address", address),
			zap.String("token", asset.Symbol))
		return big.NewInt(0), nil // Return 0 balance instead of error
	}

	// Unpack result
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack balance: %w", err)
	}

	// Handle nil balance
	if balance == nil {
		balance = big.NewInt(0)
	}

	c.logger.Debug("ERC-20 balance retrieved",
		zap.String("address", address),
		zap.String("token", asset.Symbol),
		zap.String("balance", balance.String()))

	return balance, nil
}

// sendERC20 sends ERC-20 token
func (c *EthereumChain) sendERC20(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	if req.Asset.ContractAddr == nil {
		return nil, fmt.Errorf("contract address required for token transfer")
	}

	c.logger.Info("Sending ERC-20 token",
		zap.String("from", req.From),
		zap.String("to", req.To),
		zap.String("token", req.Asset.Symbol),
		zap.String("amount", req.Amount.String()),
		zap.String("contract", *req.Asset.ContractAddr))

	fromAddr := common.HexToAddress(req.From)

	// Get nonce
	nonce, err := c.client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	// Apply priority
	gasPrice = c.applyPriority(gasPrice, req.Priority)

	// Cap gas price
	if gasPrice.Cmp(c.config.MaxGasPrice) > 0 {
		gasPrice = c.config.MaxGasPrice
	}

	// Parse ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Pack transfer function call
	toAddr := common.HexToAddress(req.To)
	data, err := parsedABI.Pack("transfer", toAddr, req.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to pack transfer: %w", err)
	}

	// Create transaction
	contractAddr := common.HexToAddress(*req.Asset.ContractAddr)
	tx := types.NewTransaction(
		nonce,
		contractAddr,
		big.NewInt(0), // Value is 0 for ERC-20 transfer
		c.config.GasLimitERC20,
		gasPrice,
		data, // Transfer function call
	)

	// Sign transaction
	signedTx, err := signTransaction(tx, req.PrivateKey, c.config.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	if err := c.client.SendTransaction(ctx, signedTx); err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	fee := new(big.Int).Mul(gasPrice, big.NewInt(int64(c.config.GasLimitERC20)))

	c.logger.Info("ERC-20 transaction sent",
		zap.String("tx_hash", txHash),
		zap.String("token", req.Asset.Symbol),
		zap.String("fee", fee.String()))

	return &domain.TransactionResult{
		TxHash:    txHash,
		Status:    domain.TxStatusPending,
		Fee:       fee,
		Timestamp: time.Now(),
	}, nil
}
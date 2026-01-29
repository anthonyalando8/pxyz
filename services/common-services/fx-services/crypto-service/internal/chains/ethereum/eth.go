// internal/chains/ethereum/eth.go
package ethereum

import (
	"context"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"
)

func (c *EthereumChain) sendETH(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	c.logger.Info("Sending ETH",
		zap.String("from", req.From),
		zap.String("to", req.To),
		zap.String("amount", req.Amount.String()))

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

	// Apply priority multiplier
	gasPrice = c.applyPriority(gasPrice, req.Priority)

	// Cap gas price
	if gasPrice.Cmp(c.config.MaxGasPrice) > 0 {
		gasPrice = c.config.MaxGasPrice
	}

	// Create transaction
	toAddr := common.HexToAddress(req.To)
	tx := types.NewTransaction(
		nonce,
		toAddr,
		req.Amount,
		c.config.GasLimitETH,
		gasPrice,
		nil, // No data for ETH transfer
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
	fee := new(big.Int).Mul(gasPrice, big.NewInt(int64(c.config.GasLimitETH)))

	c.logger.Info("ETH transaction sent",
		zap.String("tx_hash", txHash),
		zap.String("fee", fee.String()))

	return &domain.TransactionResult{
		TxHash:    txHash,
		Status:    domain.TxStatusPending,
		Fee:       fee,
		Timestamp: time.Now(),
	}, nil
}

// applyPriority adjusts gas price based on priority
func (c *EthereumChain) applyPriority(gasPrice *big.Int, priority domain.TxPriority) *big.Int {
	multiplier := big.NewFloat(1.0)

	switch priority {
	case domain.TxPriorityLow:
		multiplier = big.NewFloat(0.8) // 80%
	case domain.TxPriorityNormal:
		multiplier = big.NewFloat(1.0) // 100%
	case domain.TxPriorityHigh:
		multiplier = big.NewFloat(1.5) // 150%
	}

	result := new(big.Float).Mul(new(big.Float).SetInt(gasPrice), multiplier)
	adjusted, _ := result.Int(nil)

	return adjusted
}
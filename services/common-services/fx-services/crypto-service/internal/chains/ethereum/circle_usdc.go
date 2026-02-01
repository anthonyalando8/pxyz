// internal/chains/ethereum/circle_usdc.go
package ethereum

import (
	"context"
	"crypto-service/internal/chains/ethereum/circle"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"time"

	"go.uber.org/zap"
)

// getCircleUSDCBalance gets USDC balance via Circle API
func (c *EthereumChain) getCircleUSDCBalance(ctx context.Context, walletID string, asset *domain.Asset) (*domain.Balance, error) {
	c.logger.Debug("Getting USDC balance via Circle",
		zap.String("wallet_id", walletID))

	balance, err := c.circleClient.GetWalletBalance(ctx, walletID)
	if err != nil {
		c.logger.Warn("Circle balance check failed, falling back to ERC-20",
			zap.Error(err))
		// Fallback to ERC-20
		amount, err := c.getERC20Balance(ctx, walletID, asset)
		if err != nil {
			return nil, err
		}
		return &domain.Balance{
			Address:  walletID,
			Asset:    asset,
			Amount:   amount,
			Decimals: 18,
		}, nil
	}

	// Find USDC balance
	for _, tb := range balance.TokenBalances {
		if tb.Token.Symbol == "USDC" {
			amount := new(big.Float)
			amount.SetString(tb.Amount)

			// Convert to smallest unit (6 decimals)
			multiplier := new(big.Float).SetInt(big.NewInt(1000000))
			amount.Mul(amount, multiplier)

			result, _ := amount.Int(nil)

			return &domain.Balance{
				Address:  walletID,
				Asset:    asset,
				Amount:   result,
				Decimals: 6,
			}, nil
		}
	}

	return &domain.Balance{
		Address:  walletID,
		Asset:    asset,
		Amount:   big.NewInt(0),
		Decimals: 6,
	}, nil
}

// sendCircleUSDC sends USDC via Circle API
func (c *EthereumChain) sendCircleUSDC(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	c.logger.Info("Sending USDC via Circle",
		zap.String("from_wallet_id", req.From),
		zap.String("to_address", req.To),
		zap.String("amount", req.Amount.String()))

	// Convert amount to USDC (human-readable)
	amountFloat := new(big.Float).SetInt(req.Amount)
	amountFloat.Quo(amountFloat, big.NewFloat(1000000))
	amountStr := amountFloat.Text('f', 6)

	// Create transfer request
	transferReq := &circle.TransferRequest{
		IdempotencyKey: fmt.Sprintf("tx-%d", time.Now().UnixNano()),
		WalletID:       req.PrivateKey, // Circle wallet ID stored as "private key"
		ToAddress:      req.To,
		Amount:         amountStr,
	}

	transfer, err := c.circleClient.CreateTransfer(ctx, transferReq)
	if err != nil {
		c.logger.Error("Circle transfer failed, falling back to ERC-20",
			zap.Error(err))
		// Fallback to ERC-20
		return c.sendERC20(ctx, req)
	}

	status := domain.TxStatusPending
	if transfer.State == "COMPLETE" {
		status = domain.TxStatusConfirmed
	}

	c.logger.Info("Circle USDC transfer successful",
		zap.String("transfer_id", transfer.ID),
		zap.String("tx_hash", transfer.TransactionHash))

	return &domain.TransactionResult{
		TxHash:    transfer.TransactionHash,
		Status:    status,
		Fee:       big.NewInt(0), // Circle handles fees
		Timestamp: time.Now(),
	}, nil
}

// CreateCircleWallet creates a Circle wallet for USDC (called separately)
func (c *EthereumChain) CreateCircleWallet(ctx context.Context, userID string) (*domain.Wallet, error) {
	if !c.config.CircleEnabled {
		return nil, fmt.Errorf("Circle is not enabled")
	}

	c.logger.Info("Creating Circle wallet for USDC",
		zap.String("user_id", userID))

	wallet, err := c.circleClient.CreateWallet(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Circle wallet: %w", err)
	}

	// Return wallet with Circle wallet ID stored as "private key"
	return &domain.Wallet{
		Address:    wallet.Address, // Ethereum address managed by Circle
		PrivateKey: wallet.ID,      // Circle wallet ID (not actual private key)
		PublicKey:  wallet.Address, // Same as address
		Chain:      "ETHEREUM",
		CreatedAt:  time.Now(),
	}, nil
}

func (c *EthereumChain) GetCircleTransaction(ctx context.Context, txHash string) (*domain.Transaction, error) {
	// txHash is actually the Circle transaction ID
	tx, err := c.circleClient.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, err
	}

	status := domain.TxStatusPending
	switch tx.State {
	case "COMPLETE":
		status = domain.TxStatusConfirmed
	case "FAILED":
		status = domain.TxStatusFailed
	}

	return &domain.Transaction{
		Hash:      tx.TransactionHash,
		Chain:     "ETHEREUM",
		Status:    status,
		Timestamp: parseCircleDate(tx.CreateDate),
	}, nil
}

func parseCircleDate(dateStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, dateStr)
	return t
}

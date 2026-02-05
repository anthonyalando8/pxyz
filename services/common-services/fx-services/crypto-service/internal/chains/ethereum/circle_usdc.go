// internal/chains/ethereum/circle_usdc.go
package ethereum

import (
	"context"
	"crypto-service/internal/chains/ethereum/circle"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
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
			Decimals: 6,
		}, nil
	}

	// Find USDC balance
	for _, tb := range balance.TokenBalances {
		if tb.Token.Symbol == "USDC" {
			amount := new(big.Float)
			amount.SetString(tb.Amount)

			// Convert to smallest unit (6 decimals for USDC)
			multiplier := new(big.Float).SetInt(big.NewInt(1000000))
			amount.Mul(amount, multiplier)

			result, _ := amount.Int(nil)

			c.logger.Debug("USDC balance retrieved from Circle",
				zap.String("wallet_id", walletID),
				zap.String("balance", result.String()),
				zap.String("formatted", tb.Amount))

			return &domain.Balance{
				Address:  walletID,
				Asset:    asset,
				Amount:   result,
				Decimals: 6,
			}, nil
		}
	}

	// No USDC balance found
	c.logger.Debug("No USDC balance found in Circle wallet",
		zap.String("wallet_id", walletID))

	return &domain.Balance{
		Address:  walletID,
		Asset:    asset,
		Amount:   big.NewInt(0),
		Decimals: 6,
	}, nil
}

// sendCircleUSDC sends USDC via Circle API
// internal/chains/ethereum/circle_usdc.go

func (c *EthereumChain) sendCircleUSDC(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	c.logger.Info("Sending USDC via Circle",
		zap.String("from_wallet_id", req.From),
		zap.String("to_address", req.To),
		zap.String("amount", req.Amount.String()))

	// Convert amount to USDC (human-readable)
	amountFloat := new(big.Float).SetInt(req.Amount)
	amountFloat.Quo(amountFloat, big.NewFloat(1000000))
	amountStr := amountFloat.Text('f', 6)

	c.logger.Debug("Transfer amount formatted",
		zap.String("raw_amount", req.Amount.String()),
		zap.String("formatted_amount", amountStr))

	//  Get USDC token ID dynamically from wallet balance
	tokenID, err := c.getUSDCTokenID(ctx, req.PrivateKey)
	if err != nil {
		c.logger.Error("Failed to get USDC token ID", zap.Error(err))
		return nil, fmt.Errorf("failed to get USDC token ID: %w", err)
	}

	// Create transfer request
	transferReq := &circle.TransferRequest{
		IdempotencyKey: uuid.New().String(),
		WalletID:       req.PrivateKey, // Circle wallet ID
		ToAddress:      req.To,
		Amount:         amountStr,
		TokenID:        tokenID, //  Use correct token ID
	}

	transfer, err := c.circleClient.CreateTransfer(ctx, transferReq)
	if err != nil {
		c.logger.Error("Circle transfer failed",
			zap.Error(err),
			zap.String("wallet_id", req.PrivateKey),
			zap.String("to_address", req.To))
		return nil, fmt.Errorf("Circle transfer failed: %w", err)
	}

	// Map Circle state to domain status
	status := domain.TxStatusPending
	switch transfer.State {
	case "COMPLETE", "CONFIRMED":
		status = domain.TxStatusConfirmed
	case "CANCELLED", "FAILED", "DENIED":
		status = domain.TxStatusFailed
	default:
		status = domain.TxStatusPending
	}

	c.logger.Info("Circle USDC transfer created",
		zap.String("transfer_id", transfer.ID),
		zap.String("state", transfer.State),
		zap.String("status", string(status)))

	return &domain.TransactionResult{
		TxHash:    transfer.ID, // Use Circle transaction ID initially
		Status:    status,
		Fee:       big.NewInt(0), // Circle handles fees internally
		Timestamp: time.Now(),
	}, nil
}

//  getUSDCTokenID gets the correct USDC token ID from wallet balance
func (c *EthereumChain) getUSDCTokenID(ctx context.Context, walletID string) (string, error) {
	balance, err := c.circleClient.GetWalletBalance(ctx, walletID)
	if err != nil {
		return "", fmt.Errorf("failed to get wallet balance: %w", err)
	}

	// Find USDC token
	for _, tb := range balance.TokenBalances {
		if tb.Token.Symbol == "USDC" {
			c.logger.Debug("Found USDC token ID",
				zap.String("token_id", tb.Token.ID),
				zap.String("blockchain", tb.Token.Blockchain),
				zap.String("token_address", tb.Token.TokenAddress))
			return tb.Token.ID, nil
		}
	}

	return "", fmt.Errorf("USDC token not found in wallet")
}

// createCircleWallet creates a Circle wallet for USDC
func (c *EthereumChain) createCircleWallet(ctx context.Context, userID string) (*domain.Wallet, error) {
	if !c.config.CircleEnabled {
		return nil, fmt.Errorf("Circle is not enabled")
	}

	c.logger.Info("Creating Circle wallet for USDC",
		zap.String("user_id", userID))

	wallet, err := c.circleClient.CreateWallet(ctx, userID)
	if err != nil {
		c.logger.Error("Failed to create Circle wallet",
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create Circle wallet: %w", err)
	}

	c.logger.Info("Circle wallet created successfully",
		zap.String("wallet_id", wallet.ID),
		zap.String("address", wallet.Address),
		zap.String("blockchain", wallet.Blockchain),
		zap.String("state", wallet.State))

	// Return wallet with Circle wallet ID stored as "private key"
	return &domain.Wallet{
		Address:    wallet.Address,    // Ethereum address managed by Circle
		PrivateKey: wallet.ID,         // Circle wallet ID (not actual private key)
		PublicKey:  wallet.Address,    // Same as address
		Chain:      "ETHEREUM",
		CreatedAt:  time.Now(),
	}, nil
}

// getCircleTransaction gets Circle transaction details
func (c *EthereumChain) getCircleTransaction(ctx context.Context, txID string) (*domain.Transaction, error) {
	c.logger.Debug("Getting Circle transaction",
		zap.String("tx_id", txID))

	tx, err := c.circleClient.GetTransaction(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Circle transaction: %w", err)
	}

	// Map Circle state to domain status
	status := domain.TxStatusPending
	switch tx.State {
	case "COMPLETE":
		status = domain.TxStatusConfirmed
	case "CONFIRMED":
		status = domain.TxStatusConfirmed
	case "FAILED":
		status = domain.TxStatusFailed
	case "CANCELLED":
		status = domain.TxStatusFailed
	case "DENIED":
		status = domain.TxStatusFailed
	default:
		status = domain.TxStatusPending
	}

	// Parse amounts
	var amount *big.Int
	if len(tx.Amounts) > 0 {
		amountFloat := new(big.Float)
		amountFloat.SetString(tx.Amounts[0])
		multiplier := new(big.Float).SetInt(big.NewInt(1000000)) // 6 decimals for USDC
		amountFloat.Mul(amountFloat, multiplier)
		amount, _ = amountFloat.Int(nil)
	} else {
		amount = big.NewInt(0)
	}

	transaction := &domain.Transaction{
		Hash:      tx.TxHash,
		Chain:     "ETHEREUM",
		Status:    status,
		From:      tx.SourceAddress,
		To:        tx.DestinationAddress,
		Amount:    amount,
		Timestamp: parseCircleDate(tx.CreateDate),
	}

	c.logger.Debug("Circle transaction retrieved",
		zap.String("tx_id", txID),
		zap.String("tx_hash", tx.TxHash),
		zap.String("state", tx.State),
		zap.String("status", string(status)))

	return transaction, nil
}

// parseCircleDate parses Circle's RFC3339 date format
func parseCircleDate(dateStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, dateStr)
	return t
}
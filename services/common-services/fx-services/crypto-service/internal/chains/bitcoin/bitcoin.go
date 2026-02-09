// internal/chains/bitcoin/bitcoin.go
package bitcoin

import (
	"context"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"time"

	"go.uber.org/zap"
)

type BitcoinChain struct {
	client  *BitcoinClient
	network string
	logger  *zap. Logger
}

// NewBitcoinChain creates a new Bitcoin chain instance
func NewBitcoinChain(rpcURL, apiKey, network string, logger *zap.Logger) (*BitcoinChain, error) {
	// Validate network
	if network != "mainnet" && network != "testnet" && network != "regtest" {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	// Warn about mainnet
	if network == "mainnet" {
		logger.Warn("⚠️  BITCOIN MAINNET ACTIVE - TRANSACTIONS USE REAL BTC")
	} else {
		logger.Info("✅ Using Bitcoin testnet", zap.String("network", network))
	}

	client := NewBitcoinClient(rpcURL, apiKey, network, logger)

	logger.Info("Bitcoin chain initialized",
		zap.String("network", network),
		zap.String("rpc_url", rpcURL))

	return &BitcoinChain{
		client:  client,
		network: network,
		logger:  logger,
	}, nil
}

// Name returns the chain name
func (b *BitcoinChain) Name() string {
	return "BITCOIN"
}

// Symbol returns native coin symbol
func (b *BitcoinChain) Symbol() string {
	return "BTC"
}

// GenerateWallet creates a new Bitcoin wallet
func (b *BitcoinChain) GenerateWallet(ctx context.Context) (*domain.Wallet, error) {

	wallet, err := GenerateBitcoinWallet(b.network)
	if err != nil {
		return nil, fmt.Errorf("failed to generate wallet: %w", err)
	}

	return wallet, nil
}

// ImportWallet imports a wallet from private key
func (b *BitcoinChain) ImportWallet(ctx context.Context, privateKey string) (*domain.Wallet, error) {


	wallet, err := ImportBitcoinWallet(privateKey, b.network)
	if err != nil {
		return nil, fmt.Errorf("failed to import wallet: %w", err)
	}

	b.logger.Info("Bitcoin wallet imported",
		zap. String("address", wallet.Address))

	return wallet, nil
}

// GetBalance gets balance for address
func (b *BitcoinChain) GetBalance(ctx context.Context, address string, walletID string, asset *domain.Asset) (*domain.Balance, error) {
	// Validate address
	if err := b.ValidateAddress(address); err != nil {
		return nil, err
	}

	// Bitcoin only supports native BTC
	if asset.Type != domain.AssetTypeNative {
		return nil, fmt. Errorf("Bitcoin only supports native BTC")
	}

	// Get balance in satoshis
	balanceSats, err := b.client.GetBalance(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	balance := big.NewInt(balanceSats)


	return &domain.Balance{
		Address:  address,
		Asset:    asset,
		Amount:   balance,
		Decimals:  8, // Bitcoin has 8 decimals
	}, nil
}

// EstimateFee estimates transaction fee
func (b *BitcoinChain) EstimateFee(ctx context.Context, req *domain.TransactionRequest) (*domain.Fee, error) {


	// Get current fee rate
	var confirmationTarget int
	switch req.Priority {
	case domain.TxPriorityHigh:
		confirmationTarget = 1 // Next block
	case domain.TxPriorityNormal:
		confirmationTarget = 3 // ~30 minutes
	case domain.TxPriorityLow: 
		confirmationTarget = 6 // ~1 hour
	default:
		confirmationTarget = 3
	}

	feeRateSatPerVB, err := b.client. EstimateFee(ctx, confirmationTarget)
	if err != nil {
		// Fallback to default rates
		b.logger.Warn("Failed to estimate fee, using defaults", zap.Error(err))
		switch req.Priority {
		case domain.TxPriorityHigh:
			feeRateSatPerVB = 50
		case domain.TxPriorityNormal:
			feeRateSatPerVB = 20
		default:
			feeRateSatPerVB = 10
		}
	}

	// Estimate transaction size
	// Typical P2PKH transaction: 1 input + 2 outputs ≈ 226 vBytes
	// For safety, calculate based on amount and UTXOs needed
	estimatedSize := int64(250) // Conservative estimate

	// Calculate fee
	feeSats := int64(float64(estimatedSize) * feeRateSatPerVB)

	b.logger.Info("Fee estimated",
		zap.Float64("fee_rate_sat_per_vb", feeRateSatPerVB),
		zap.Int64("estimated_size_vb", estimatedSize),
		zap.Int64("fee_sats", feeSats))

	return &domain.Fee{
		Amount:   big.NewInt(feeSats),
		Currency: "BTC",
		GasLimit: &estimatedSize,
		GasPrice: big.NewInt(int64(feeRateSatPerVB)),
	}, nil
}

// Send sends a Bitcoin transaction
func (b *BitcoinChain) Send(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {

	// Validate addresses
	if err := b.ValidateAddress(req.From); err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}
	if err := b.ValidateAddress(req.To); err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	// Get UTXOs for sender
	utxos, err := b.client.GetUTXOs(ctx, req. From)
	if err != nil {
		return nil, fmt. Errorf("failed to get UTXOs: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("no UTXOs available")
	}

	// Estimate fee
	feeEstimate, err := b.EstimateFee(ctx, req)
	if err != nil {
		return nil, fmt. Errorf("failed to estimate fee:  %w", err)
	}

	// Create transaction builder
	txBuilder, err := NewTransactionBuilder(b.network)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction builder: %w", err)
	}

	// Select UTXOs and add inputs
	amountSats := req.Amount. Int64()
	feeSats := feeEstimate. Amount.Int64()
	totalNeeded := amountSats + feeSats

	var totalInput int64
	for _, utxo := range utxos {
		if ! utxo.Status. Confirmed {
			continue // Skip unconfirmed UTXOs
		}

		if err := txBuilder.AddInput(utxo, req. PrivateKey); err != nil {
			return nil, fmt. Errorf("failed to add input:  %w", err)
		}

		totalInput += utxo.Value

		if totalInput >= totalNeeded {
			break
		}
	}

	if totalInput < totalNeeded {
		return nil, fmt.Errorf("insufficient balance:  have %d sats, need %d sats", totalInput, totalNeeded)
	}

	// Add recipient output
	if err := txBuilder. AddOutput(req.To, amountSats); err != nil {
		return nil, fmt. Errorf("failed to add recipient output: %w", err)
	}

	// Add change output if necessary
	change := totalInput - amountSats - feeSats
	if change > 546 { // Dust limit is 546 satoshis
		if err := txBuilder.AddOutput(req.From, change); err != nil {
			return nil, fmt.Errorf("failed to add change output:  %w", err)
		}
	}

	// Sign transaction
	if err := txBuilder.Sign(); err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Serialize transaction
	rawTx, err := txBuilder. Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// Get transaction hash before broadcasting
	txHash := txBuilder.GetTxHash()

	b.logger.Info("Transaction built and signed",
		zap.String("tx_hash", txHash),
		zap.Int64("total_input", totalInput),
		zap.Int64("amount", amountSats),
		zap.Int64("fee", feeSats),
		zap.Int64("change", change))

	// Broadcast transaction
	broadcastedHash, err := b.client.BroadcastTransaction(ctx, rawTx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	b.logger. Info("Bitcoin transaction broadcast successfully",
		zap.String("tx_hash", broadcastedHash))

	return &domain.TransactionResult{
		TxHash:    broadcastedHash,
		Status:    domain.TxStatusPending,
		Fee:       big.NewInt(feeSats),
		Timestamp: time.Now(),
	}, nil
}

// GetTransaction gets transaction details
func (b *BitcoinChain) GetTransaction(ctx context.Context, txHash string) (*domain.Transaction, error) {
	b.logger.Info("Getting Bitcoin transaction",
		zap.String("tx_hash", txHash))

	txInfo, err := b.client.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Build domain transaction
	tx := &domain.Transaction{
		Hash:  txInfo.TxID,
		Chain: "BITCOIN",
		Fee:   big.NewInt(txInfo.Fee),
		Asset: &domain.Asset{
			Chain:    "BITCOIN",
			Symbol:   "BTC",
			Type:     domain.AssetTypeNative,
			Decimals: 8,
		},
	}

	// Set status
	if txInfo.Status. Confirmed {
		tx.Status = domain.TxStatusConfirmed
		blockNum := txInfo.Status.BlockHeight
		tx.BlockNumber = &blockNum
		tx. Timestamp = time.Unix(txInfo.Status.BlockTime, 0)

		// Get current block height to calculate confirmations
		info, err := b.client.GetBlockchainInfo(ctx)
		if err == nil {
			confirmations := int(info. Blocks - txInfo.Status.BlockHeight + 1)
			tx.Confirmations = confirmations
		}
	} else {
		tx.Status = domain.TxStatusPending
		tx.Confirmations = 0
	}

	// Extract from/to addresses and amount
	if len(txInfo. Vin) > 0 && ! txInfo.Vin[0].IsCoinbase {
		tx.From = txInfo.Vin[0]. Prevout.ScriptPubKeyAddress
	}

	if len(txInfo. Vout) > 0 {
		tx.To = txInfo.Vout[0].ScriptPubKeyAddress
		tx.Amount = big.NewInt(txInfo.Vout[0].Value)
	}

	b.logger.Info("Bitcoin transaction retrieved",
		zap.String("tx_hash", txHash),
		zap.String("status", string(tx.Status)),
		zap.Int("confirmations", tx.Confirmations))

	return tx, nil
}

// ValidateAddress validates Bitcoin address format
func (b *BitcoinChain) ValidateAddress(address string) error {
	return ValidateBitcoinAddress(address, b.network)
}

// Stop gracefully stops the Bitcoin chain client
func (b *BitcoinChain) Stop() error {
	b.logger.Info("Bitcoin chain stopped")
	return nil
}
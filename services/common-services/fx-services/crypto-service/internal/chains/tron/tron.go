// internal/chains/tron/tron. go
package tron

import (
	"context"
	"crypto-service/internal/domain"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type TronChain struct {
	grpcClient *client.GrpcClient
	httpClient *TronHTTPClient
	network    string
	logger     *zap.Logger
}

func NewTronChain(apiKey, network string, logger *zap.Logger) (*TronChain, error) {
	var grpcURL, httpURL string

	switch network {
	case "mainnet":
		grpcURL = "grpc.trongrid.io:50051"
		httpURL = "https://api.trongrid.io"
	case "shasta": 
		grpcURL = "grpc.shasta.trongrid.io:50051"
		httpURL = "https://api.shasta.trongrid.io"
	case "nile":
		grpcURL = "grpc.nile.trongrid.io:50051"
		httpURL = "https://api.nile.trongrid.io"
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	// Warn about mainnet
	if network == "mainnet" {
		logger.Warn("⚠️  MAINNET ACTIVE - TRANSACTIONS USE REAL TRX")
	} else {
		logger.Info("✅ Using testnet", zap.String("network", network))
	}

	// ✅ Initialize gRPC client with insecure credentials
	grpcClient := client. NewGrpcClient(grpcURL)
	grpcClient.SetAPIKey(apiKey)

	// ✅ Set insecure credentials (required for gRPC)
	if err := grpcClient.Start(grpc.WithTransportCredentials(insecure.NewCredentials())); err != nil {
		return nil, fmt.Errorf("failed to start TRON gRPC client: %w", err)
	}

	// Initialize HTTP client
	httpClient := NewTronHTTPClient(httpURL, apiKey, logger)

	logger.Info("TRON chain initialized",
		zap.String("network", network),
		zap.String("grpc_url", grpcURL),
		zap.String("http_url", httpURL))

	return &TronChain{
		grpcClient: grpcClient,
		httpClient: httpClient,
		network:    network,
		logger:     logger,
	}, nil
}

// Stop gracefully stops the TRON chain client
func (t *TronChain) Stop() error {
	if t.grpcClient != nil {
		t.grpcClient.Stop()
		t.logger.Info("TRON gRPC client stopped")
	}
	return nil
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
		zap.String("address", wallet.Address))

	return wallet, nil
}

// GetBalance implementation with TRC20 support
func (t *TronChain) GetBalance(ctx context.Context, address string, asset *domain.Asset) (*domain.Balance, error) {
	// Validate address
	if err := t.ValidateAddress(address); err != nil {
		return nil, err
	}

	// Route to appropriate handler
	if asset.Type == domain.AssetTypeNative {
		return t.getTRXBalance(ctx, address, asset)
	}

	return t.getTRC20Balance(ctx, address, asset)
}

// getTRXBalance gets native TRX balance
// getTRXBalance gets native TRX balance using HTTP API
func (t *TronChain) getTRXBalance(ctx context. Context, addr string, asset *domain.Asset) (*domain.Balance, error) {
	t.logger.Info("getting TRX balance via HTTP",
		zap.String("address", addr))

	// ✅ Use HTTP API (most reliable)
	accountInfo, err := t.httpClient.GetAccountInfo(ctx, addr)
	if err != nil {
		// Account might not exist yet (no transactions)
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			t.logger.Info("account not found, returning zero balance",
				zap. String("address", addr))
			
			return &domain.Balance{
				Address:  addr,
				Asset:    asset,
				Amount:   big.NewInt(0),
				Decimals: 6,
			}, nil
		}
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	balance := big.NewInt(accountInfo.Balance)

	t.logger.Info("TRX balance retrieved",
		zap.String("address", addr),
		zap.String("balance", balance.String()))

	return &domain.Balance{
		Address:  addr,
		Asset:    asset,
		Amount:   balance,
		Decimals: 6,
	}, nil
}

// Send implementation with TRC20 support
func (t *TronChain) Send(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	// Validate addresses
	if err := t.ValidateAddress(req.From); err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}
	if err := t.ValidateAddress(req.To); err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	// Route to appropriate handler
	if req.Asset.Type == domain.AssetTypeNative {
		return t.sendTRX(ctx, req)
	}

	return t.sendTRC20(ctx, req)
}

// sendTRX sends native TRX
// internal/chains/tron/tron_complete.go (UPDATE sendTRX)
// internal/chains/tron/tron_complete.go (UPDATE sendTRX)

// sendTRX sends native TRX
func (t *TronChain) sendTRX(ctx context.Context, req *domain. TransactionRequest) (*domain.TransactionResult, error) {
	t.logger.Info("sending TRX transaction",
		zap.String("from", req.From),
		zap.String("to", req.To),
		zap.String("amount", req.Amount.String()))

	// ✅ Use the addresses as Base58 strings directly
	// The SDK should handle the conversion internally
	tx, err := t.grpcClient.Transfer(req.From, req.To, req.Amount.Int64())
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	if tx == nil || tx.Transaction == nil {
		return nil, fmt.Errorf("transaction creation returned empty result")
	}

	// Check transaction result
	if tx.Result != nil && tx.Result.Code != 0 {
		return nil, fmt.Errorf("transaction creation failed: %s", string(tx.Result.Message))
	}

	// Sign transaction
	signedTx, err := t.signTransaction(tx.Transaction, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Calculate hash
	txHash, err := getTxHash(signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction hash:  %w", err)
	}

	// Broadcast
	result, err := t.grpcClient.Broadcast(signedTx)
	if err != nil {
		return nil, fmt. Errorf("failed to broadcast: %w", err)
	}

	if !result.Result {
		return nil, fmt.Errorf("broadcast failed: %s", string(result.Message))
	}

	t.logger.Info("TRX transaction sent successfully",
		zap.String("tx_hash", txHash),
		zap.String("from", req.From),
		zap.String("to", req.To),
		zap.String("amount", req.Amount.String()))

	return &domain.TransactionResult{
		TxHash:    txHash,
		Status:    domain.TxStatusPending,
		Fee:       big.NewInt(0),
		Timestamp: time.Now(),
	}, nil
}

// EstimateFee with TRC20 support
func (t *TronChain) EstimateFee(ctx context.Context, req *domain.TransactionRequest) (*domain.Fee, error) {
	if req.Asset.Type == domain.AssetTypeNative {
		// TRX transfers are usually free (using bandwidth)
		return &domain.Fee{
			Amount:   big.NewInt(0),
			Currency: "TRX",
		}, nil
	}

	return t.estimateTRC20Fee(ctx, req)
}

// ValidateAddress validates TRON address format
func (t *TronChain) ValidateAddress(addr string) error {
	// TRON addresses start with 'T' and are 34 characters
	if !strings.HasPrefix(addr, "T") {
		return fmt.Errorf("invalid TRON address:  must start with 'T'")
	}

	if len(addr) != 34 {
		return fmt.Errorf("invalid TRON address:  must be 34 characters")
	}

	// Try to decode
	_, err := address.Base58ToAddress(addr)
	if err != nil {
		return fmt.Errorf("invalid TRON address: %w", err)
	}

	return nil
}

// GetTransaction gets transaction details and status
func (t *TronChain) GetTransaction(ctx context.Context, txHash string) (*domain.Transaction, error) {
	t.logger.Info("getting transaction",
		zap.String("tx_hash", txHash),
		zap.String("chain", "TRON"))

	// Get transaction info (for status, fees, block info)
	txInfo, err := t.grpcClient.GetTransactionInfoByID(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction info: %w", err)
	}

	// Get transaction details (for from, to, amount)
	tx, err := t.grpcClient.GetTransactionByID(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Build result
	result := &domain.Transaction{
		Hash:  txHash,
		Chain: "TRON",
	}

	// Determine transaction type and parse accordingly
	if tx.RawData != nil && len(tx.RawData.Contract) > 0 {
		contract := tx.RawData.Contract[0]

		switch contract.Type {
		case core.Transaction_Contract_TransferContract:
			// Native TRX transfer
			t.parseTRXTransaction(result, contract, txInfo)

		case core.Transaction_Contract_TriggerSmartContract:
			// Smart contract call (TRC20, TRC721, etc.)
			t.parseTRC20Transaction(result, contract, txInfo)

		default:
			t.logger.Warn("unsupported transaction type",
				zap.String("type", contract.Type.String()))
		}
	}

	// Set status
	if txInfo.Receipt != nil {
		if txInfo.Receipt.Result == core.Transaction_Result_SUCCESS {
			result.Status = domain.TxStatusConfirmed
		} else {
			result.Status = domain.TxStatusFailed
		}
	} else {
		result.Status = domain.TxStatusPending
	}

	// Set block info
	if txInfo.BlockNumber > 0 {
		blockNum := txInfo.BlockNumber
		result.BlockNumber = &blockNum
		result.Timestamp = time.Unix(txInfo.BlockTimeStamp/1000, 0)

		// Calculate confirmations
		currentBlock, err := t.grpcClient.GetNowBlock()
		if err == nil && currentBlock.BlockHeader != nil {
			confirmations := int(currentBlock.BlockHeader.RawData.Number - txInfo.BlockNumber)
			result.Confirmations = confirmations
		}
	}

	// Set fee
	if txInfo.Fee > 0 {
		result.Fee = big.NewInt(txInfo.Fee)
	}

	t.logger.Info("transaction retrieved",
		zap.String("tx_hash", txHash),
		zap.String("status", string(result.Status)),
		zap.Int("confirmations", result.Confirmations))

	return result, nil
}

// parseTRXTransaction parses native TRX transfer
func (t *TronChain) parseTRXTransaction(result *domain.Transaction, contract *core.Transaction_Contract, txInfo *core.TransactionInfo) {
	var transferContract core.TransferContract
	if err := proto.Unmarshal(contract.Parameter.Value, &transferContract); err != nil {
		t.logger.Error("failed to unmarshal transfer contract", zap.Error(err))
		return
	}

	// Set from/to/amount
	result.From = address.Address(transferContract.OwnerAddress).String()
	result.To = address.Address(transferContract.ToAddress).String()
	result.Amount = big.NewInt(transferContract.Amount)

	// Set asset
	result.Asset = &domain.Asset{
		Chain:    "TRON",
		Symbol:   "TRX",
		Type:     domain.AssetTypeNative,
		Decimals: 6,
	}
}

// parseTRC20Transaction parses TRC20 token transfer
func (t *TronChain) parseTRC20Transaction(result *domain.Transaction, contract *core.Transaction_Contract, txInfo *core.TransactionInfo) {
	var triggerContract core.TriggerSmartContract
	if err := proto.Unmarshal(contract.Parameter.Value, &triggerContract); err != nil {
		t.logger.Error("failed to unmarshal trigger contract", zap.Error(err))
		return
	}

	// Set from address
	result.From = address.Address(triggerContract.OwnerAddress).String()

	// Decode transaction data
	dataHex := hex.EncodeToString(triggerContract.Data)

	// Check if it's a transfer call (method signature:  a9059cbb)
	if len(dataHex) >= 8 && dataHex[:8] == "a9059cbb" {
		// Extract to address (bytes 8-72, padded to 64 hex chars)
		if len(dataHex) >= 72 {
			toHex := dataHex[32:72]                 // Skip padding, get last 40 hex chars (20 bytes)
			toBytes := common.FromHex("41" + toHex) // Add TRON prefix
			result.To = address.Address(toBytes).String()
		}

		// Extract amount (bytes 72-136)
		if len(dataHex) >= 136 {
			amountHex := dataHex[72:136]
			amountBytes := common.FromHex(amountHex)
			result.Amount = new(big.Int).SetBytes(amountBytes)
		}

		// Get contract address
		contractAddr := address.Address(triggerContract.ContractAddress).String()

		// Try to identify token (you can add a registry later)
		symbol := "UNKNOWN"
		decimals := 6

		if contractAddr == GetUSDTContract(t.network) {
			symbol = "USDT"
			decimals = 6
		}

		// Set asset
		result.Asset = &domain.Asset{
			Chain:        "TRON",
			Symbol:       symbol,
			ContractAddr: &contractAddr,
			Type:         domain.AssetTypeToken,
			Decimals:     decimals,
		}
	}
}

// GetTransactionReceipt gets transaction receipt with events
func (t *TronChain) GetTransactionReceipt(ctx context.Context, txHash string) (*TransactionReceipt, error) {
	t.logger.Info("getting transaction receipt",
		zap.String("tx_hash", txHash))

	txInfo, err := t.grpcClient.GetTransactionInfoByID(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction info: %w", err)
	}

	receipt := &TransactionReceipt{
		TxHash:      txHash,
		BlockNumber: txInfo.BlockNumber,
		Status:      txInfo.Receipt.Result.String(),
		Fee:         txInfo.Fee,
		EnergyUsed:  txInfo.Receipt.EnergyUsage,
		NetUsed:     txInfo.Receipt.NetUsage,
	}

	// Parse logs/events
	if len(txInfo.Log) > 0 {
		receipt.Logs = make([]Log, 0, len(txInfo.Log))
		for _, log := range txInfo.Log {
			receipt.Logs = append(receipt.Logs, Log{
				Address: address.Address(log.Address).String(),
				Topics:  log.Topics,
				Data:    log.Data,
			})
		}
	}

	return receipt, nil
}

type TransactionReceipt struct {
	TxHash      string
	BlockNumber int64
	Status      string
	Fee         int64
	EnergyUsed  int64
	NetUsed     int64
	Logs        []Log
}

type Log struct {
	Address string
	Topics  [][]byte
	Data    []byte
}

// WaitForConfirmation waits for transaction to be confirmed
func (t *TronChain) WaitForConfirmation(ctx context.Context, txHash string, requiredConfirmations int) (*domain.Transaction, error) {
	t.logger.Info("waiting for confirmation",
		zap.String("tx_hash", txHash),
		zap.Int("required_confirmations", requiredConfirmations))

	ticker := time.NewTicker(3 * time.Second) // Check every 3 seconds
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute) // 5 minute timeout

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for confirmation")

		case <-ticker.C:
			tx, err := t.GetTransaction(ctx, txHash)
			if err != nil {
				t.logger.Warn("failed to get transaction",
					zap.String("tx_hash", txHash),
					zap.Error(err))
				continue
			}

			// Check if failed
			if tx.Status == domain.TxStatusFailed {
				return tx, fmt.Errorf("transaction failed")
			}

			// Check confirmations
			if tx.Confirmations >= requiredConfirmations {
				t.logger.Info("transaction confirmed",
					zap.String("tx_hash", txHash),
					zap.Int("confirmations", tx.Confirmations))
				return tx, nil
			}

			t.logger.Debug("waiting for more confirmations",
				zap.String("tx_hash", txHash),
				zap.Int("current", tx.Confirmations),
				zap.Int("required", requiredConfirmations))
		}
	}
}

// GetTransactions gets multiple transactions
func (t *TronChain) GetTransactions(ctx context.Context, txHashes []string) ([]*domain.Transaction, error) {
	t.logger.Info("getting multiple transactions",
		zap.Int("count", len(txHashes)))

	results := make([]*domain.Transaction, 0, len(txHashes))

	for _, txHash := range txHashes {
		tx, err := t.GetTransaction(ctx, txHash)
		if err != nil {
			t.logger.Warn("failed to get transaction",
				zap.String("tx_hash", txHash),
				zap.Error(err))
			continue
		}
		results = append(results, tx)
	}

	return results, nil
}

// NOW IMPLEMENT GetAccountTransactions using HTTP client
func (t *TronChain) GetAccountTransactions(ctx context.Context, address string, limit int) ([]*domain.Transaction, error) {
	t.logger.Info("getting account transactions",
		zap.String("address", address),
		zap.Int("limit", limit))

	// Use HTTP client
	resp, err := t.httpClient.GetAccountTransactions(ctx, address, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	// Convert to domain transactions
	transactions := make([]*domain.Transaction, 0, len(resp.Data))

	for _, txRecord := range resp.Data {
		tx := &domain.Transaction{
			Hash:      txRecord.TxID,
			Chain:     "TRON",
			From:      txRecord.From,
			To:        txRecord.To,
			Status:    domain.TxStatusConfirmed,
			Timestamp: time.Unix(txRecord.BlockTimestamp/1000, 0),
		}

		if txRecord.BlockNumber > 0 {
			blockNum := txRecord.BlockNumber
			tx.BlockNumber = &blockNum
		}

		// Parse amount
		if txRecord.Value != "" {
			amount, ok := new(big.Int).SetString(txRecord.Value, 10)
			if ok {
				tx.Amount = amount
			}
		}

		// Set asset based on transaction type
		if txRecord.TokenInfo.Symbol != "" {
			// TRC20 token
			contractAddr := txRecord.TokenInfo.Address
			tx.Asset = &domain.Asset{
				Chain:        "TRON",
				Symbol:       txRecord.TokenInfo.Symbol,
				ContractAddr: &contractAddr,
				Type:         domain.AssetTypeToken,
				Decimals:     txRecord.TokenInfo.Decimals,
			}
		} else {
			// Native TRX
			tx.Asset = &domain.Asset{
				Chain:    "TRON",
				Symbol:   "TRX",
				Type:     domain.AssetTypeNative,
				Decimals: 6,
			}
		}

		transactions = append(transactions, tx)
	}

	t.logger.Info("account transactions retrieved",
		zap.String("address", address),
		zap.Int("count", len(transactions)))

	return transactions, nil
}

// Get account info (useful for balance checking)
func (t *TronChain) GetAccountInfo(ctx context.Context, address string) (*AccountInfo, error) {
	return t.httpClient.GetAccountInfo(ctx, address)
}

// Get TRC20 transactions specifically
func (t *TronChain) GetTRC20Transactions(ctx context.Context, address, contractAddress string, limit int) ([]TransactionRecord, error) {
	return t.httpClient.GetTRC20Transactions(ctx, address, contractAddress, limit)
}

// internal/chains/tron/trc20.go
package tron

import (
	"context"
	"crypto-service/internal/domain"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	//"strings"

	"github.com/ethereum/go-ethereum/common"
	//"github.com/ethereum/go-ethereum/crypto"
	//"github.com/fbsobreira/gotron-sdk/pkg/abi"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	//"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Well-known TRC20 contracts
const (
	USDTContractMainnet = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t" // USDT on mainnet
	USDTContractShasta  = "TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs" // USDT on testnet (Shasta)
	USDTContractNile    = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj" // USDT on testnet (Nile)
)

// TRC20Token represents a TRC20 token handler
type TRC20Token struct {
	chain           *TronChain
	contractAddress string
	decimals        int
	symbol          string
}

// NewTRC20Token creates a new TRC20 token handler
func NewTRC20Token(chain *TronChain, contractAddress, symbol string, decimals int) *TRC20Token {
	return &TRC20Token{
		chain:           chain,
		contractAddress: contractAddress,
		decimals:        decimals,
		symbol:          symbol,
	}
}

// GetUSDTContract returns USDT contract address for network
func GetUSDTContract(network string) string {
	switch network {
	case "mainnet":
		return USDTContractMainnet
	case "shasta":
		return USDTContractShasta
	case "nile":
		return USDTContractNile
	default:
		return USDTContractMainnet
	}
}

// getTRC20Balance gets TRC20 token balance using HTTP API
func (t *TronChain) getTRC20Balance(ctx context.Context, addr string, asset *domain.Asset) (*domain.Balance, error) {
	if asset.ContractAddr == nil {
		return nil, fmt.Errorf("contract address required for TRC20 tokens")
	}

	t.logger. Info("getting TRC20 balance via HTTP API",
		zap.String("address", addr),
		zap.String("contract", *asset.ContractAddr),
		zap.String("symbol", asset.Symbol))

	// ✅ Use HTTP client (most reliable)
	balanceStr, err := t.httpClient.GetTokenBalance(ctx, addr, *asset.ContractAddr)
	if err != nil {
		// ✅ Change from Warn to Debug for 404 errors
		if strings.Contains(err.Error(), "404") {
			t.logger.Debug("no token transactions yet, returning zero",
				zap.String("address", addr),
				zap.String("token", asset.Symbol))
		} else {
			t.logger. Warn("failed to get token balance",
				zap.String("address", addr),
				zap.Error(err))
		}
		balanceStr = "0"
	}

	// Parse balance
	balance, ok := new(big.Int).SetString(balanceStr, 10)
	if !ok {
		return nil, fmt.Errorf("invalid balance format:  %s", balanceStr)
	}

	t.logger. Info("TRC20 balance retrieved",
		zap.String("address", addr),
		zap.String("symbol", asset.Symbol),
		zap.String("balance", balance.String()))

	return &domain.Balance{
		Address:  addr,
		Asset:    asset,
		Amount:    balance,
		Decimals: asset.Decimals,
	}, nil
}

// sendTRC20 sends TRC20 tokens
func (t *TronChain) sendTRC20(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	if req.Asset.ContractAddr == nil {
		return nil, fmt.Errorf("contract address required for TRC20 tokens")
	}

	t.logger.Info("sending TRC20 transaction",
		zap.String("from", req.From),
		zap.String("to", req.To),
		zap.String("amount", req.Amount.String()),
		zap.String("contract", *req.Asset.ContractAddr))

	// Build transfer function call
	// transfer(address,uint256)
	functionSelector := "a9059cbb" // transfer method ID

	// ✅ Convert addresses properly
	toAddress, err := address.Base58ToAddress(req.To)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	fromAddress, err := address.Base58ToAddress(req.From)
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}

	contractAddress, err := address.Base58ToAddress(*req.Asset.ContractAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid contract address: %w", err)
	}

	// ✅ Encode parameters
	// Remove 0x41 prefix and pad to 32 bytes
	toAddrBytes := toAddress.Bytes()[1:]
	toParam := common.LeftPadBytes(toAddrBytes, 32)
	amountParam := common.LeftPadBytes(req.Amount.Bytes(), 32)

	// Build call data
	data := functionSelector + hex.EncodeToString(toParam) + hex.EncodeToString(amountParam)

	// Build transaction using the SDK method
	tx, err := t.grpcClient.TriggerContract(
		fromAddress.String(),
		contractAddress.String(),
		data,
		"0",      // feeLimit (0 = auto)
		int64(0), // callValue
		int64(0), // tokenValue
		"",       // tokenId
		int64(0), // permission_id
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}

	if tx.Result.Code != 0 {
		return nil, fmt.Errorf("transaction build failed: %s", string(tx.Result.Message))
	}

	// Sign transaction
	signedTx, err := t.signTransaction(tx.Transaction, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Calculate transaction hash
	txHash, err := getTxHash(signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction hash: %w", err)
	}

	// Broadcast transaction
	result, err := t.grpcClient.Broadcast(signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	if !result.Result {
		return nil, fmt.Errorf("broadcast failed: %s", result.Message)
	}

	t.logger.Info("TRC20 transaction sent",
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

// estimateTRC20Fee estimates TRC20 transfer fee
// internal/chains/tron/tron. go

func (t *TronChain) estimateTRC20Fee(ctx context. Context, req *domain.TransactionRequest) (*domain.Fee, error) {
	if req.Asset.ContractAddr == nil {
		return nil, fmt.Errorf("contract address required for TRC20 tokens")
	}

	t.logger.Info("Estimating TRC20 fee",
		zap.String("from", req.From),
		zap.String("to", req. To),
		zap.String("contract", *req.Asset. ContractAddr))

	// Build function call data for transfer(address,uint256)
	functionSelector := "transfer(address,uint256)"
	
	// Convert to address to hex format
	toAddress, err := address.Base58ToAddress(req.To)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}
	
	// Remove the first byte (0x41) and pad to 32 bytes
	toAddrBytes := toAddress. Bytes()[1:]
	toParam := hex.EncodeToString(common.LeftPadBytes(toAddrBytes, 32))
	
	// Pad amount to 32 bytes
	amountParam := hex.EncodeToString(common.LeftPadBytes(req.Amount. Bytes(), 32))
	
	// Combine parameters (without function selector for parameter field)
	parameter := toParam + amountParam

	// ✅ Use HTTP client instead of gRPC
	result, err := t.httpClient.TriggerConstantContract(
		ctx,
		req.From,
		*req.Asset.ContractAddr,
		functionSelector,
		parameter,
	)
	if err != nil {
		t.logger.Warn("Failed to estimate via API, using conservative estimate", 
			zap.Error(err))
		return t.getConservativeTRC20Fee(), nil
	}

	if ! result.Result.Result {
		t.logger.Warn("Contract call failed, using conservative estimate",
			zap.String("message", result.Result.Message))
		return t.getConservativeTRC20Fee(), nil
	}

	energyUsed := result.EnergyUsed
	if energyUsed == 0 {
		// Default energy for TRC20 transfer
		energyUsed = 65000
		t.logger.Info("No energy estimate returned, using default", 
			zap.Int64("default_energy", energyUsed))
	}

	// ✅ Get account resources to check available energy
	accountResources, err := t.httpClient.GetAccountResources(ctx, req.From)
	if err != nil {
		t. logger.Warn("Failed to get account resources, assuming no free energy",
			zap.Error(err))
		accountResources = &AccountResources{
			EnergyAvailable:     0,
			BandwidthAvailable: 0,
		}
	}

	// ✅ Calculate actual fee based on resources
	fee := t.calculateTRC20FeeFromEnergy(energyUsed, accountResources)

	t.logger.Info("TRC20 fee estimated",
		zap.Int64("energy_required", energyUsed),
		zap.Int64("energy_available", accountResources.EnergyAvailable),
		zap.Int64("bandwidth_available", accountResources.BandwidthAvailable),
		zap.String("fee_sun", fee.Amount.String()),
		zap.String("fee_trx", fmt.Sprintf("%.6f", float64(fee.Amount. Int64())/1000000)))

	return fee, nil
}

//  Calculate TRC20 fee based on energy and bandwidth
func (t *TronChain) calculateTRC20FeeFromEnergy(energyRequired int64, resources *AccountResources) *domain.Fee {
	const (
		energyPriceInSUN = 420  // SUN per energy unit (dynamic, but ~420 is typical)
		bandwidthCost    = 345  // Bandwidth units for TRC20
		bandwidthPrice   = 1000 // SUN per bandwidth unit
	)

	var totalFeeSUN int64

	// 1. Calculate energy cost
	if resources.EnergyAvailable >= energyRequired {
		// User has enough free energy - no energy cost
		t.logger.Info("Using free energy",
			zap.Int64("required", energyRequired),
			zap.Int64("available", resources. EnergyAvailable))
	} else {
		// Need to burn TRX for energy
		energyDeficit := energyRequired - resources. EnergyAvailable
		energyCost := energyDeficit * energyPriceInSUN
		totalFeeSUN += energyCost
		
		t.logger.Info("Energy deficit - will burn TRX",
			zap.Int64("deficit", energyDeficit),
			zap.Int64("cost_sun", energyCost),
			zap.String("cost_trx", fmt.Sprintf("%.6f", float64(energyCost)/1000000)))
	}

	// 2. Calculate bandwidth cost
	if resources.BandwidthAvailable >= bandwidthCost {
		// User has enough free bandwidth
		t.logger.Info("Using free bandwidth",
			zap.Int64("required", bandwidthCost),
			zap.Int64("available", resources.BandwidthAvailable))
	} else {
		// Need to burn TRX for bandwidth
		bandwidthDeficit := bandwidthCost - resources.BandwidthAvailable
		bwCost := bandwidthDeficit * bandwidthPrice
		totalFeeSUN += bwCost
		
		t.logger. Info("Bandwidth deficit - will burn TRX",
			zap.Int64("deficit", bandwidthDeficit),
			zap.Int64("cost_sun", bwCost),
			zap.String("cost_trx", fmt.Sprintf("%.6f", float64(bwCost)/1000000)))
	}

	// 3. Apply minimum fee (bandwidth is always needed, even with energy)
	if totalFeeSUN < 345000 {
		totalFeeSUN = 345000 // 0.345 TRX minimum (bandwidth cost)
	}

	return &domain.Fee{
		Amount:   big.NewInt(totalFeeSUN),
		Currency: "TRX",
		GasLimit:  &energyRequired,
	}
}

//  Conservative fallback estimate
func (t *TronChain) getConservativeTRC20Fee() *domain.Fee {
	// Conservative estimate assuming: 
	// - No free energy:  65,000 * 420 = 27,300,000 SUN
	// - No free bandwidth: 345 * 1000 = 345,000 SUN
	// Total: ~27.645 TRX
	// Use middle estimate: 14 TRX
	
	conservativeFee := big.NewInt(14000000) // 14 TRX
	energyEstimate := int64(65000)
	
	t.logger.Info("Using conservative TRC20 fee estimate",
		zap.String("fee_trx", "14.0"),
		zap.Int64("estimated_energy", energyEstimate))
	
	return &domain. Fee{
		Amount:   conservativeFee,
		Currency:  "TRX",
		GasLimit: &energyEstimate,
	}
}

// getTRC20Transaction gets TRC20 transaction details
func (t *TronChain) getTRC20Transaction(ctx context.Context, txHash string) (*domain.Transaction, error) {
	t.logger.Info("getting TRC20 transaction",
		zap.String("tx_hash", txHash))

	// Get transaction info
	txInfo, err := t.grpcClient.GetTransactionInfoByID(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction info: %w", err)
	}

	// Get transaction
	tx, err := t.grpcClient.GetTransactionByID(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Parse transaction
	result := &domain.Transaction{
		Hash:   txHash,
		Chain:  "TRON",
		Status: domain.TxStatusPending,
	}

	// Check if confirmed
	if txInfo.BlockNumber > 0 {
		result.Status = domain.TxStatusConfirmed
		blockNum := txInfo.BlockNumber
		result.BlockNumber = &blockNum
		result.Timestamp = time.Unix(txInfo.BlockTimeStamp/1000, 0)
	}

	// Parse contract data to get from, to, amount
	if tx.RawData != nil && len(tx.RawData.Contract) > 0 {
		contract := tx.RawData.Contract[0]
		if contract.Type == core.Transaction_Contract_TriggerSmartContract {
			var triggerContract core.TriggerSmartContract
			if err := proto.Unmarshal(contract.Parameter.Value, &triggerContract); err == nil {
				// Decode transfer data
				dataHex := hex.EncodeToString(triggerContract.Data)
				if len(dataHex) >= 8 && dataHex[:8] == "a9059cbb" {
					// This is a transfer call
					// Extract to address (bytes 8-72)
					if len(dataHex) >= 72 {
						toBytes := common.FromHex(dataHex[8:72])
						toAddr := address.Address(toBytes[12:]) // Last 20 bytes
						result.To = toAddr.String()
					}

					// Extract amount (bytes 72-136)
					if len(dataHex) >= 136 {
						amountBytes := common.FromHex(dataHex[72:136])
						result.Amount = new(big.Int).SetBytes(amountBytes)
					}
				}

				result.From = address.Address(triggerContract.OwnerAddress).String()
			}
		}
	}

	// Get fee
	if txInfo.Fee > 0 {
		result.Fee = big.NewInt(txInfo.Fee)
	}

	t.logger.Info("TRC20 transaction retrieved",
		zap.String("tx_hash", txHash),
		zap.String("status", string(result.Status)),
		zap.Int64("block", *result.BlockNumber))

	return result, nil
}

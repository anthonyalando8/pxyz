package bitcoin

// internal/chains/bitcoin/client.go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type BitcoinClient struct {
	httpClient *http.Client
	rpcURL     string
	apiKey     string
	network    string // mainnet, testnet, regtest
	logger     *zap.Logger
}

func NewBitcoinClient(rpcURL, apiKey, network string, logger *zap.Logger) *BitcoinClient {
	return &BitcoinClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rpcURL:  rpcURL,
		apiKey:  apiKey,
		network: network,
		logger:  logger,
	}
}

// RPC request/response structures
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type RPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
	ID     string          `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// doRPCRequest performs a JSON-RPC request
func (c *BitcoinClient) doRPCRequest(ctx context. Context, method string, params []interface{}) (json.RawMessage, error) {
	reqBody := RPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("%d", time. Now().UnixNano()),
		Method:  method,
		Params:  params,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes. NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt. Errorf("RPC request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// GetBlockchainInfo gets blockchain information
func (c *BitcoinClient) GetBlockchainInfo(ctx context.Context) (*BlockchainInfo, error) {
	result, err := c. doRPCRequest(ctx, "getblockchaininfo", []interface{}{})
	if err != nil {
		return nil, err
	}

	var info BlockchainInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// GetBalance gets wallet balance
func (c *BitcoinClient) GetBalance(ctx context.Context, address string) (int64, error) {
	// For Bitcoin, we use block explorer API (blockchain.info, blockstream, etc.)
	// Or if running a full node with address indexing
	
	// Using Blockstream API for simplicity
	apiURL := c.getBlockExplorerURL() + "/address/" + address

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp. Body)
	if err != nil {
		return 0, err
	}

	var addrInfo AddressInfo
	if err := json.Unmarshal(body, &addrInfo); err != nil {
		return 0, err
	}

	// Balance is in satoshis
	balance := addrInfo.ChainStats.FundedTxoSum - addrInfo.ChainStats.SpentTxoSum

	return balance, nil
}

// GetUTXOs gets unspent transaction outputs for address
func (c *BitcoinClient) GetUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	apiURL := c.getBlockExplorerURL() + "/address/" + address + "/utxo"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c. httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXOs:  %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var utxos []UTXO
	if err := json.Unmarshal(body, &utxos); err != nil {
		return nil, err
	}

	return utxos, nil
}

// BroadcastTransaction broadcasts a signed transaction (FIXED)
func (c *BitcoinClient) BroadcastTransaction(ctx context.Context, rawTx string) (string, error) {
	// Ensure proper URL format
	baseURL := strings.TrimSuffix(c.getBlockExplorerURL(), "/")
	broadcastURL := baseURL + "/tx"
	
	c.logger.Info("Broadcasting transaction",
		zap.String("url", broadcastURL))
	
	req, err := http.NewRequestWithContext(ctx, "POST", broadcastURL, strings.NewReader(rawTx))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "text/plain")
	
	// Create client without redirect following
	client := &http.Client{
		Timeout: 30 * time. Second,
		CheckRedirect:  func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	
	resp, err := client. Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body. Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt. Errorf("failed to read response: %w", err)
	}
	
	c.logger.Info("Broadcast response",
		zap.Int("status", resp.StatusCode),
		zap.String("body", string(body)))
	
	if resp.StatusCode == http.StatusOK {
		txHash := strings.TrimSpace(string(body))
		return txHash, nil
	}
	
	return "", fmt. Errorf("broadcast failed (status %d): %s", resp.StatusCode, string(body))
}

// GetTransaction gets transaction details
func (c *BitcoinClient) GetTransaction(ctx context. Context, txHash string) (*TransactionInfo, error) {
	apiURL := c.getBlockExplorerURL() + "/tx/" + txHash

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	defer resp.Body.Close()

	body, err := io. ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var txInfo TransactionInfo
	if err := json.Unmarshal(body, &txInfo); err != nil {
		return nil, err
	}

	return &txInfo, nil
}


// EstimateFee estimates transaction fee with multiple fallbacks
func (c *BitcoinClient) EstimateFee(ctx context.Context, confirmationTarget int) (float64, error) {
	// Try 1: Mempool.space (most reliable)
	if fee, err := c.estimateFeeMempool(ctx, confirmationTarget); err == nil {
		return fee, nil
	}

	// Try 2: Blockstream API
	if fee, err := c. estimateFeeBlockstream(ctx, confirmationTarget); err == nil {
		return fee, nil
	}

	// Try 3: Bitcoin Core RPC (if configured)
	if c.rpcURL != "" && ! strings.Contains(c.rpcURL, "blockstream") && !strings.Contains(c.rpcURL, "mempool") {
		if fee, err := c.estimateFeeRPC(ctx, confirmationTarget); err == nil {
			return fee, nil
		}
	}

	// Fallback:  Use conservative defaults based on network conditions
	return c.getDefaultFeeRate(confirmationTarget), nil
}

// estimateFeeMempool gets fee from mempool.space
func (c *BitcoinClient) estimateFeeMempool(ctx context.Context, confirmationTarget int) (float64, error) {
	var feeAPIURL string
	switch c.network {
	case "mainnet":
		feeAPIURL = "https://mempool.space/api/v1/fees/recommended"
	case "testnet":
		feeAPIURL = "https://mempool.space/testnet/api/v1/fees/recommended"
	default:
		feeAPIURL = "https://mempool.space/testnet/api/v1/fees/recommended"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feeAPIURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp. Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp. Body)
	if err != nil {
		return 0, err
	}

	var feeRates struct {
		FastestFee  int `json:"fastestFee"`
		HalfHourFee int `json:"halfHourFee"`
		HourFee     int `json:"hourFee"`
		EconomyFee  int `json:"economyFee"`
		MinimumFee  int `json:"minimumFee"`
	}

	if err := json.Unmarshal(body, &feeRates); err != nil {
		return 0, err
	}

	switch {
	case confirmationTarget <= 1:
		return float64(feeRates.FastestFee), nil
	case confirmationTarget <= 3:
		return float64(feeRates.HalfHourFee), nil
	case confirmationTarget <= 6:
		return float64(feeRates. HourFee), nil
	default:
		return float64(feeRates.EconomyFee), nil
	}
}

// estimateFeeBlockstream gets fee from Blockstream
func (c *BitcoinClient) estimateFeeBlockstream(ctx context.Context, confirmationTarget int) (float64, error) {
	feeAPIURL := c.getBlockExplorerURL() + "/fee-estimates"

	req, err := http.NewRequestWithContext(ctx, "GET", feeAPIURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var feeEstimates map[string]float64
	if err := json.Unmarshal(body, &feeEstimates); err != nil {
		return 0, err
	}

	// Try exact match first
	if fee, ok := feeEstimates[fmt.Sprintf("%d", confirmationTarget)]; ok {
		return fee, nil
	}

	// Fallback to closest
	for _, target := range []int{3, 6, 1, 2, 12} {
		if fee, ok := feeEstimates[fmt. Sprintf("%d", target)]; ok {
			return fee, nil
		}
	}

	return 0, fmt.Errorf("no fee estimates available")
}

// estimateFeeRPC gets fee via Bitcoin Core RPC
func (c *BitcoinClient) estimateFeeRPC(ctx context.Context, confirmationTarget int) (float64, error) {
	result, err := c.doRPCRequest(ctx, "estimatesmartfee", []interface{}{confirmationTarget})
	if err != nil {
		return 0, err
	}

	var feeResult struct {
		FeeRate float64 `json:"feerate"` // BTC/kB
		Blocks  int     `json:"blocks"`
	}

	if err := json.Unmarshal(result, &feeResult); err != nil {
		return 0, err
	}

	if feeResult.FeeRate <= 0 {
		return 0, fmt.Errorf("invalid fee rate")
	}

	// Convert BTC/kB to sat/vB
	satPerByte := (feeResult.FeeRate * 100000000) / 1000
	return satPerByte, nil
}

// getDefaultFeeRate returns conservative default fee rates
func (c *BitcoinClient) getDefaultFeeRate(confirmationTarget int) float64 {
	// Conservative defaults for testnet and mainnet
	defaults := map[string]map[int]float64{
		"mainnet": {
			1:  50.0, // High priority
			3:  20.0, // Normal
			6:  10.0, // Low
			12: 5.0,  // Economy
		},
		"testnet": {
			1:  10.0, // High priority (testnet fees are lower)
			3:  5.0,  // Normal
			6:  2.0,  // Low
			12: 1.0,  // Economy
		},
	}

	networkDefaults, ok := defaults[c.network]
	if !ok {
		networkDefaults = defaults["testnet"]
	}

	// Find closest target
	for _, target := range []int{confirmationTarget, 3, 6, 1, 12} {
		if fee, ok := networkDefaults[target]; ok {
			return fee
		}
	}

	return 5.0 // Ultimate fallback
}

// getBlockExplorerURL returns the appropriate block explorer URL
func (c *BitcoinClient) getBlockExplorerURL() string {
	switch c.network {
	case "mainnet":
		return "https://blockstream.info/api"
	case "testnet": 
		return "https://blockstream.info/testnet/api"
	default:
		return "https://blockstream.info/testnet/api"
	}
}

// Response structures
type BlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	BestBlockHash        string  `json:"bestblockhash"`
	Difficulty           float64 `json:"difficulty"`
	MedianTime           int64   `json:"mediantime"`
	VerificationProgress float64 `json:"verificationprogress"`
	Pruned               bool    `json:"pruned"`
}

type AddressInfo struct {
	Address    string `json:"address"`
	ChainStats struct {
		FundedTxoCount int64 `json:"funded_txo_count"`
		FundedTxoSum   int64 `json:"funded_txo_sum"`
		SpentTxoCount  int64 `json:"spent_txo_count"`
		SpentTxoSum    int64 `json:"spent_txo_sum"`
		TxCount        int64 `json:"tx_count"`
	} `json:"chain_stats"`
	MempoolStats struct {
		FundedTxoCount int64 `json:"funded_txo_count"`
		FundedTxoSum   int64 `json:"funded_txo_sum"`
		SpentTxoCount  int64 `json:"spent_txo_count"`
		SpentTxoSum    int64 `json:"spent_txo_sum"`
		TxCount        int64 `json:"tx_count"`
	} `json:"mempool_stats"`
}

type UTXO struct {
	TxID   string `json:"txid"`
	Vout   uint32 `json:"vout"`
	Status struct {
		Confirmed   bool  `json:"confirmed"`
		BlockHeight int64 `json:"block_height"`
		BlockHash   string `json:"block_hash"`
		BlockTime   int64  `json:"block_time"`
	} `json:"status"`
	Value int64 `json:"value"` // satoshis
}

type TransactionInfo struct {
	TxID     string `json:"txid"`
	Version  int    `json:"version"`
	Locktime int64  `json:"locktime"`
	Size     int    `json:"size"`
	Weight   int    `json:"weight"`
	Fee      int64  `json:"fee"`
	Status   struct {
		Confirmed   bool   `json:"confirmed"`
		BlockHeight int64  `json:"block_height"`
		BlockHash   string `json:"block_hash"`
		BlockTime   int64  `json:"block_time"`
	} `json:"status"`
	Vin  []TxInput  `json:"vin"`
	Vout []TxOutput `json:"vout"`
}

type TxInput struct {
	TxID    string `json:"txid"`
	Vout    uint32 `json:"vout"`
	Prevout struct {
		ScriptPubKey        string `json:"scriptpubkey"`
		ScriptPubKeyAsm     string `json:"scriptpubkey_asm"`
		ScriptPubKeyType    string `json:"scriptpubkey_type"`
		ScriptPubKeyAddress string `json:"scriptpubkey_address"`
		Value               int64  `json:"value"`
	} `json:"prevout"`
	ScriptSig    string   `json:"scriptsig"`
	ScriptSigAsm string   `json:"scriptsig_asm"`
	Witness      []string `json:"witness"`
	IsCoinbase   bool     `json:"is_coinbase"`
	Sequence     uint32   `json:"sequence"`
}

type TxOutput struct {
	ScriptPubKey        string `json:"scriptpubkey"`
	ScriptPubKeyAsm     string `json:"scriptpubkey_asm"`
	ScriptPubKeyType    string `json:"scriptpubkey_type"`
	ScriptPubKeyAddress string `json:"scriptpubkey_address"`
	Value               int64  `json:"value"`
}
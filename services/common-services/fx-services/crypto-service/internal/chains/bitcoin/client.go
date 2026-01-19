package bitcoin
// internal/chains/bitcoin/client.go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// BroadcastTransaction broadcasts a signed transaction
func (c *BitcoinClient) BroadcastTransaction(ctx context.Context, rawTx string) (string, error) {
	result, err := c.doRPCRequest(ctx, "sendrawtransaction", []interface{}{rawTx})
	if err != nil {
		return "", fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	var txHash string
	if err := json. Unmarshal(result, &txHash); err != nil {
		return "", err
	}

	return txHash, nil
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

// EstimateFee estimates transaction fee (sat/vB)
func (c *BitcoinClient) EstimateFee(ctx context.Context, confirmationTarget int) (float64, error) {
	result, err := c.doRPCRequest(ctx, "estimatesmartfee", []interface{}{confirmationTarget})
	if err != nil {
		return 0, err
	}

	var feeResult struct {
		FeeRate float64 `json:"feerate"` // BTC/kB
		Blocks  int     `json:"blocks"`
	}

	if err := json. Unmarshal(result, &feeResult); err != nil {
		return 0, err
	}

	// Convert BTC/kB to sat/vB
	// 1 BTC = 100,000,000 satoshis
	// 1 kB = 1000 bytes
	satPerByte := (feeResult.FeeRate * 100000000) / 1000

	return satPerByte, nil
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
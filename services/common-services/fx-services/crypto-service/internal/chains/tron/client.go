// internal/chains/tron/client.go
package tron

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

// TronHTTPClient handles HTTP API calls to TronGrid
type TronHTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewTronHTTPClient creates a new HTTP client for TronGrid
func NewTronHTTPClient(baseURL, apiKey string, logger *zap.Logger) *TronHTTPClient {
	return &TronHTTPClient{
		baseURL:  baseURL,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// AccountTransactionsResponse represents account transactions response
type AccountTransactionsResponse struct {
	Success bool                `json:"success"`
	Data    []TransactionRecord `json:"data"`
	Meta    struct {
		At          int64 `json:"at"`
		Fingerprint string `json:"fingerprint"`
		PageSize    int    `json:"page_size"`
	} `json:"meta"`
}

// TransactionRecord represents a transaction record from API
type TransactionRecord struct {
	TxID           string `json:"txID"`
	BlockNumber    int64  `json:"block"`
	BlockTimestamp int64  `json:"block_timestamp"`
	From           string `json:"from"`
	To             string `json:"to"`
	Value          string `json:"value"`
	Type           string `json:"type"`
	ContractType   string `json:"contract_type"`
	Confirmed      bool   `json:"confirmed"`
	ContractData   struct {
		OwnerAddress    string `json:"owner_address"`
		ContractAddress string `json:"contract_address"`
		CallValue       int64  `json:"call_value"`
		Data            string `json:"data"`
	} `json:"contract_data,omitempty"`
	TokenInfo struct {
		Symbol   string `json:"symbol"`
		Address  string `json:"address"`
		Decimals int    `json:"decimals"`
		Name     string `json:"name"`
	} `json:"token_info,omitempty"`
}

// GetAccountTransactions gets transaction history for an address
func (c *TronHTTPClient) GetAccountTransactions(ctx context.Context, address string, limit, offset int) (*AccountTransactionsResponse, error) {
	c.logger.Info("getting account transactions via HTTP",
		zap.String("address", address),
		zap.Int("limit", limit),
		zap.Int("offset", offset))

	// TronGrid REST API endpoint
	url := fmt.Sprintf("%s/v1/accounts/%s/transactions?limit=%d&start=%d", 
		c. baseURL, address, limit, offset)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add API key header
	if c.apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c. apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp. Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result AccountTransactionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("account transactions retrieved",
		zap.String("address", address),
		zap.Int("count", len(result.Data)))

	return &result, nil
}

// GetAccountInfo gets detailed account information
func (c *TronHTTPClient) GetAccountInfo(ctx context.Context, address string) (*AccountInfo, error) {
	c.logger.Info("getting account info via HTTP",
		zap.String("address", address))

	url := fmt.Sprintf("%s/v1/accounts/%s", c.baseURL, address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c.apiKey)
	}
	req.Header. Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt. Errorf("failed to execute request: %w", err)
	}
	defer resp.Body. Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io. ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool        `json:"success"`
		Data    AccountInfo `json:"data"`
	}

	if err := json. NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt. Errorf("failed to decode response: %w", err)
	}

	return &result.Data, nil
}

// AccountInfo represents account information
type AccountInfo struct {
	Address       string                 `json:"address"`
	Balance       int64                  `json:"balance"`
	CreateTime    int64                  `json:"create_time"`
	LatestOpTime  int64                  `json:"latest_opration_time"`
	FreeNetUsed   int64                  `json:"free_net_used"`
	FreeNetLimit  int64                  `json:"free_net_limit"`
	NetUsed       int64                  `json:"net_used"`
	NetLimit      int64                  `json:"net_limit"`
	EnergyUsed    int64                  `json:"energy_used"`
	EnergyLimit   int64                  `json:"energy_limit"`
	TotalTxCount  int64                  `json:"total_tx_count"`
	Tokens        map[string]interface{} `json:"tokens,omitempty"`
}

// GetTRC20Transactions gets TRC20 token transactions
func (c *TronHTTPClient) GetTRC20Transactions(ctx context.Context, address string, contractAddress string, limit int) ([]TransactionRecord, error) {
	c.logger.Info("getting TRC20 transactions via HTTP",
		zap.String("address", address),
		zap.String("contract", contractAddress),
		zap.Int("limit", limit))

	url := fmt.Sprintf("%s/v1/accounts/%s/transactions/trc20?limit=%d&contract_address=%s",
		c.baseURL, address, limit, contractAddress)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c. apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool                `json:"success"`
		Data    []TransactionRecord `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("TRC20 transactions retrieved",
		zap.String("address", address),
		zap.Int("count", len(result.Data)))

	return result.Data, nil
}

// BroadcastHex broadcasts a raw transaction (hex encoded)
func (c *TronHTTPClient) BroadcastHex(ctx context.Context, txHex string) (*BroadcastResult, error) {
	c.logger.Info("broadcasting transaction via HTTP")

	url := fmt.Sprintf("%s/wallet/broadcasthex", c.baseURL)

	payload := map[string]string{
		"transaction": txHex,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt. Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt. Errorf("failed to execute request:  %w", err)
	}
	defer resp.Body.Close()

	var result BroadcastResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Result {
		return &result, fmt.Errorf("broadcast failed: %s", result. Message)
	}

	c.logger.Info("transaction broadcasted",
		zap.String("tx_id", result.TxID))

	return &result, nil
}

// BroadcastResult represents broadcast response
type BroadcastResult struct {
	Result  bool   `json:"result"`
	TxID    string `json:"txid"`
	Message string `json:"message,omitempty"`
}

// GetTokenBalance gets TRC20 token balance using HTTP API
func (c *TronHTTPClient) GetTokenBalance(ctx context.Context, address, contractAddress string) (string, error) {
	c.logger.Info("getting token balance via HTTP",
		zap.String("address", address),
		zap.String("contract", contractAddress))

	// This uses a different endpoint structure
	url := fmt.Sprintf("%s/v1/accounts/%s/trc20/%s", c.baseURL, address, contractAddress)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt. Errorf("failed to execute request: %w", err)
	}
	defer resp.Body. Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io. ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool   `json:"success"`
		Data    string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt. Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}
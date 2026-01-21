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

	// Use wallet/getaccount endpoint (works for new accounts too)
	url := fmt.Sprintf("%s/wallet/getaccount", c.baseURL)

	payload := map[string]interface{}{
		"address": address,
		"visible": true, // Use Base58 address format
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req. Header.Set("TRON-PRO-API-KEY", c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c. httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp. Body)

	// Check for empty response (account doesn't exist yet)
	if len(body) == 0 || string(body) == "{}" {
		c.logger.Info("account has no data yet, returning zero balance",
			zap.String("address", address))
		
		return &AccountInfo{
			Address:       address,
			Balance:      0,
			CreateTime:   0,
			TotalTxCount: 0,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var accountInfo AccountInfo
	if err := json. Unmarshal(body, &accountInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If address is empty in response, account doesn't exist
	if accountInfo.Address == "" {
		accountInfo.Address = address
		accountInfo.Balance = 0
	}

	c.logger.Info("account info retrieved",
		zap.String("address", address),
		zap.Int64("balance", accountInfo.Balance))

	return &accountInfo, nil
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

// new

// internal/chains/tron/client.go

// Add these methods to your TronHTTPClient:

// TriggerConstantContract triggers a constant contract call (doesn't modify state)
func (c *TronHTTPClient) TriggerConstantContract(ctx context.Context, ownerAddress, contractAddress, functionSelector, parameter string) (*TriggerConstantResult, error) {
	c.logger.Info("triggering constant contract",
		zap.String("owner", ownerAddress),
		zap.String("contract", contractAddress))

	url := fmt.Sprintf("%s/wallet/triggerconstantcontract", c. baseURL)

	payload := map[string]interface{}{
		"owner_address":      ownerAddress,
		"contract_address":  contractAddress,
		"function_selector": functionSelector,
		"parameter":         parameter,
		"visible":            true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt. Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt. Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c. apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c. httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp. Body)
	if err != nil {
		return nil, fmt. Errorf("failed to read response:  %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result TriggerConstantResult
	if err := json. Unmarshal(body, &result); err != nil {
		c.logger.Warn("Failed to parse trigger response",
			zap.Error(err),
			zap.String("response", string(body)))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("constant contract triggered",
		zap. Int64("energy_used", result. EnergyUsed),
		zap.Bool("success", result.Result. Result))

	return &result, nil
}

// TriggerConstantResult represents the response from triggerconstantcontract
type TriggerConstantResult struct {
	Result struct {
		Result  bool   `json:"result"`
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"result"`
	EnergyUsed     int64    `json:"energy_used"`
	ConstantResult []string `json:"constant_result"`
	Transaction    struct {
		Ret []struct {
			ContractRet string `json:"contractRet"`
		} `json:"ret"`
		Visible  bool `json:"visible"`
		TxID     string `json:"txID"`
		RawData  struct {
			Contract []struct {
				Parameter struct {
					Value struct {
						Data            string `json:"data"`
						OwnerAddress    string `json:"owner_address"`
						ContractAddress string `json:"contract_address"`
					} `json:"value"`
					TypeUrl string `json:"type_url"`
				} `json:"parameter"`
				Type string `json:"type"`
			} `json:"contract"`
			RefBlockBytes string `json:"ref_block_bytes"`
			RefBlockHash  string `json:"ref_block_hash"`
			Expiration    int64  `json:"expiration"`
			Timestamp     int64  `json:"timestamp"`
		} `json:"raw_data"`
	} `json:"transaction"`
}

// GetAccountResources gets account energy and bandwidth resources
func (c *TronHTTPClient) GetAccountResources(ctx context.Context, address string) (*AccountResources, error) {
	c.logger.Info("getting account resources",
		zap.String("address", address))

	url := fmt.Sprintf("%s/wallet/getaccountresource", c.baseURL)

	payload := map[string]interface{}{
		"address": address,
		"visible": true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
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
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp. Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Empty response means new account with no resources
	if len(body) == 0 || string(body) == "{}" {
		return &AccountResources{
			EnergyLimit:        0,
			EnergyUsed:         0,
			EnergyAvailable:    0,
			NetLimit:           0,
			NetUsed:            0,
			BandwidthAvailable: 0,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		EnergyLimit         int64 `json:"EnergyLimit"`
		EnergyUsed          int64 `json:"EnergyUsed"`
		TotalEnergyLimit    int64 `json:"TotalEnergyLimit"`
		TotalEnergyWeight   int64 `json:"TotalEnergyWeight"`
		NetLimit            int64 `json:"NetLimit"`
		NetUsed             int64 `json:"NetUsed"`
		TotalNetLimit       int64 `json:"TotalNetLimit"`
		TotalNetWeight      int64 `json:"TotalNetWeight"`
		FreeNetLimit        int64 `json:"freeNetLimit"`
		FreeNetUsed         int64 `json:"freeNetUsed"`
		AssetNetLimit       []struct {
			Key   string `json:"key"`
			Value int64  `json:"value"`
		} `json:"assetNetLimit,omitempty"`
		AssetNetUsed []struct {
			Key   string `json:"key"`
			Value int64  `json:"value"`
		} `json:"assetNetUsed,omitempty"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	resources := &AccountResources{
		EnergyLimit:        result. EnergyLimit,
		EnergyUsed:         result. EnergyUsed,
		EnergyAvailable:    result.EnergyLimit - result.EnergyUsed,
		NetLimit:           result.NetLimit + result.FreeNetLimit,
		NetUsed:            result.NetUsed + result.FreeNetUsed,
		BandwidthAvailable: (result.NetLimit + result.FreeNetLimit) - (result.NetUsed + result. FreeNetUsed),
	}

	// Ensure non-negative
	if resources.EnergyAvailable < 0 {
		resources.EnergyAvailable = 0
	}
	if resources.BandwidthAvailable < 0 {
		resources.BandwidthAvailable = 0
	}

	c.logger.Info("account resources retrieved",
		zap. String("address", address),
		zap.Int64("energy_available", resources.EnergyAvailable),
		zap.Int64("bandwidth_available", resources.BandwidthAvailable))

	return resources, nil
}

// AccountResources represents TRON account resources
type AccountResources struct {
	EnergyLimit        int64
	EnergyUsed         int64
	EnergyAvailable    int64
	NetLimit           int64
	NetUsed            int64
	BandwidthAvailable int64
}
// internal/chains/circle/client.go
package circle

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

type Client struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	logger      *zap.Logger
	entityID    string
	walletSetID string
}

func NewClient(apiKey, environment string, logger *zap.Logger) (*Client, error) {
	baseURL := "https://api-sandbox.circle.com"
	if environment == "production" {
		baseURL = "https://api.circle.com"
	}

	client := &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}

	//  Initialize entity and wallet set
	ctx := context.Background()
	if err := client.initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Circle: %w", err)
	}

	return client, nil
}

//  Initialize creates entity and wallet set if they don't exist
func (c *Client) initialize(ctx context.Context) error {
	c.logger.Info("Initializing Circle configuration...")

	// 1. Get or create entity
	entity, err := c.getOrCreateEntity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get/create entity: %w", err)
	}
	c.entityID = entity.ID
	c.logger.Info("Entity initialized", zap.String("entity_id", c.entityID))

	// 2. Get or create wallet set
	walletSet, err := c.getOrCreateWalletSet(ctx)
	if err != nil {
		return fmt.Errorf("failed to get/create wallet set: %w", err)
	}
	c.walletSetID = walletSet.ID
	c.logger.Info("Wallet set initialized", zap.String("wallet_set_id", c.walletSetID))

	c.logger.Info("Circle initialization complete",
		zap.String("entity_id", c.entityID),
		zap.String("wallet_set_id", c.walletSetID))

	return nil
}

// ============================================================================
// ENTITY MANAGEMENT
// ============================================================================

// getOrCreateEntity gets existing entity or creates new one
func (c *Client) getOrCreateEntity(ctx context.Context) (*Entity, error) {
	// Try to get existing entity
	entities, err := c.listEntities(ctx)
	if err != nil {
		c.logger.Warn("Failed to list entities, will try to create", zap.Error(err))
	} else if len(entities) > 0 {
		c.logger.Info("Using existing entity", zap.String("entity_id", entities[0].ID))
		return &entities[0], nil
	}

	// Create new entity
	c.logger.Info("Creating new Circle entity...")
	return c.createEntity(ctx)
}

func (c *Client) listEntities(ctx context.Context) ([]Entity, error) {
	var result struct {
		Data []Entity `json:"data"`
	}

	if err := c.get(ctx, "/v1/w3s/config/entity", &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *Client) createEntity(ctx context.Context) (*Entity, error) {
	payload := map[string]interface{}{
		"idempotencyKey": fmt.Sprintf("entity-%d", time.Now().Unix()),
	}

	var result struct {
		Data Entity `json:"data"`
	}

	if err := c.post(ctx, "/v1/w3s/config/entity", payload, &result); err != nil {
		return nil, err
	}

	c.logger.Info("Entity created successfully", zap.String("entity_id", result.Data.ID))
	return &result.Data, nil
}

// ============================================================================
// WALLET SET MANAGEMENT
// ============================================================================

// getOrCreateWalletSet gets existing wallet set or creates new one
func (c *Client) getOrCreateWalletSet(ctx context.Context) (*WalletSet, error) {
	// Try to get existing wallet sets
	walletSets, err := c.listWalletSets(ctx)
	if err != nil {
		c.logger.Warn("Failed to list wallet sets, will try to create", zap.Error(err))
	} else if len(walletSets) > 0 {
		c.logger.Info("Using existing wallet set",
			zap.String("wallet_set_id", walletSets[0].ID),
			zap.String("name", walletSets[0].Name))
		return &walletSets[0], nil
	}

	// Create new wallet set
	c.logger.Info("Creating new wallet set...")
	return c.createWalletSet(ctx, "Crypto Service Wallets")
}

func (c *Client) listWalletSets(ctx context.Context) ([]WalletSet, error) {
	var result struct {
		Data struct {
			WalletSets []WalletSet `json:"walletSets"`
		} `json:"data"`
	}

	if err := c.get(ctx, "/v1/w3s/walletSets", &result); err != nil {
		return nil, err
	}

	return result.Data.WalletSets, nil
}

func (c *Client) createWalletSet(ctx context.Context, name string) (*WalletSet, error) {
	payload := map[string]interface{}{
		"idempotencyKey": fmt.Sprintf("walletset-%d", time.Now().Unix()),
		"name":           name,
	}

	var result struct {
		Data WalletSet `json:"data"`
	}

	if err := c.post(ctx, "/v1/w3s/walletSets", payload, &result); err != nil {
		return nil, err
	}

	c.logger.Info("Wallet set created successfully",
		zap.String("wallet_set_id", result.Data.ID),
		zap.String("name", result.Data.Name))
	
	return &result.Data, nil
}

// ============================================================================
// WALLET OPERATIONS
// ============================================================================

func (c *Client) CreateWallet(ctx context.Context, userID string) (*CircleWallet, error) {
	payload := map[string]interface{}{
		"idempotencyKey": fmt.Sprintf("wallet-%s-%d", userID, time.Now().Unix()),
		"walletSetId":    c.walletSetID,
		"metadata": map[string]string{
			"user_id": userID,
		},
	}

	var result struct {
		Data CircleWallet `json:"data"`
	}

	if err := c.post(ctx, "/v1/w3s/developer/wallets", payload, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) GetWalletBalance(ctx context.Context, walletID string) (*CircleBalance, error) {
	var result struct {
		Data CircleBalance `json:"data"`
	}

	if err := c.get(ctx, fmt.Sprintf("/v1/w3s/wallets/%s/balances", walletID), &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) CreateTransfer(ctx context.Context, req *TransferRequest) (*CircleTransfer, error) {
	payload := map[string]interface{}{
		"idempotencyKey":     req.IdempotencyKey,
		"walletId":           req.WalletID,
		"destinationAddress": req.ToAddress,
		"amounts":            []string{req.Amount},
		"tokenId":            "36b1737c-dd00-5b26-b6d7-ab8c0a129dfa", // USDC on Ethereum
		"fee": map[string]interface{}{
			"type": "level",
			"config": map[string]string{
				"feeLevel": "MEDIUM",
			},
		},
	}

	var result struct {
		Data CircleTransfer `json:"data"`
	}

	if err := c.post(ctx, "/v1/w3s/developer/transactions/transfer", payload, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) GetTransaction(ctx context.Context, txID string) (*CircleTransaction, error) {
	var result struct {
		Data CircleTransaction `json:"data"`
	}

	if err := c.get(ctx, fmt.Sprintf("/v1/w3s/transactions/%s", txID), &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// ============================================================================
// HTTP HELPERS
// ============================================================================

func (c *Client) post(ctx context.Context, path string, payload interface{}, result interface{}) error {
	return c.request(ctx, "POST", path, payload, result)
}

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	return c.request(ctx, "GET", path, nil, result)
}

func (c *Client) request(ctx context.Context, method, path string, payload, result interface{}) error {
	url := c.baseURL + path

	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Circle API request",
		zap.String("method", method),
		zap.String("path", path))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		c.logger.Error("Circle API error",
			zap.Int("status", resp.StatusCode),
			zap.String("response", string(bodyBytes)))
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// ============================================================================
// RESPONSE TYPES
// ============================================================================

type Entity struct {
	ID         string `json:"id"`
	CreateDate string `json:"createDate"`
	UpdateDate string `json:"updateDate"`
}

type WalletSet struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreateDate string `json:"createDate"`
	UpdateDate string `json:"updateDate"`
}

type CircleWallet struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	WalletSetID string `json:"walletSetId"`
	Address     string `json:"address"`
	Blockchain  string `json:"blockchain"`
	CreateDate  string `json:"createDate"`
}

type CircleBalance struct {
	TokenBalances []TokenBalance `json:"tokenBalances"`
}

type TokenBalance struct {
	Token  Token  `json:"token"`
	Amount string `json:"amount"`
}

type Token struct {
	ID       string `json:"id"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type TransferRequest struct {
	IdempotencyKey string
	WalletID       string
	ToAddress      string
	Amount         string
}

type CircleTransfer struct {
	ID              string `json:"id"`
	State           string `json:"state"`
	TransactionHash string `json:"txHash"`
	CreateDate      string `json:"createDate"`
}

type CircleTransaction struct {
	ID              string `json:"id"`
	State           string `json:"state"`
	TransactionHash string `json:"txHash"`
	Blockchain      string `json:"blockchain"`
	CreateDate      string `json:"createDate"`
	UpdateDate      string `json:"updateDate"`
}
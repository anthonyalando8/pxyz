// internal/chains/ethereum/circle/client.go
package circle

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid" //  Add this import
	"go.uber.org/zap"
)

type Client struct {
	apiKey          string
	baseURL         string
	httpClient      *http.Client
	logger          *zap.Logger
	entityPublicKey string
	entitySecret    []byte //  Store plain secret (not ciphertext)
	walletSetID     string
}

// Updated signature - takes plain entity secret
func NewClient(apiKey, environment, entitySecretBase64 string, logger *zap.Logger) (*Client, error) {
	baseURL := "https://api.circle.com"

	client := &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}

	//  If entity secret provided, decode it
	if entitySecretBase64 != "" {
		secret, err := base64.StdEncoding.DecodeString(entitySecretBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode entity secret: %w", err)
		}
		client.entitySecret = secret
		logger.Info("Using provided entity secret")
	}

	// Initialize
	ctx := context.Background()
	if err := client.initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Circle: %w", err)
	}

	logger.Info("Circle client initialized successfully",
		zap.String("wallet_set_id", client.walletSetID))

	return client, nil
}

// ============================================================================
// INITIALIZATION
// ============================================================================

func (c *Client) initialize(ctx context.Context) error {

	// Step 1: Get entity public key
	if err := c.fetchEntityPublicKey(ctx); err != nil {
		return fmt.Errorf("failed to fetch entity public key: %w", err)
	}

	// Step 2: Generate entity secret if not provided
	if len(c.entitySecret) == 0 {
		if err := c.generateEntitySecret(); err != nil {
			return fmt.Errorf("failed to generate entity secret: %w", err)
		}
	}

	// Step 3: Get or create wallet set
	walletSet, err := c.getOrCreateWalletSet(ctx)
	if err != nil {
		return fmt.Errorf("failed to get/create wallet set: %w", err)
	}
	c.walletSetID = walletSet.ID

	c.logger.Info("Circle initialization complete",
		zap.String("wallet_set_id", c.walletSetID))

	return nil
}

func (c *Client) fetchEntityPublicKey(ctx context.Context) error {

	var result struct {
		Data struct {
			PublicKey string `json:"publicKey"`
		} `json:"data"`
	}

	if err := c.get(ctx, "/v1/w3s/config/entity/publicKey", &result); err != nil {
		return err
	}

	c.entityPublicKey = result.Data.PublicKey

	return nil
}

func (c *Client) generateEntitySecret() error {

	// Generate 32 random bytes
	c.entitySecret = make([]byte, 32)
	if _, err := rand.Read(c.entitySecret); err != nil {
		return fmt.Errorf("failed to generate random secret: %w", err)
	}

	//  Log the base64-encoded secret so you can save it
	secretBase64 := base64.StdEncoding.EncodeToString(c.entitySecret)
	c.logger.Info("Entity secret generated",
		zap.String("secret_base64", secretBase64),
		zap.String("note", "Save this to CIRCLE_ENTITY_SECRET env var"))

	return nil
}

// encryptEntitySecret encrypts fresh each time
func (c *Client) encryptEntitySecret() (string, error) {
	// Parse PEM-encoded public key
	block, _ := pem.Decode([]byte(c.entityPublicKey))
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("not an RSA public key")
	}

	//  Encrypt using RSA-OAEP with SHA-256 (fresh encryption each time)
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, c.entitySecret, nil)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// ============================================================================
// WALLET SET MANAGEMENT
// ============================================================================

func (c *Client) getOrCreateWalletSet(ctx context.Context) (*WalletSet, error) {
	// Try to list existing wallet sets
	walletSets, err := c.listWalletSets(ctx)
	if err != nil {
		c.logger.Warn("Failed to list wallet sets, will try to create", zap.Error(err))
	} else if len(walletSets) > 0 {
		c.logger.Info("Using existing wallet set",
			zap.String("wallet_set_id", walletSets[0].ID))
		return &walletSets[0], nil
	}

	// Create new wallet set

	return c.createWalletSet(ctx, "PXYZ Wallets")
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
	//  Encrypt fresh for this request
	ciphertext, err := c.encryptEntitySecret()
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt entity secret: %w", err)
	}

	payload := map[string]interface{}{
		"idempotencyKey":         uuid.New().String(),
		"name":                   name,
		"entitySecretCiphertext": ciphertext, //  Fresh ciphertext
	}

	var result struct {
		Data struct {
			WalletSet WalletSet `json:"walletSet"`
		} `json:"data"`
	}

	if err := c.post(ctx, "/v1/w3s/developer/walletSets", payload, &result); err != nil {
		return nil, err
	}

	c.logger.Info("Wallet set created successfully",
		zap.String("wallet_set_id", result.Data.WalletSet.ID))

	return &result.Data.WalletSet, nil
}

// ============================================================================
// WALLET OPERATIONS
// ============================================================================

func (c *Client) CreateWallet(ctx context.Context, userID string) (*CircleWallet, error) {
	c.logger.Info("Creating Circle developer wallet",
		zap.String("user_id", userID))

	//  Encrypt fresh for this request
	ciphertext, err := c.encryptEntitySecret()
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt entity secret: %w", err)
	}

	payload := map[string]interface{}{
		"idempotencyKey":         uuid.New().String(),
		"entitySecretCiphertext": ciphertext, //  Fresh ciphertext
		"walletSetId":            c.walletSetID,
		"blockchains":            []string{"ETH-SEPOLIA"},
		"count":                  1,
		"accountType":            "SCA",
		"metadata": []map[string]string{
			{
				"name":  "user_id",
				"refId": userID,
			},
		},
	}

	var result struct {
		Data struct {
			Wallets []CircleWallet `json:"wallets"`
		} `json:"data"`
	}

	if err := c.post(ctx, "/v1/w3s/developer/wallets", payload, &result); err != nil {
		return nil, err
	}

	if len(result.Data.Wallets) == 0 {
		return nil, fmt.Errorf("no wallet created")
	}

	wallet := result.Data.Wallets[0]

	c.logger.Info("Circle wallet created successfully",
		zap.String("wallet_id", wallet.ID),
		zap.String("address", wallet.Address),
		zap.String("blockchain", wallet.Blockchain),
		zap.String("state", wallet.State))

	return &wallet, nil
}
func (c *Client) GetWallet(ctx context.Context, walletID string) (*CircleWallet, error) {
	var result struct {
		Data struct {
			Wallet CircleWallet `json:"wallet"`
		} `json:"data"`
	}

	if err := c.get(ctx, fmt.Sprintf("/v1/w3s/wallets/%s", walletID), &result); err != nil {
		return nil, err
	}

	return &result.Data.Wallet, nil
}

func (c *Client) ListWallets(ctx context.Context) ([]CircleWallet, error) {
	var result struct {
		Data struct {
			Wallets []CircleWallet `json:"wallets"`
		} `json:"data"`
	}

	if err := c.get(ctx, "/v1/w3s/wallets", &result); err != nil {
		return nil, err
	}

	return result.Data.Wallets, nil
}

func (c *Client) GetWalletBalance(ctx context.Context, walletID string) (*CircleBalance, error) {
	c.logger.Debug("Getting wallet balance", zap.String("wallet_id", walletID))

	var result struct {
		Data CircleBalance `json:"data"`
	}

	if err := c.get(ctx, fmt.Sprintf("/v1/w3s/wallets/%s/balances", walletID), &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}
func (c *Client) CreateTransfer(ctx context.Context, req *TransferRequest) (*CircleTransfer, error) {
	c.logger.Info("Creating Circle transfer",
		zap.String("wallet_id", req.WalletID),
		zap.String("to_address", req.ToAddress),
		zap.String("amount", req.Amount))

	//  Encrypt fresh for this request
	ciphertext, err := c.encryptEntitySecret()
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt entity secret: %w", err)
	}

	payload := map[string]interface{}{
		"idempotencyKey":         uuid.New().String(),
		"entitySecretCiphertext": ciphertext, //  Fresh ciphertext
		"walletId":               req.WalletID,
		"destinationAddress":     req.ToAddress,
		"amounts":                []string{req.Amount},
		"feeLevel":               "MEDIUM",
	}

	if req.TokenID != "" {
		payload["tokenId"] = req.TokenID
	}

	var result struct {
		Data CircleTransfer `json:"data"`
	}

	if err := c.post(ctx, "/v1/w3s/developer/transactions/transfer", payload, &result); err != nil {
		return nil, err
	}

	c.logger.Info("Circle transfer created",
		zap.String("transfer_id", result.Data.ID),
		zap.String("state", result.Data.State))

	return &result.Data, nil
}

func (c *Client) GetTransaction(ctx context.Context, txID string) (*CircleTransaction, error) {
	var result struct {
		Data struct {
			Transaction CircleTransaction `json:"transaction"`
		} `json:"data"`
	}

	if err := c.get(ctx, fmt.Sprintf("/v1/w3s/transactions/%s", txID), &result); err != nil {
		return nil, err
	}

	return &result.Data.Transaction, nil
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
	var payloadStr string
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
		payloadStr = string(jsonData)
	}

	c.logger.Debug("Circle API request",
		zap.String("method", method),
		zap.String("url", url),
		zap.String("payload", payloadStr))

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	responseStr := string(bodyBytes)

	c.logger.Debug("Circle API response",
		zap.Int("status", resp.StatusCode),
		zap.String("response", responseStr))

	if resp.StatusCode >= 400 {
		c.logger.Error("Circle API error",
			zap.Int("status", resp.StatusCode),
			zap.String("method", method),
			zap.String("url", url),
			zap.String("response", responseStr))
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, responseStr)
	}

	if result != nil {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			c.logger.Error("Failed to decode response",
				zap.String("response", responseStr),
				zap.Error(err))
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// ============================================================================
// RESPONSE TYPES (Same as before)
// ============================================================================

type WalletSet struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	CreateDate  string `json:"createDate"`
	UpdateDate  string `json:"updateDate"`
	CustodyType string `json:"custodyType"`
}

type CircleWallet struct {
	ID               string `json:"id"`
	Address          string `json:"address"`
	Blockchain       string `json:"blockchain"`
	CreateDate       string `json:"createDate"`
	UpdateDate       string `json:"updateDate"`
	CustodyType      string `json:"custodyType"`
	State            string `json:"state"`
	WalletSetID      string `json:"walletSetId"`
	AccountType      string `json:"accountType"`
	Name             string `json:"name,omitempty"`
	RefID            string `json:"refId,omitempty"`
	UserID           string `json:"userId,omitempty"`
	InitialPublicKey string `json:"initialPublicKey,omitempty"`
}

type CircleBalance struct {
	TokenBalances []TokenBalance `json:"tokenBalances"`
}

type TokenBalance struct {
	Amount     string `json:"amount"`
	Token      Token  `json:"token"`
	UpdateDate string `json:"updateDate"`
}

type Token struct {
	ID           string `json:"id"`
	Blockchain   string `json:"blockchain"`
	IsNative     bool   `json:"isNative"`
	Name         string `json:"name"`
	Standard     string `json:"standard"`
	Decimals     int    `json:"decimals"`
	Symbol       string `json:"symbol"`
	TokenAddress string `json:"tokenAddress"`
}

type TransferRequest struct {
	IdempotencyKey string
	WalletID       string
	ToAddress      string
	Amount         string
	TokenID        string // Optional: for ERC-20 tokens like USDC
}

type CircleTransfer struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type CircleTransaction struct {
	ID                 string   `json:"id"`
	Blockchain         string   `json:"blockchain"`
	CreateDate         string   `json:"createDate"`
	State              string   `json:"state"`
	TransactionType    string   `json:"transactionType"`
	UpdateDate         string   `json:"updateDate"`
	Amounts            []string `json:"amounts"`
	TxHash             string   `json:"txHash,omitempty"`
	DestinationAddress string   `json:"destinationAddress,omitempty"`
	SourceAddress      string   `json:"sourceAddress,omitempty"`
	WalletID           string   `json:"walletId"`
	TokenID            string   `json:"tokenId,omitempty"`
	CustodyType        string   `json:"custodyType"`
}

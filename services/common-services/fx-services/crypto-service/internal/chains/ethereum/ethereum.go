// internal/chains/ethereum/ethereum.go
package ethereum

import (
	"context"
	"crypto-service/internal/chains/ethereum/circle"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

type EthereumChain struct {
	client       *ethclient.Client
	circleClient *circle.Client
	logger       *zap.Logger
	config       *Config
}

type Config struct {
	RPCURL        string
	ChainID       *big.Int
	GasLimitETH   uint64
	GasLimitERC20 uint64
	MaxGasPrice   *big.Int
	Confirmations int
	// Circle config
	CircleEnabled bool
}

func NewEthereumChain(rpcURL string, circleAPIKey string, circleEnv string, logger *zap.Logger) (*EthereumChain, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum: %w", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	config := &Config{
		RPCURL:        rpcURL,
		ChainID:       chainID,
		GasLimitETH:   21000,
		GasLimitERC20: 65000,
		MaxGasPrice:   big.NewInt(100e9),
		Confirmations: 12,
		CircleEnabled: false,
	}

	chain := &EthereumChain{
		client: client,
		logger: logger,
		config: config,
	}

	// Initialize Circle if API key provided
	if circleAPIKey != "" {
		logger.Info("Initializing Circle for USDC operations",
			zap.String("environment", circleEnv))

		circleClient, err := circle.NewClient(circleAPIKey, circleEnv, logger)
		if err != nil {
			logger.Warn("Failed to initialize Circle, will use ERC-20 fallback", zap.Error(err))
		} else {
			chain.circleClient = circleClient
			config.CircleEnabled = true
			logger.Info("Circle initialized successfully for USDC")
		}
	} else {
		logger.Info("Circle not configured, using ERC-20 for USDC")
	}

	logger.Info("Ethereum chain initialized",
		zap.String("rpc", rpcURL),
		zap.String("chain_id", chainID.String()),
		zap.Bool("circle_enabled", config.CircleEnabled))

	return chain, nil
}

func (c *EthereumChain) Name() string {
	return "ETHEREUM"
}

func (c *EthereumChain) Symbol() string {
	return "ETH"
}

//  GenerateWallet - Circle-aware if enabled
func (c *EthereumChain) GenerateWallet(ctx context.Context) (*domain.Wallet, error) {
	// Check wallet type from context
	walletType, ok := ctx.Value(domain.WalletTypeKey).(string)
	if !ok {
		walletType = domain.WalletTypeStandard // Default to standard
	}

	c.logger.Info("Generating wallet",
		zap.String("type", walletType),
		zap.Bool("circle_enabled", c.config.CircleEnabled))

	switch walletType {
	case domain.WalletTypeCircle:
		// Generate Circle wallet if enabled
		if !c.config.CircleEnabled {
			c.logger.Warn("Circle wallet requested but Circle not enabled, generating standard wallet")
			return generateEthereumWallet()
		}

		// Get user ID from context
		userID, ok := ctx.Value(domain.UserIDKey).(string)
		if !ok || userID == "" {
			return nil, fmt.Errorf("user_id required for Circle wallet generation")
		}

		return c.CreateCircleWallet(ctx, userID)

	case domain.WalletTypeStandard:
		fallthrough
	default:
		// Generate standard Ethereum wallet
		return generateEthereumWallet()
	}
}

func (c *EthereumChain) ImportWallet(ctx context.Context, privateKey string) (*domain.Wallet, error) {
	return importEthereumWallet(privateKey)
}

func (c *EthereumChain) ValidateAddress(address string) error {
	if !common.IsHexAddress(address) {
		return fmt.Errorf("invalid Ethereum address format")
	}

	addr := common.HexToAddress(address)
	if addr.Hex() != address && !strings.EqualFold(addr.Hex(), address) {
		return fmt.Errorf("invalid address checksum")
	}

	return nil
}

func (c *EthereumChain) GetBalance(ctx context.Context, address string, asset *domain.Asset) (*domain.Balance, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is required")
	}

	// Native ETH balance
	if asset.Type == domain.AssetTypeNative {
		addr := common.HexToAddress(address)
		balance, err := c.client.BalanceAt(ctx, addr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get ETH balance: %w", err)
		}

		return &domain.Balance{
			Address:  address,
			Asset:    asset,
			Amount:   balance,
			Decimals: 18,
		}, nil
	}

	// USDC via Circle (if enabled)
	if asset.Type == domain.AssetTypeToken && asset.Symbol == "USDC" && c.config.CircleEnabled {
		return c.getCircleUSDCBalance(ctx, address, asset)
	}

	// Other ERC-20 tokens (or USDC fallback)
	if asset.Type == domain.AssetTypeToken {
		balance, err := c.getERC20Balance(ctx, address, asset)
		if err != nil {
			return nil, err
		}

		return &domain.Balance{
			Address:  address,
			Asset:    asset,
			Amount:   balance,
			Decimals: asset.Decimals,
		}, nil
	}

	return nil, fmt.Errorf("unsupported asset type: %s", asset.Type)
}

func (c *EthereumChain) EstimateFee(ctx context.Context, req *domain.TransactionRequest) (*domain.Fee, error) {
	if req.Asset == nil {
		return nil, fmt.Errorf("asset is required")
	}

	// USDC via Circle - no separate fee
	if req.Asset.Symbol == "USDC" && c.config.CircleEnabled {
		return &domain.Fee{
			Amount:   big.NewInt(0),
			Currency: "USDC",
		}, nil
	}

	// ETH or ERC-20 - calculate gas fee
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	if gasPrice.Cmp(c.config.MaxGasPrice) > 0 {
		gasPrice = c.config.MaxGasPrice
	}

	var gasLimit uint64
	if req.Asset.Type == domain.AssetTypeNative {
		gasLimit = c.config.GasLimitETH
	} else {
		gasLimit = c.config.GasLimitERC20
	}

	fee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	gasLimitInt := int64(gasLimit)

	return &domain.Fee{
		Amount:   fee,
		Currency: "ETH",
		GasLimit: &gasLimitInt,
		GasPrice: gasPrice,
	}, nil
}

func (c *EthereumChain) Send(ctx context.Context, req *domain.TransactionRequest) (*domain.TransactionResult, error) {
	if req.Asset == nil {
		return nil, fmt.Errorf("asset is required")
	}

	c.logger.Info("Sending Ethereum transaction",
		zap.String("from", req.From),
		zap.String("to", req.To),
		zap.String("asset", req.Asset.Symbol),
		zap.String("amount", req.Amount.String()))

	// Native ETH transfer
	if req.Asset.Type == domain.AssetTypeNative {
		return c.sendETH(ctx, req)
	}

	// USDC via Circle (if enabled)
	if req.Asset.Symbol == "USDC" && c.config.CircleEnabled {
		return c.sendCircleUSDC(ctx, req)
	}

	// Other ERC-20 tokens (or USDC fallback)
	if req.Asset.Type == domain.AssetTypeToken {
		return c.sendERC20(ctx, req)
	}

	return nil, fmt.Errorf("unsupported asset type: %s", req.Asset.Type)
}

func (c *EthereumChain) GetTransaction(ctx context.Context, txHash string) (*domain.Transaction, error) {
	hash := common.HexToHash(txHash)

	tx, isPending, err := c.client.TransactionByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	transaction := &domain.Transaction{
		Hash:   txHash,
		Chain:  c.Name(),
		Amount: tx.Value(),
		Fee:    new(big.Int).Mul(tx.GasPrice(), big.NewInt(int64(tx.Gas()))),
	}

	if tx.To() != nil {
		transaction.To = tx.To().Hex()
	}

	signer := types.LatestSignerForChainID(c.config.ChainID)
	sender, err := types.Sender(signer, tx)
	if err == nil {
		transaction.From = sender.Hex()
	}

	if isPending {
		transaction.Status = domain.TxStatusPending
		transaction.Confirmations = 0
		return transaction, nil
	}

	receipt, err := c.client.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %w", err)
	}

	blockNum := receipt.BlockNumber.Int64()
	transaction.BlockNumber = &blockNum

	currentBlock, err := c.client.BlockNumber(ctx)
	if err == nil {
		transaction.Confirmations = int(currentBlock - receipt.BlockNumber.Uint64())
	}

	if receipt.Status == types.ReceiptStatusSuccessful {
		if transaction.Confirmations >= c.config.Confirmations {
			transaction.Status = domain.TxStatusConfirmed
		} else {
			transaction.Status = domain.TxStatusPending
		}
	} else {
		transaction.Status = domain.TxStatusFailed
	}

	if block, err := c.client.BlockByNumber(ctx, receipt.BlockNumber); err == nil {
		transaction.Timestamp = time.Unix(int64(block.Time()), 0)
	}

	transaction.Fee = new(big.Int).Mul(tx.GasPrice(), big.NewInt(int64(receipt.GasUsed)))

	return transaction, nil
}
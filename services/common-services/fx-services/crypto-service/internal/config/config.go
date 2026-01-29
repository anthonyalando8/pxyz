// config/config.go
package config

import (
	"os"
	"strconv"

	"go.uber.org/zap"
)

type Config struct {
	Security SecurityConfig
	Tron     TronConfig
	Bitcoin  BitcoinConfig
	Ethereum EthereumConfig
}

type SecurityConfig struct {
	MasterKey     string
	VaultProvider string // "env", "file", "hashicorp"
	VaultAddress  string
	VaultToken    string
	FileVaultDir  string
	FileVaultKey  string
}

type TronConfig struct {
	APIKey  string
	Network string
	HTTPUrl string
	GRPCUrl string
}

type BitcoinConfig struct {
	RPCURL  string
	APIKey  string
	Network string // mainnet, testnet, regtest
}

type EthereumConfig struct {
	Enabled     bool
	RPCURL      string
	Network     string // mainnet, goerli, sepolia
	ChainID     int64
	USDCAddress string
	MaxGasPrice int64 // in Gwei
}

func Load(logger *zap.Logger) (*Config, error) {
	// ============================================================================
	// TRON Configuration
	// ============================================================================
	tronNetwork := getEnv("TRON_NETWORK", "shasta")
	
	var tronHTTPUrl, tronGRPCUrl string
	switch tronNetwork {
	case "mainnet":
		tronHTTPUrl = "https://api.trongrid.io"
		tronGRPCUrl = "grpc.trongrid.io:50051"
	case "shasta":
		tronHTTPUrl = "https://api.shasta.trongrid.io"
		tronGRPCUrl = "grpc.shasta.trongrid.io:50051"
	case "nile":
		tronHTTPUrl = "https://api.nile.trongrid.io"
		tronGRPCUrl = "grpc.nile.trongrid.io:50051"
	}

	// ============================================================================
	// Bitcoin Configuration
	// ============================================================================
	btcNetwork := getEnv("BTC_NETWORK", "testnet")
	btcRPCURL := getEnv("BTC_RPC_URL", "")
	
	// Use default public endpoints if not specified
	if btcRPCURL == "" {
		switch btcNetwork {
		case "mainnet":
			btcRPCURL = "https://blockstream.info/api"
		case "testnet":
			btcRPCURL = "https://blockstream.info/testnet/api"
		}
	}

	// ============================================================================
	// Ethereum Configuration
	// ============================================================================
	ethEnabled := getEnvAsBool("ETHEREUM_ENABLED", true)
	ethNetwork := getEnv("ETHEREUM_NETWORK", "sepolia")
	ethRPCURL := getEnv("ETHEREUM_RPC_URL", "")
	
	// Default RPC URLs based on network
	if ethRPCURL == "" {
		switch ethNetwork {
		case "mainnet":
			ethRPCURL = "https://eth-mainnet.g.alchemy.com/v2/Qpdkf6vx2xPgSxuMpWtjA" // Use your own API key
		case "goerli":
			ethRPCURL = "https://eth-goerli.g.alchemy.com/v2/Qpdkf6vx2xPgSxuMpWtjA"
		case "sepolia":
			ethRPCURL = "https://eth-sepolia.g.alchemy.com/v2/Qpdkf6vx2xPgSxuMpWtjA"
		default:
			ethRPCURL = "https://eth-sepolia.g.alchemy.com/v2/Qpdkf6vx2xPgSxuMpWtjA"
		}
	}

	// Chain ID based on network
	var ethChainID int64
	switch ethNetwork {
	case "mainnet":
		ethChainID = 1
	case "goerli":
		ethChainID = 5
	case "sepolia":
		ethChainID = 11155111
	default:
		ethChainID = getEnvAsInt64("ETHEREUM_CHAIN_ID", 11155111) // Default to Sepolia
	}

	// USDC contract address based on network
	ethUSDCAddress := getEnv("ETHEREUM_USDC_ADDRESS", "")
	if ethUSDCAddress == "" {
		switch ethNetwork {
		case "mainnet":
			ethUSDCAddress = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48" // Mainnet USDC
		case "goerli":
			ethUSDCAddress = "0x07865c6E87B9F70255377e024ace6630C1Eaa37F" // Goerli USDC
		case "sepolia":
			ethUSDCAddress = "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238" // Sepolia USDC
		default:
			ethUSDCAddress = "0x07865c6E87B9F70255377e024ace6630C1Eaa37F"
		}
	}

	// Max gas price in Gwei (default 100 Gwei)
	ethMaxGasPrice := getEnvAsInt64("ETHEREUM_MAX_GAS_PRICE", 100)

	// ============================================================================
	// Security Configuration
	// ============================================================================
	return &Config{
		Tron: TronConfig{
			APIKey:  getEnv("TRON_API_KEY", ""),
			Network: tronNetwork,
			HTTPUrl: tronHTTPUrl,
			GRPCUrl: tronGRPCUrl,
		},
		Bitcoin: BitcoinConfig{
			RPCURL:  btcRPCURL,
			APIKey:  getEnv("BTC_API_KEY", ""),
			Network: btcNetwork,
		},
		Ethereum: EthereumConfig{
			Enabled:     ethEnabled,
			RPCURL:      ethRPCURL,
			Network:     ethNetwork,
			ChainID:     ethChainID,
			USDCAddress: ethUSDCAddress,
			MaxGasPrice: ethMaxGasPrice,
		},
		Security: SecurityConfig{
			MasterKey:     os.Getenv("CRYPTO_MASTER_KEY"),
			VaultProvider: getEnv("VAULT_PROVIDER", "env"),
			VaultAddress:  os.Getenv("VAULT_ADDRESS"),
			VaultToken:    os.Getenv("VAULT_TOKEN"),
			FileVaultDir:  getEnv("FILE_VAULT_DIR", "./vault"),
			FileVaultKey:  os.Getenv("FILE_VAULT_KEY"),
		},
	}, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
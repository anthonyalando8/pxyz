package config

import (
	"os"

	"go.uber.org/zap"
)

// config/config.go
type Config struct {
	// ... other config
	// Server     ServerConfig
	// Database   DatabaseConfig
	Security   SecurityConfig
	// Blockchain BlockchainConfig

	Tron TronConfig
	Bitcoin    BitcoinConfig
}

type SecurityConfig struct {
	MasterKey       string
	VaultProvider   string // "env", "file", "hashicorp"
	VaultAddress    string
	VaultToken      string
	FileVaultDir    string
	FileVaultKey    string
}

type TronConfig struct {
	APIKey    string
	Network   string
	HTTPUrl   string
	GRPCUrl   string
}

type BitcoinConfig struct {
	RPCURL  string
	APIKey  string
	Network string // mainnet, testnet, regtest
}

func Load(logger *zap.Logger) (*Config, error) {
	network := getEnv("TRON_NETWORK", "shasta")
	
	var httpUrl, grpcUrl string
	switch network {
	case "mainnet":
		httpUrl = "https://api.trongrid.io"
		grpcUrl = "grpc.trongrid.io:50051"
	case "shasta":
		httpUrl = "https://api.shasta.trongrid.io"
		grpcUrl = "grpc.shasta.trongrid.io:50051"
	case "nile":
		httpUrl = "https://api.nile.trongrid.io"
		grpcUrl = "grpc.nile.trongrid.io:50051"
	}

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

	return &Config{
		Tron: TronConfig{
			APIKey:  getEnv("TRON_API_KEY", ""),
			Network: network,
			HTTPUrl: httpUrl,
			GRPCUrl: grpcUrl,
		},
		Bitcoin:  BitcoinConfig{
			RPCURL:  btcRPCURL,
			APIKey:  getEnv("BTC_API_KEY", ""),
			Network: btcNetwork,
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
func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
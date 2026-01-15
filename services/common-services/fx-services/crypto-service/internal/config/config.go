package config

import (
	"os"

	"go.uber.org/zap"
)

// config/config.go
type Config struct {
	// ... other config

	Tron TronConfig
}

type TronConfig struct {
	APIKey    string
	Network   string
	HTTPUrl   string
	GRPCUrl   string
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

	return &Config{
		Tron: TronConfig{
			APIKey:  getEnv("TRON_API_KEY", ""),
			Network: network,
			HTTPUrl: httpUrl,
			GRPCUrl: grpcUrl,
		},
	}, nil
}
func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
// shared/crypto/client.go
package cryptoclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	cryptopb "x/shared/genproto/shared/accounting/cryptopb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CryptoClient struct {
	WalletClient      cryptopb.WalletServiceClient
	TransactionClient cryptopb.TransactionServiceClient
	DepositClient     cryptopb. DepositServiceClient
	//SystemClient      cryptopb.SystemServiceClient
}

// NewCryptoClient connects to the crypto service and returns a ready-to-use client
func NewCryptoClient() *CryptoClient {
	// Default to crypto-service: 8028 if env var not set
	cryptoServiceAddr := getEnv("CRYPTO_SERVICE_ADDR", "crypto-service: 8028")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc. DialContext(ctx, cryptoServiceAddr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // Wait for connection
	)
	if err != nil {
		log. Fatalf("Failed to connect to crypto service: %v", err)
	}

	fmt.Println("✅ Connected to Crypto Service at", cryptoServiceAddr)

	return &CryptoClient{
		WalletClient:      cryptopb.NewWalletServiceClient(conn),
		TransactionClient: cryptopb.NewTransactionServiceClient(conn),
		DepositClient:     cryptopb.NewDepositServiceClient(conn),
		//SystemClient:      cryptopb.NewSystemServiceClient(conn),
	}
}

// NewCryptoClientOrNil returns client or nil if connection fails (non-fatal)
func NewCryptoClientOrNil() *CryptoClient {
	cryptoServiceAddr := getEnv("CRYPTO_SERVICE_ADDR", "crypto-service:8028")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cryptoServiceAddr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Printf("⚠️  Could not connect to crypto service (will skip wallet creation): %v", err)
		return nil
	}

	fmt.Println("✅ Connected to Crypto Service at", cryptoServiceAddr)

	return &CryptoClient{
		WalletClient:      cryptopb. NewWalletServiceClient(conn),
		TransactionClient: cryptopb.NewTransactionServiceClient(conn),
		DepositClient:     cryptopb.NewDepositServiceClient(conn),
		//SystemClient:      cryptopb.NewSystemServiceClient(conn),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
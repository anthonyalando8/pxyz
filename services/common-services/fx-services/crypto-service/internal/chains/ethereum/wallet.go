// internal/chains/ethereum/wallet.go
package ethereum

import (
	"crypto-service/internal/domain"
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// generateEthereumWallet generates a new Ethereum wallet
func generateEthereumWallet() (*domain.Wallet, error) {
	// Generate private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Get private key bytes
	privateKeyBytes := crypto.FromECDSA(privateKey)
	privateKeyHex := hexutil.Encode(privateKeyBytes)[2:] // Remove 0x prefix

	// Get public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast public key")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	publicKeyHex := hexutil.Encode(publicKeyBytes)[2:] // Remove 0x prefix

	// Get address
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	return &domain.Wallet{
		Address:    address,
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Chain:      "ETHEREUM",
		CreatedAt:  time.Now(),
	}, nil
}

// importEthereumWallet imports wallet from private key
func importEthereumWallet(privateKeyHex string) (*domain.Wallet, error) {
	// Remove 0x prefix if present
	if len(privateKeyHex) > 2 && privateKeyHex[:2] == "0x" {
		privateKeyHex = privateKeyHex[2:]
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// Get public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast public key")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	publicKeyHex := hexutil.Encode(publicKeyBytes)[2:]

	// Get address
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	return &domain.Wallet{
		Address:    address,
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Chain:      "ETHEREUM",
		CreatedAt:  time.Now(),
	}, nil
}
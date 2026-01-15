// internal/chains/tron/wallet.go
package tron

import (
	"crypto-service/internal/domain"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"time"
	
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

func generateTronWallet() (*domain.Wallet, error) {
	// Generate private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt. Errorf("failed to generate private key: %w", err)
	}
	
	// Get public key
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	
	// Generate TRON address
	addr := address.PubkeyToAddress(*publicKey)
	
	// Convert to hex strings
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	publicKeyHex := hex.EncodeToString(crypto.FromECDSAPub(publicKey))
	
	return &domain.Wallet{
		Address:    addr.String(),
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Chain:      "TRON",
		CreatedAt:  time.Now(),
	}, nil
}

func importTronWallet(privateKeyHex string) (*domain.Wallet, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	addr := address.PubkeyToAddress(*publicKey)
	publicKeyHex := hex.EncodeToString(crypto.FromECDSAPub(publicKey))
	
	return &domain.Wallet{
		Address:    addr.String(),
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Chain:      "TRON",
		CreatedAt:  time.Now(),
	}, nil
}
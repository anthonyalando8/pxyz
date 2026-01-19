// internal/chains/bitcoin/wallet.go
package bitcoin

import (
	"crypto-service/internal/domain"
	// "crypto/rand"
	// "crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
)

// GenerateBitcoinWallet generates a new Bitcoin wallet
func GenerateBitcoinWallet(network string) (*domain.Wallet, error) {
	// Get network params
	params, err := getNetworkParams(network)
	if err != nil {
		return nil, err
	}

	// Generate private key
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key:  %w", err)
	}

	// Get public key
	publicKey := privateKey.PubKey()

	// Create address (P2PKH - Pay to Public Key Hash)
	pubKeyHash := btcutil.Hash160(publicKey.SerializeCompressed())
	address, err := btcutil.NewAddressPubKeyHash(pubKeyHash, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create address: %w", err)
	}

	// Export private key in WIF format
	wif, err := btcutil.NewWIF(privateKey, params, true) // true = compressed
	if err != nil {
		return nil, fmt. Errorf("failed to create WIF: %w", err)
	}

	return &domain.Wallet{
		Address:    address. EncodeAddress(),
		PrivateKey: wif. String(),
		PublicKey:  hex.EncodeToString(publicKey.SerializeCompressed()),
		Chain:      "BITCOIN",
		CreatedAt:  time.Now(),
	}, nil
}

// ImportBitcoinWallet imports a wallet from WIF private key
func ImportBitcoinWallet(privateKeyWIF, network string) (*domain.Wallet, error) {
	// Get network params
	params, err := getNetworkParams(network)
	if err != nil {
		return nil, err
	}

	// Decode WIF
	wif, err := btcutil.DecodeWIF(privateKeyWIF)
	if err != nil {
		return nil, fmt.Errorf("invalid WIF private key: %w", err)
	}

	// Get public key
	publicKey := wif. PrivKey.PubKey()

	// Create address
	pubKeyHash := btcutil.Hash160(publicKey.SerializeCompressed())
	address, err := btcutil.NewAddressPubKeyHash(pubKeyHash, params)
	if err != nil {
		return nil, fmt. Errorf("failed to create address: %w", err)
	}

	return &domain.Wallet{
		Address:    address.EncodeAddress(),
		PrivateKey: privateKeyWIF,
		PublicKey:  hex.EncodeToString(publicKey.SerializeCompressed()),
		Chain:      "BITCOIN",
		CreatedAt:  time.Now(),
	}, nil
}

// getNetworkParams returns chaincfg params for network
func getNetworkParams(network string) (*chaincfg.Params, error) {
	switch network {
	case "mainnet":
		return &chaincfg.MainNetParams, nil
	case "testnet":
		return &chaincfg.TestNet3Params, nil
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	default: 
		return nil, fmt.Errorf("unsupported network: %s", network)
	}
}

// ValidateBitcoinAddress validates a Bitcoin address
func ValidateBitcoinAddress(address, network string) error {
	params, err := getNetworkParams(network)
	if err != nil {
		return err
	}

	_, err = btcutil.DecodeAddress(address, params)
	if err != nil {
		return fmt.Errorf("invalid Bitcoin address: %w", err)
	}

	return nil
}
// internal/chains/ethereum/signer.go
package ethereum

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	//"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// signTransaction signs an Ethereum transaction
func signTransaction(tx *types.Transaction, privateKeyHex string, chainID *big.Int) (*types.Transaction, error) {
	// Remove 0x prefix if present
	if len(privateKeyHex) > 2 && privateKeyHex[:2] == "0x" {
		privateKeyHex = privateKeyHex[2:]
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// Sign transaction
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return signedTx, nil
}

// recoverSigner recovers the sender address from a signed transaction
func recoverSigner(tx *types.Transaction, chainID *big.Int) (string, error) {
	signer := types.NewEIP155Signer(chainID)
	
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return "", fmt.Errorf("failed to recover sender: %w", err)
	}

	return sender.Hex(), nil
}

// privateKeyToAddress converts private key to address
func privateKeyToAddress(privateKeyHex string) (string, error) {
	// Remove 0x prefix if present
	if len(privateKeyHex) > 2 && privateKeyHex[:2] == "0x" {
		privateKeyHex = privateKeyHex[2:]
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("failed to cast public key")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	return address, nil
}
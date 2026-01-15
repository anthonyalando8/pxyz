// internal/chains/tron/signer.go
package tron

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
)

// signTransaction signs a TRON transaction
func (t *TronChain) signTransaction(tx *core.Transaction, privateKeyHex string) (*core.Transaction, error) {
	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// Get raw data hash
	rawData, err := proto.Marshal(tx.RawData)
	if err != nil {
		return nil, fmt. Errorf("failed to marshal raw data: %w", err)
	}

	// Hash the raw data
	hash := sha256.Sum256(rawData)

	// Sign
	signature, err := crypto.Sign(hash[: ], privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign:  %w", err)
	}

	// Set signature
	tx.Signature = [][]byte{signature}

	return tx, nil
}

// getTxHash calculates transaction hash from signed transaction
func getTxHash(tx *core.Transaction) (string, error) {
	// Serialize raw data
	rawData, err := proto.Marshal(tx.RawData)
	if err != nil {
		return "", fmt. Errorf("failed to marshal raw data: %w", err)
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(rawData)
	
	// Return hex encoded hash
	return hex.EncodeToString(hash[:]), nil
}

// getTxID is an alias for getTxHash
func getTxID(tx *core.Transaction) (string, error) {
	return getTxHash(tx)
}

// recoverAddressFromSignature recovers address from signature (for verification)
func recoverAddressFromSignature(tx *core.Transaction) (string, error) {
	if len(tx.Signature) == 0 {
		return "", fmt.Errorf("no signature found")
	}

	rawData, err := proto.Marshal(tx.RawData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal raw data: %w", err)
	}

	hash := sha256.Sum256(rawData)
	
	pubKey, err := crypto.SigToPub(hash[:], tx. Signature[0])
	if err != nil {
		return "", fmt. Errorf("failed to recover public key: %w", err)
	}

	addr := address.PubkeyToAddress(*pubKey)
	return addr.String(), nil
}

// VerifySignature verifies transaction signature matches from address
func VerifySignature(tx *core.Transaction, expectedAddress string) (bool, error) {
	recoveredAddr, err := recoverAddressFromSignature(tx)
	if err != nil {
		return false, err
	}

	return recoveredAddr == expectedAddress, nil
}
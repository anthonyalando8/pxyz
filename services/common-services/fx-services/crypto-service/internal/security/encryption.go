// internal/security/encryption.go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encryption handles encryption and decryption of sensitive data
type Encryption struct {
	masterKey []byte
	version   string
}

// NewEncryption creates a new encryption instance
// masterKey should be 32 bytes for AES-256
func NewEncryption(masterKey string) (*Encryption, error) {
	// Decode base64 key or use raw key
	keyBytes := []byte(masterKey)

	// If key is base64 encoded, decode it
	if decoded, err := base64.StdEncoding.DecodeString(masterKey); err == nil {
		keyBytes = decoded
	}

	// Validate key length (must be 16, 24, or 32 bytes for AES)
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid master key length: must be 32 bytes for AES-256, got %d", len(keyBytes))
	}

	return &Encryption{
		masterKey: keyBytes,
		version:   "v1",
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64 encoded ciphertext with nonce prepended
func (e *Encryption) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", fmt.Errorf("plaintext cannot be empty")
	}

	// Create AES cipher block
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher:  %w", err)
	}

	// Use GCM mode (Galois/Counter Mode) for authenticated encryption
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt plaintext
	// Nonce is prepended to ciphertext for decryption
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 for storage
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return encoded, nil
}

// Decrypt decrypts base64 encoded ciphertext using AES-256-GCM
func (e *Encryption) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", fmt.Errorf("ciphertext cannot be empty")
	}

	// Decode from base64
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher block
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Use GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Validate ciphertext length
	nonceSize := gcm.NonceSize()
	if len(decoded) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertextBytes := decoded[:nonceSize], decoded[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// EncryptBytes encrypts byte data
func (e *Encryption) EncryptBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data cannot be empty")
	}

	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

// DecryptBytes decrypts byte data
func (e *Encryption) DecryptBytes(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}

	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// GetVersion returns the encryption version
func (e *Encryption) GetVersion() string {
	return e.version
}

// RotateKey creates a new encryption instance with a new key
// Used for key rotation
func (e *Encryption) RotateKey(newMasterKey string) (*Encryption, error) {
	return NewEncryption(newMasterKey)
}

// ReEncrypt re-encrypts data with a new encryption instance
// Used during key rotation
func (e *Encryption) ReEncrypt(ciphertext string, newEncryption *Encryption) (string, error) {
	// Decrypt with old key
	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt with old key: %w", err)
	}

	// Encrypt with new key
	newCiphertext, err := newEncryption.Encrypt(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt with new key: %w", err)
	}

	return newCiphertext, nil
}

// GenerateMasterKey generates a random 32-byte master key for AES-256
func GenerateMasterKey() (string, error) {
	key := make([]byte, 32) // 256 bits
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}

	// Encode to base64 for easy storage in config/env
	return base64.StdEncoding.EncodeToString(key), nil
}

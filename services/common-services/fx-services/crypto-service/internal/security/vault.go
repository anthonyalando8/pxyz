// internal/security/vault.go
package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// VaultProvider defines interface for secret storage backends
type VaultProvider interface {
	GetSecret(ctx context.Context, path string) (string, error)
	SetSecret(ctx context.Context, path, value string) error
	DeleteSecret(ctx context.Context, path string) error
	ListSecrets(ctx context.Context, path string) ([]string, error)
}

// Vault manages encryption keys and secrets
type Vault struct {
	provider    VaultProvider
	cache       map[string]*cachedSecret
	cacheMutex  sync.RWMutex
	cacheTTL    time.Duration
	logger      *zap.Logger
}

type cachedSecret struct {
	value     string
	expiresAt time.Time
}

// NewVault creates a new vault instance
func NewVault(provider VaultProvider, logger *zap.Logger) *Vault {
	return &Vault{
		provider:   provider,
		cache:      make(map[string]*cachedSecret),
		cacheTTL:   5 * time.Minute, // Cache secrets for 5 minutes
		logger:     logger,
	}
}

// GetMasterKey retrieves the master encryption key
func (v *Vault) GetMasterKey(ctx context.Context) (string, error) {
	return v.GetSecret(ctx, "crypto/master-key")
}

// GetSecret retrieves a secret with caching
func (v *Vault) GetSecret(ctx context.Context, path string) (string, error) {
	// Check cache first
	v.cacheMutex.RLock()
	if cached, ok := v.cache[path]; ok {
		if time.Now().Before(cached.expiresAt) {
			v.cacheMutex.RUnlock()
			v.logger.Debug("Secret retrieved from cache", zap.String("path", path))
			return cached.value, nil
		}
	}
	v.cacheMutex.RUnlock()
	
	// Fetch from provider
	v.logger.Debug("Fetching secret from provider", zap.String("path", path))
	secret, err := v.provider.GetSecret(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to get secret from vault: %w", err)
	}
	
	// Cache the secret
	v.cacheMutex.Lock()
	v.cache[path] = &cachedSecret{
		value:     secret,
		expiresAt: time.Now().Add(v.cacheTTL),
	}
	v.cacheMutex. Unlock()
	
	return secret, nil
}

// SetSecret stores a secret
func (v *Vault) SetSecret(ctx context.Context, path, value string) error {
	if err := v.provider.SetSecret(ctx, path, value); err != nil {
		return fmt. Errorf("failed to set secret in vault: %w", err)
	}
	
	// Invalidate cache
	v.cacheMutex.Lock()
	delete(v.cache, path)
	v.cacheMutex.Unlock()
	
	v.logger.Info("Secret updated in vault", zap.String("path", path))
	
	return nil
}

// DeleteSecret removes a secret
func (v *Vault) DeleteSecret(ctx context. Context, path string) error {
	if err := v.provider. DeleteSecret(ctx, path); err != nil {
		return fmt. Errorf("failed to delete secret:  %w", err)
	}
	
	// Invalidate cache
	v.cacheMutex.Lock()
	delete(v.cache, path)
	v.cacheMutex. Unlock()
	
	v.logger.Info("Secret deleted from vault", zap.String("path", path))
	
	return nil
}

// ClearCache clears the secret cache
func (v *Vault) ClearCache() {
	v.cacheMutex.Lock()
	v.cache = make(map[string]*cachedSecret)
	v.cacheMutex.Unlock()
	
	v.logger.Info("Vault cache cleared")
}

// ============================================================================
// VAULT PROVIDERS
// ============================================================================

// EnvVaultProvider stores secrets in environment variables (for development)
type EnvVaultProvider struct{}

func NewEnvVaultProvider() *EnvVaultProvider {
	return &EnvVaultProvider{}
}

func (p *EnvVaultProvider) GetSecret(ctx context.Context, path string) (string, error) {
	// Convert path to env var name
	// e.g., "crypto/master-key" -> "CRYPTO_MASTER_KEY"
	envKey := pathToEnvKey(path)
	
	value := os.Getenv(envKey)
	if value == "" {
		return "", fmt. Errorf("secret not found: %s (env:  %s)", path, envKey)
	}
	
	return value, nil
}

func (p *EnvVaultProvider) SetSecret(ctx context.Context, path, value string) error {
	envKey := pathToEnvKey(path)
	return os.Setenv(envKey, value)
}

func (p *EnvVaultProvider) DeleteSecret(ctx context.Context, path string) error {
	envKey := pathToEnvKey(path)
	return os.Unsetenv(envKey)
}

func (p *EnvVaultProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
	return nil, fmt.Errorf("ListSecrets not supported for EnvVaultProvider")
}

// ============================================================================
// FILE-BASED VAULT PROVIDER (for testing/development)
// ============================================================================

// FileVaultProvider stores secrets in encrypted files
type FileVaultProvider struct {
	baseDir    string
	encryption *Encryption
	mutex      sync.RWMutex
}

func NewFileVaultProvider(baseDir, encryptionKey string) (*FileVaultProvider, error) {
	encryption, err := NewEncryption(encryptionKey)
	if err != nil {
		return nil, err
	}
	
	// Create base directory if not exists
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create vault directory: %w", err)
	}
	
	return &FileVaultProvider{
		baseDir:    baseDir,
		encryption: encryption,
	}, nil
}

func (p *FileVaultProvider) GetSecret(ctx context.Context, path string) (string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	filePath := fmt.Sprintf("%s/%s. enc", p.baseDir, path)
	
	// Read encrypted file
	ciphertext, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt. Errorf("secret not found:  %s", path)
		}
		return "", fmt.Errorf("failed to read secret:  %w", err)
	}
	
	// Decrypt
	plaintext, err := p. encryption.DecryptBytes(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}
	
	return string(plaintext), nil
}

func (p *FileVaultProvider) SetSecret(ctx context.Context, path, value string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Encrypt
	ciphertext, err := p.encryption.EncryptBytes([]byte(value))
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}
	
	filePath := fmt.Sprintf("%s/%s.enc", p.baseDir, path)
	
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Write encrypted file
	if err := os. WriteFile(filePath, ciphertext, 0600); err != nil {
		return fmt. Errorf("failed to write secret:  %w", err)
	}
	
	return nil
}

func (p *FileVaultProvider) DeleteSecret(ctx context. Context, path string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	filePath := fmt.Sprintf("%s/%s.enc", p.baseDir, path)
	
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("secret not found: %s", path)
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	
	return nil
}

func (p *FileVaultProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
	// Implementation for listing secrets in directory
	return nil, fmt.Errorf("not implemented")
}